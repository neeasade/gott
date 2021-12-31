package main

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/imdario/mergo"
	"github.com/pelletier/go-toml/v2"
)

// turn a path into a flat string like a.0.foo-bar
// nb: you can't define methods on interface{} (which makes sense)
func toString(path []interface{}) string {
	result := ""
	for _, v := range path {
		switch v := v.(type) {
		case string:
			result = result + v + "."
		case int:
			result = result + fmt.Sprintf("%d", v) + "."
		}
	}

	if result != "" {
		// remove trailing .
		result = result[0:len(result)-1]
	}
	return result
}

// "path" is a slice of strings+ints leading to a node in config
// todo: see about using type alises to define these methods on strings/interface{}?
func toPath(s string) []interface{} {
	path := []interface{}{}
	for _, v := range strings.Split(s, ".") {
		// todo: this should account for ints
		i, err := strconv.Atoi(v)
		if err == nil {
			path = append(path, i)
		} else {
			path = append(path, v)
		}
	}
	return path
}

type Node struct {
	// a string, int, date (ie a non-map, non-array)
	// could be a leaf value, or an identifier
	value    interface{}
	children []Node
}

func (n Node) isLeaf() bool {
	return len(n.children) == 0
}

// func (n Node) toMap() (map[string]interface{}, error) {
// 	toReturn := map[string]interface{}{}

// 	if n.isLeaf() {
// 		return nil, errors.New("node can't be turned into map (I'm a leaf)")
// 	}

// 	for _, n_ := range n.children {
// 		if n_.isLeaf() {
// 			return nil, errors.New("node can't be turned into map (missing grandchild)")
// 		}

// 		grandchildren := n_.children
// 		key := n_.value.(string)
// 		if len(grandchildren) == 1 {
// 			toReturn[key] = grandchildren[0].value
// 		} else {
// 			v := []interface{}{}
// 			for _, gc := range grandchildren {
// 				if ! gc.isLeaf() {
// 					return nil, errors.New("node can't be turned into map (grand-grandchildren present)")
// 				}
// 				v = append(v, gc.value)
// 			}
// 			toReturn[key] = v
// 		}
// 	}

// 	return toReturn, nil
// }

func (n Node) toArray() ([]interface{}, error) {
	toReturn = []interface{}

	for _, n_ := range n.children {
		switch v := n_.value.(type) {
		case int:
			toReturn = append(toReturn, n_.value.children.To)
		}

		// if n_.value == key {
		// 	return n_.find(path[1:])
		// }
	}

	return Node{}, errors.New("Invalid path")
}

func (n Node) find(path []interface{}) (Node, error) {
	if len(path) == 0 {
		return n, nil
	}

	for _, n_ := range n.children {
		if n_.value == key {
			return n_.find(path[1:])
		}
	}

	return Node{}, errors.New("Invalid path")
}

func (n Node) changeLeaves(path []interface{}, operation func(Node, []interface{}) (interface{}, error)) error {
	if n.isLeaf() {
		newVal, err := operation(n, path)
		if err != nil {
			vlog("transform err at node %s", toString(path))
			panic(err)
		}

		if newVal != n.value {
			n.value = newVal
		}
	}

	for _, n_ := range n.children {
		n_.changeLeaves(append(path, n.value), operation)
	}

	return nil
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

// todo: this could probably return interface, then is_map dies
func (c config) Dig(path []string) (result config, is_map bool, error error) {
	if len(path) == 0 {
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
// todo: this could probably act on interface as well
func (c config) Transform(m map[string]interface{}, path []string, operation func(string, config, []string) (string, error)) {
	for k, v := range m {
		switch v := v.(type) {
		case map[string]interface{}:
			c.Transform(v, append(path, k), operation)
		case string:
			{
				result, err := operation(v, c, append(path, k))
				if err != nil {
					panic(err)
				}
				m[k] = result
			}
		}
	}
}
