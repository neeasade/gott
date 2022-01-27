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

	for _, e := range os.Environ() {
		kv := strings.SplitN(e, "=", 2)
		rootNode.add("env", kv[0], kv[1])
	}

	rootNode.resolveSplices(NodePath{}, rootNode)
	rootNode.walk(NodePath{},
		func(path NodePath) error {
			n, err := rootNode.find(path[1:]...)
			if err != nil {
				// we walked into a child we just removed
				return nil
			}

			nv := n.value

			if n.isLeaf() ||
				fmt.Sprintf("%T", nv) != "string"  ||
				nv.(string) == "" {
				return nil
			}

			if nv.(string)[0:1] == "-" {
				rootNode.remove(path[1:]...)
			}

			return nil
		})

	rootNode.changeLeaves(NodePath{},
		func(path NodePath) (interface{}, error) {
			return qualifyTransform(path, *rootNode)
		})

	vlog(rootNode.view("toml"))
	vlog("------")

	rootNode.changeLeaves(NodePath{},
		func(path NodePath) (interface{}, error) {
			val := path.last()
			if fmt.Sprintf("%T", val) != "string" {
				return val, nil
			}
			return identTransform(val.(string))
		})

	realizeTransform := func(path NodePath) (interface{}, error) {
		nv := path.last()
		if fmt.Sprintf("%T", nv) != "string" {
			return nv, nil
		}
		v := nv.(string)

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
	rootNode.remove("env")

	for _, p := range promotions {
		rootNode.promote(toPath(p))
	}

	if narrow != "" {
		rootNode = rootNode.mustFind(toPath(narrow)...)
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

		r, err := identTransform(string(bytes))
		fmt.Println(rootNode.mustRender(tmpl, r))
	}

	if queryString != "" {
		s := toPath(queryString).ToIndexCall()
		queryString = "{{" + s + "}}"
		fmt.Println(rootNode.mustRender(tmpl, queryString))
	}

	if queryStringPlain != "" {
		fmt.Println(rootNode.render(tmpl, queryStringPlain))
	}
}
