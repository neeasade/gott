package main

import (
	"flag"
	// "fmt"
	"log"
	"os"
	// "strings"
)

var verbose bool = false
var glog *log.Logger = log.New(os.Stderr, "", 0)

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

	flag.Var(&tomlFiles, "t", "Add a toml file to consider")
	flag.Var(&tomlText, "T", "Add raw toml to consider")
	flag.Var(&promotions, "p", "Promote a namespace to the top level")
	flag.Var(&renderTargets, "r", "Render a file")
	flag.StringVar(&action, "o", "", "Output type <shell|toml>")
	flag.BoolVar(&verbose, "v", false, "be verbose")
	flag.StringVar(&queryString, "q", "", "Render a string (implicit surrounding {{}})")
	flag.StringVar(&queryStringPlain, "R", "", "Render a string")
	flag.StringVar(&narrow, "n", "", "Narrow the namespaces to consider")
	flag.Parse()

	// c := parseToml(tomlFiles, tomlText)

	// test := Node{
	// 	"a",
	// 	[
	// 		Node"c"
	// 		]
	// }

	os.Exit(0)
	// tmpl := makeTemplate()
	// c.Transform(c, []string{}, qualifyTransform)

	// // vlog(c.View("toml"))

	// vlog("------")
	// c.Transform(c, []string{}, identTransform)
	// vlog(c.View("toml"))

	// realizeTransform := func(v string, c config, path []string) (string, error) {
	// 	// oof
	// 	for strings.Contains(v, "{{") {
	// 		original := v
	// 		name := strings.Join(path, ".")
	// 		vlog("rendering %s: %s", name, v)
	// 		v = c.Render(tmpl, v)
	// 		if original != v {
	// 			vlog("   result %s: %s", name, v)
	// 		}
	// 	}
	// 	return v, nil
	// }

	// c.Transform(c, []string{}, realizeTransform)

	// for _, p := range promotions {
	// 	c = c.Promote(strings.Split(p, "."))
	// }

	// if narrow != "" {
	// 	d, is_map, err := c.Dig(strings.Split(narrow, "."))
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	if !is_map {
	// 		glog.Fatalf("Narrowed to a non-map value! %s. Use -q instead.", narrow)
	// 	}
	// 	c = d
	// }

	// if action != "" {
	// 	view, err := c.View(action)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	fmt.Print(view)
	// }

	// for _, file := range renderTargets {
	// 	bytes, err := os.ReadFile(file)
	// 	if err != nil {
	// 		glog.Fatalf("render file not found: %s", file)
	// 	}

	// 	r, err := identTransform(string(bytes), c, []string{})
	// 	fmt.Println(c.Render(tmpl, r))
	// }

	// if queryString != "" {
	// 	// text/template doesn't like '-', but you can get around it with the index function
	// 	// {{wow.a-thing.cool}} -> {{index .wow "a-thing" "cool"}}
	// 	parts := strings.Split(queryString, ".")
	// 	if len(parts) > 1 {
	// 		queryString = fmt.Sprintf("{{index .%s \"%s\"}}", parts[0], strings.Join(parts[1:], "\" \""))
	// 		vlog("queryString: %s", queryString)
	// 	} else {
	// 		queryString = "{{." + queryString + "}}"
	// 	}
	// 	fmt.Println(c.Render(tmpl, queryString))
	// }

	// if queryStringPlain != "" {
	// 	fmt.Println(c.Render(tmpl, queryStringPlain))
	// }
}
