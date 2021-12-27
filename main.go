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

func (c config) Promote(path []string) config {
	result := config{}
	mergo.Merge(&result, c)

	zoom, _ := c.Narrow(path)
	if err := mergo.Merge(&c, zoom); err != nil {
		panic(err)
	}
	return c
}

func (c config) pathExists(path []string) bool {
	var dig map[string]interface{} = c
	for _, key := range path {
		if v, ok := dig[key]; ok { // ok will be false because 1 is not in the map.
			nested, is_map := v.(map[string]interface{})
			if is_map {
				dig = nested
			}
		} else {
			return false
		}
	}
	return true
}

func (c config) Narrow(path []string) (config, error) {
	// result := config{}
	// mergo.Merge(&result, c)

	var dig map[string]interface{} = c
	for _, key := range path {
		if v, ok := dig[key]; ok { // ok will be false because 1 is not in the map.
			dig = v.(map[string]interface{})
		} else {
			return nil, errors.New("not reached")
		}
	}
	return dig, nil
}

func makeTemplate() *template.Template {
	funcMap := (sprig.TxtFuncMap())
	funcMap["sh"] = func(command, value string) string {
		cmd := exec.Command("bash", "-c", command)
		stdin, _ := cmd.StdinPipe()

		go func() {
			defer stdin.Close()
			io.WriteString(stdin, value)
		}()

		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatal(command)
			log.Fatal(err)
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

func qualifyConfig(m map[string]interface{}, c config, path []string) map[string]interface{} {
	for k, v := range m {
		switch v := v.(type) {
		case []interface{}:
		// for i, v := range v {
		// 	m[k][i] = qualifyConfig(v, config, path)
		// }
		case map[string]interface{}:
			m[k] = qualifyConfig(v, c, append(path, k))
		case string:
			parent, _ := c.Narrow(path)

			identRe := regexp.MustCompile("({{| )((\\.[a-zA-Z0-9]+)+)")

			matches := identRe.FindAllStringSubmatch(fmt.Sprintf("%v", v), -1)

			if len(matches) > 0 {
				name := strings.Join(append(path, k), ".")
				vlog("qualifying %s: %s", name, v)
			}

			for _, groups := range matches {
				// result[2*n:2*n+1]
				// match := groups[0]
				matchPrefix := groups[1]
				matchPath := strings.Split(groups[2][1:], ".")
				matchKey := matchPath[0]

				vlog("groups: %v", groups)
				vlog("parent: %v", parent)
				vlog("path, key: %v %s", path, k)
				vlog("matchkey: %s", matchKey)
				if matchKey != k {
					if !c.pathExists(matchPath) {
						if _, ok := parent[matchKey]; ok {
							new := "." + strings.Join(append(path, matchKey), ".")
							vlog("qualifying %s to %s", matchKey, new)
							m[k] = strings.ReplaceAll(m[k].(string), matchPrefix+"."+matchKey, matchPrefix+new)
						}
					}
				}
			}
		}
	}
	return m
}

func realizeConfig(m map[string]interface{}, config config, path []string, tmpl *template.Template) map[string]interface{} {
	for k, v := range m {
		switch v := v.(type) {
		// case []interface{}:
		// for i, v := range v {
		// 	// m[k][i] = walk(v, config, path)
		// }
		case map[string]interface{}:
			m[k] = realizeConfig(v, config, append(path, k), tmpl)
		case string:
			// oof
			for strings.Contains(m[k].(string), "{{") {
				v := m[k].(string)

				name := strings.Join(append(path, k), ".")
				vlog("rendering %s: %s", name, v)
				m[k] = config.Render(tmpl, v)
				if m[k] != v {
					vlog("   result %s: %s", name, m[k])
				}
			}
		}
	}
	return m
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

	config := parseToml(tomlFiles, tomlText)
	tmpl := makeTemplate()
	qualifyConfig(config, config, []string{})
	realizeConfig(config, config, []string{}, tmpl)

	for _, p := range promotions {
		config = config.Promote(strings.Split(p, "."))
	}

	if narrow != "" {
		c, err := config.Narrow(strings.Split(narrow, "."))
		if err != nil {
			panic(err)
		}
		config = c
	}

	switch action {
	case "toml":
		b, err := toml.Marshal(config)
		if err != nil {
			panic(err)
		}
		fmt.Print(string(b))
	case "keys":
		for k, _ := range config.Flatten() {
			fmt.Println(k)
		}
	case "shell":
		// todo: this should handle arrays and 1d tables
		for k, v := range config.Flatten() {
			k = strings.ReplaceAll(k, ".", "_")
			// // meh on this replace value
			k = strings.ReplaceAll(k, "-", "_")
			v = strings.ReplaceAll(v, "'", "'\\''")
			fmt.Printf("%s='%s'\n", k, v)
		}
	}

	for _, file := range renderTargets {
		bytes, err := os.ReadFile(file)
		if err != nil {
			glog.Fatalf("render file not found: %s", file)
		}
		fmt.Println(config.Render(tmpl, string(bytes)))
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
		fmt.Println(config.Render(tmpl, queryString))
	}

	if queryStringPlain != "" {
		fmt.Println(config.Render(tmpl, queryStringPlain))
	}
}
