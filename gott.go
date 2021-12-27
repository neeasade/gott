package main

import (
	"bytes"
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
	"github.com/otaviokr/topological-sort/toposort"
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
	if err := mergo.Merge(&c, c.Narrow(path)); err != nil {
		panic(err)
	}
	return c
}

func (c config) Narrow(path []string) config {
	var dig map[string]interface{} = c
	for _, key := range path {
		dig = dig[key].(map[string]interface{})
	}
	return dig
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
	// .Parse(template_text)
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

func (c config) inferRenderOrder() []string {
	depMap := map[string][]string{}

	for k, v := range c {
		depMap[k] = []string{}

		identPattern := "({{| ).([a-zA-Z0-9]+)"
		identRe := regexp.MustCompile(identPattern)

		vlog("%s", fmt.Sprintf("%v", v))
		matches := identRe.FindAllStringSubmatch(fmt.Sprintf("%v", v), -1)
		if matches == nil {
			depMap[k]= []string{}
		}

		for _, groups := range matches {
			dep := groups[2]
			if _, ok := c[dep]; ok {
				if dep == k {
					continue
				}
				add := true
				for _, v := range depMap[k] {
					if v == dep {
						add = false
					}
				}
				if add {
					depMap[k] = append(depMap[k], dep)
				}
			}
		}
	}

	for k, v := range depMap {
		vlog("%s: %s", k, fmt.Sprintf("%v", v))
	}

	// got lazy
	sorted, err := toposort.KahnSort(depMap)
	if err != nil {
		panic(err)
	}

	for left, right := 0, len(sorted)-1; left < right; left, right = left+1, right-1 {
		sorted[left], sorted[right] = sorted[right], sorted[left]
	}

	vlog("%v", sorted)
	return sorted
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
				m[k] = config.Promote(path).Render(tmpl, v)
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

	// reverse tomlFiles
	for left, right := 0, len(tomlFiles)-1; left < right; left, right = left+1, right-1 {
		tomlFiles[left], tomlFiles[right] = tomlFiles[right], tomlFiles[left]
	}

	for _, file := range tomlFiles {
		bytes, err := os.ReadFile(file)
		if err != nil {
			glog.Fatalf("TOML file not found: %s", file)
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

	for _, k := range config.inferRenderOrder() {
		nested, is_map := config[k].(map[string]interface{})
		if is_map {
			realizeConfig(nested, config, []string{k}, tmpl)
		} else {
			// fill me in to render top level render values
		}
	}

	for _, p := range promotions {
		config = config.Promote(strings.Split(p, "."))
	}

	if narrow != "" {
		config = config.Narrow(strings.Split(narrow, "."))
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
		fmt.Println(config.Render(makeTemplate(), string(bytes)))
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
		fmt.Println(config.Render(makeTemplate(), queryString))
	}

	if queryStringPlain != "" {
		fmt.Println(config.Render(makeTemplate(), "queryPlain"))
	}
}
