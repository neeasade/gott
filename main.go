package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
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

	tomlMap := parseToml(tomlFiles, tomlText)
	tmpl := makeTemplate()
	rootNode := NewNode("root")
	mapToNode(rootNode, tomlMap, NodePath{})

	rootNode.changeLeaves(NodePath{},
		func(n *Node, path NodePath) (interface{}, error) {
			return qualifyTransform(n, path, *rootNode)
		})

	vlog(rootNode.view("toml"))

	vlog("------")

	// not doing this for now.
	// rootNode.Transform(c, []string{}, identTransform)
	// vlog(rootNode.View("toml"))

	realizeTransform := func(n *Node, path NodePath) (interface{}, error) {
		if fmt.Sprintf("%T", n.value) != "string" {
			return n.value, nil
		}
		v := n.value.(string)

		// oof
		for strings.Contains(v, "{{") {
			original := v
			vlog("rendering %s: %s", path.ToString(), v)
			v = rootNode.mustRender(tmpl, v)
			if original != v {
				vlog("   result %s: %s", path.ToString(), v)
			}
		}
		return v, nil
	}

	rootNode.changeLeaves(NodePath{}, realizeTransform)

	if len(promotions) > 0 {
		rootNode.promote(toPath(strings.Join(promotions, ".")))
	}

	if narrow != "" {
		rootNode = rootNode.mustFind(toPath(narrow))
	}

	if action != "" {
		view, err := rootNode.view(action)
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

		// r, err := identTransform(string(bytes), c, []string{})
		// fmt.Println(c.Render(tmpl, r))
		fmt.Println(rootNode.render(tmpl, string(bytes)))
	}

	if queryString != "" {
		// text/template doesn't like '-', but you can get around it with the index function
		// {{wow.a-thing.cool}} -> {{index .wow "a-thing" "cool"}}
		s := toPath(queryString).ToIndexCall()
		queryString = "{{" + s + "}}"
		fmt.Println(rootNode.render(tmpl, queryString))
	}

	if queryStringPlain != "" {
		fmt.Println(rootNode.render(tmpl, queryStringPlain))
	}
}
