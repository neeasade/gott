package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/imdario/mergo"
	"github.com/pelletier/go-toml/v2"
)

var verbose bool = false
var glog *log.Logger = log.New(os.Stderr, "", 0)

type config map[string]interface{}

func vlog(format string, args ...interface{}) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func flattenMap(results map[string]string, m map[string]interface{}, namespace string) {
	if namespace != "" {
		namespace = namespace + "."
	}

	for key, value := range m {
		nested, is_map := value.(map[string]interface{})
		_, is_array := value.([]interface{})
		if is_map {
			flattenMap(results, nested, namespace+key)
		} else if is_array {
			// do nothing (string array indexes sounds gross)
			// for index, _ := range arrayVal {
			// 	index_string := fmt.Sprintf("[%i]", index)
			// 	flattenMap(nested, namespace + key + index_string, results)
			// }
		} else {
			results[namespace+key] = fmt.Sprintf("%v", value)
		}
	}
}

func (c config) Flatten() map[string]string {
	results := map[string]string{}
	flattenMap(results, c, "")
	return results
}

func (c config) View(kind string) (string, error) {
	switch kind {
	case "toml":
		b, err := toml.Marshal(c)
		if err != nil {
			panic(err)
		}
		return string(b), nil
	case "keys":
		keys := []string{}
		for k, _ := range c.Flatten() {
			keys = append(keys, k)
		}
		return strings.Join(keys, "\n") + "\n", nil
	case "shell":
		var b bytes.Buffer
		// todo: this should handle arrays and 1d tables
		for k, v := range c.Flatten() {
			k = strings.ReplaceAll(k, ".", "_")
			// todo: meh on this replace value
			k = strings.ReplaceAll(k, "-", "_")
			v = strings.ReplaceAll(v, "'", "'\\''")
			fmt.Fprintf(&b, "%s='%s'\n", k, v)
		}
		return b.String(), nil
	}
	return "", errors.New("invalid view requested")
}

func (c config) Promote(path []string) config {
	result := config{}
	mergo.Merge(&result, c)

	zoom, _, _ := c.Dig(path)
	if err := mergo.Merge(&c, zoom); err != nil {
		panic(err)
	}
	return c
}

func (c config) Dig(path []string) (result config, is_map bool, error error) {
	if (len(path) == 0) {
		return c, true, nil
	}

	if v, ok := c[path[0]]; ok {
		m, is_map := v.(map[string]interface{})
		if is_map {
			if len(path) == 1 {
				return config(m), true, nil
			} else {
				return config(m).Dig(path[1:])
			}
		} else {
			if len(path) == 1 {
				return nil, false, nil
			} else {
				return nil, false, errors.New("not reached")
			}
		}
	}

	return nil, false, errors.New("Bad path")
}

func makeTemplate() *template.Template {
	funcMap := (sprig.TxtFuncMap())
	funcMap["shpipe"] = func(command, value string) string {
		cmd := exec.Command("bash", "-c", command)
		stdin, _ := cmd.StdinPipe()

		go func() {
			defer stdin.Close()
			io.WriteString(stdin, value)
		}()

		out, err := cmd.CombinedOutput()
		if err != nil {
			glog.Fatal(err)
		}

		return string(out)
	}

	funcMap["sh"] = func(command string) string {
		out, err := exec.Command("bash", "-c", command).Output()
		if err != nil {
			glog.Fatal(err)
		}
		return string(out)
	}

	funcMap["eq"] = func(a, b interface{}) bool { return a == b }

	if verbose {
		vlog("loaded funcmap functions are:")
		for k, _ := range funcMap {
			vlog(k)
		}
	}

	// todo: (toInt64 is a "cast" wrapper)
	// funcMap["inc"] = func(i interface{}) int64 { return toInt64(i) + 1 }
	// funcMap["dec"] = func(i interface{}) int64 { return toInt64(i) - 1 }

	return template.New("🌳").Option("missingkey=zero").Funcs(funcMap)
}

func (c config) Render(tmpl *template.Template, template_text string) string {
	t := template.Must(tmpl.Parse(template_text))
	result := new(bytes.Buffer)
	err := t.Execute(result, c)
	if err != nil {
		panic(err)
	}
	return result.String()
}

// transform the string values in the map
func (c config) Transform(m map[string]interface{}, path []string, operation func(string, config, []string) (string, error)) {
	for k, v := range m {
		switch v := v.(type) {
		case map[string]interface{}:
			c.Transform(v, append(path, k), operation)
		case string: {
			result, err := operation(v, c, append(path, k))
			if err != nil {
				panic(err)
			}
			m[k] = result
		}
		}
	}
}

// The go template default selection mechanism kinda sucks
// you can't used dashes or numbers, even if they are keys in a table
// invalid: {{.wow.0}} {{.wow-ok}}
// but we want that (mostly dashes). so we'll take every selection and turn it into an index function call.
// todo: consider slicing syntax here as well
func identTransform(v string, c config, path []string) (string, error) {
	identRe := regexp.MustCompile("({{)[^{}\\.]*((\\.[a-zA-Z0-9-]+)+)[^{}]*(}})")
	// reference
	// match: {{sub .a.some-test 1}}
	// Match groups :
	// 0	-	{{
	// 1	-	.a.some-test
	// 2	-	.some-test
	// 3	-	}}

	// have to be a little delicate -- tracking moving point of the edit as we replace larger
	// strings at indexes
	delta := 0

	matches := identRe.FindAllStringSubmatchIndex(fmt.Sprintf("%v", v), -1)
	// matches and submatches are identified by byte index pairs within the input string:
	// result[2*n:2*n+1] identifies the indexes of the nth submatch. The pair for n==0 identifies the
	// match of the entire expression
	for _, groups := range matches {
		toString := func(n int) string {
			return v[delta+groups[2*n]:delta+groups[2*n+1]]
		}
		fullMatch := toString(0)
		ident := toString(2)
		start := groups[2*2]+delta
		end := groups[2*2+1]+delta
		length := end - start

		vlog("fullmatch: %s", fullMatch)

		// you're always replacing at the ident location, it's just a question of adding the ()
		addingBraces := strings.ReplaceAll(fullMatch, " ", "") != "{{" + ident + "}}"

		parts := strings.Split(ident[1:], ".")
		new := fmt.Sprintf("index . \"%s\"", strings.Join(parts, "\" \""))
		if addingBraces {
			new = fmt.Sprintf("(%s)", new)
		}

		v = v[:start] + new + v[end:]
		delta = delta + len(new) - length
	}

	return v, nil
}

func qualifyTransform(v string, c config, path []string) (string, error) {
	identRe := regexp.MustCompile("({{| )((\\.[a-zA-Z0-9-]+)+)")
	matches := identRe.FindAllStringSubmatchIndex(fmt.Sprintf("%v", v), -1)
	parent, _, _ := c.Dig(path[0:len(path)-1])

	if len(matches) > 0 {
		name := strings.Join(path, ".")
		vlog("\nqualifying .%s: %s", name, v)
	}

	delta := 0
	for _, groups := range matches {
		toString := func(n int) string {
			return v[delta+groups[2*n]:delta+groups[2*n+1]]
		}
		start := groups[2*2]+delta
		end := groups[2*2+1]+delta
		length := end - start

		matchPath := strings.Split(toString(2)[1:], ".")
		matchKey := matchPath[0]

		vlog("parent: %v", parent)
		vlog("looking at: .%s", strings.Join(path, "."))
		vlog("matchKey, matchPath: %s, %v", matchKey, matchPath)

		disqualify := func(cond bool, message string) bool {
			if cond {
				vlog("disqualified: %s", message)
			}
			return cond
		}

		_, in_parent := parent[matchKey]
		_, digIsMap, digErr := c.Dig(matchPath)

		if disqualify(matchKey == path[len(path)-1], "self") ||
			disqualify(!in_parent, "not present in parent") ||
			disqualify(digErr == nil && !digIsMap, "matchPath exists in map, and is a value") {
			continue
		}

		new := "." + strings.Join(append(path[0:len(path)-1], matchKey), ".")
		v = v[:start] + new + v[end:]
		delta = delta + len(new) - length
	}
	return v, nil
}

func parseToml(tomlFiles, tomlText []string) config {
	result := map[string]interface{}{}

	for _, file := range tomlFiles {
		bytes, err := os.ReadFile(file)
		if err != nil {
			glog.Fatalf("err reading TOML file: %s", file)
		}
		// "prepend"
		tomlText = append([]string{string(bytes)}, tomlText...)
	}

	for _, text := range tomlText {
		var parsed map[string]interface{}
		err := toml.Unmarshal([]byte(text), &parsed)
		if err != nil {
			panic(err)
		}
		if err := mergo.Merge(&result, parsed); err != nil {
			panic(err)
		}
	}

	return result
}

// add an array type for flag
type arrayFlag []string

func (i *arrayFlag) String() string {
	return ""
}

func (i *arrayFlag) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	var tomlFiles, promotions, renderTargets, tomlText arrayFlag
	var action, queryString, queryStringPlain, narrow string
	var skipCache bool

	flag.Var(&tomlFiles, "t", "Add a toml file to consider")
	flag.Var(&tomlText, "T", "Add raw toml to consider")
	flag.Var(&promotions, "p", "Promote a namespace to the top level")
	flag.Var(&renderTargets, "r", "Render a file")
	flag.StringVar(&action, "o", "", "Output type <shell|toml>")
	flag.BoolVar(&skipCache, "c", false, "skip caching")
	flag.BoolVar(&verbose, "v", false, "be verbose")
	flag.StringVar(&queryString, "q", "", "Render a string (implicit surrounding {{}})")
	flag.StringVar(&queryStringPlain, "R", "", "Render a string")
	flag.StringVar(&narrow, "n", "", "Narrow the namespaces to consider")
	flag.Parse()

	c := parseToml(tomlFiles, tomlText)
	tmpl := makeTemplate()

	c.Transform(c, []string{}, qualifyTransform)
	vlog(c.View("toml"))

	vlog("------")
	c.Transform(c, []string{}, identTransform)
	vlog(c.View("toml"))

	realizeTransform := func (v string, c config, path []string) (string, error) {
		// oof
		for strings.Contains(v, "{{") {
			original := v
			name := strings.Join(path, ".")
			vlog("rendering %s: %s", name, v)
			v = c.Render(tmpl, v)
			if original != v {
				vlog("   result %s: %s", name, v)
			}
		}
		return v, nil
	}

	c.Transform(c, []string{}, realizeTransform)

	for _, p := range promotions {
		c = c.Promote(strings.Split(p, "."))
	}

	if narrow != "" {
		d, is_map, err := c.Dig(strings.Split(narrow, "."))
		if err != nil {
			panic(err)
		}
		if ! is_map {
			glog.Fatalf("Narrowed to a non-map value! %s. Use -q instead.", narrow)
		}
		c = d
	}

	if action != "" {
		view, err := c.View(action)
		if err != nil {
			panic(err)
		}
		fmt.Print(view)
	}

	for _, file := range renderTargets {
		bytes, err := os.ReadFile(file)
		if err != nil {
			glog.Fatalf("render file not found: %s", file)
		}
		fmt.Println(c.Render(tmpl, string(bytes)))
	}

	if queryString != "" {
		// text/template doesn't like '-', but you can get around it with the index function
		// {{wow.a-thing.cool}} -> {{index .wow "a-thing" "cool"}}
		parts := strings.Split(queryString, ".")
		if len(parts) > 1 {
			queryString = fmt.Sprintf("{{index .%s \"%s\"}}", parts[0], strings.Join(parts[1:], "\" \""))
			vlog("queryString: %s", queryString)
		} else {
			queryString = "{{." + queryString + "}}"
		}
		fmt.Println(c.Render(tmpl, queryString))
	}

	if queryStringPlain != "" {
		fmt.Println(c.Render(tmpl, queryStringPlain))
	}
}
