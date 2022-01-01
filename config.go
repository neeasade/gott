package main

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	// "text/template"

	"github.com/imdario/mergo"
	"github.com/pelletier/go-toml/v2"
)


type NodePath []interface{} // collection of strings and ints leading to a node

type Node struct {
	// value is a string, int, date (ie a non-map, non-array)
	// could be a leaf value, or an identifier (map string key or array index)
	value    interface{}
	children []*Node
}

func (root Node) ToMap() map[string]interface{} {
	result := map[string]interface{}{}

	root.changeLeaves([]interface{}{},
		func(n *Node, path NodePath) (interface{}, error) {
			vlog("map from leaf: %s", path.ToString())
			path = path[1:]

			var base interface{} = map[string]interface{}{}
			var runningVal interface{} = base

			runningNode := root

			for _, p := range path {
				vlog("base: %v", base)

				next, err := runningNode.find(p)
				if err != nil {
					panic(err)
				}

				asMap, isMap := runningVal.(map[string]interface{})
				asArray, isArray := runningVal.([]interface{})

				cString, cStringp := runningNode.value.(string)
				cInt, cIntp  := runningNode.value.(int)

				_, nStringp := next.value.(string)
				_, nIntp  := next.value.(int)

				if next.isLeaf() {
					vlog("next is leaf")
					if isMap && cStringp {
						vlog("is leaf map: %s %s", cString, next.value)
						asMap[cString] = next.value
						// runningVal.(map[string]interface{})[cString] = next.value
						break
					}
					if isArray && cIntp {
						vlog("is leaf array: %v %v", cInt, next.value)
						asArray[0] = next.value
						break
					}
					panic("what")
				}

				var nextVal interface{}

				if nStringp {
					nextVal = map[string]interface{}{}
				} else if nIntp {
					// throwing this value away
					nextVal = []interface{}{nil}
				}

				if isMap {
					asMap[cString] = nextVal
					runningVal = asMap[cString]
				} else if isArray {
					asArray[0] = nextVal
				}

				runningNode = next
			}

			vlog("resulting map: %v", base)

			if err := mergo.Merge(&result, base, mergo.WithAppendSlice); err != nil {
				panic(err)
			}

			return n.value, nil
		})

	return result
}

func (path NodePath) ToString() string {
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
func toPath(s string) NodePath {
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

func (n Node) isLeaf() bool {
	return len(n.children) == 0
}

func (n *Node) Add(v interface{}, path_ ...interface{}) *Node {
	path := NodePath(path_)

	if len(path) == 0 {
		n.value = v
		return n
	}

	child, err := n.find(path[0])
	if err == nil {
		child.Add(v, path[1:]...)
	} else {
		new := &Node{value: path[0], children: []*Node{}}
		n.children = append(n.children, new.Add(v, path[1:]...))
	}
	return n
}

func (n Node) find(path_ ...interface{}) (Node, error) {
	path := NodePath(path_)

	if len(path) == 0 {
		return n, nil
	}

	vlog("find: %s", path.ToString())

	for _, n_ := range n.children {
		vlog("find: compare %v == %v ?", path[0], n_.value)
		if n_.value == path[0] {
			vlog("find: found %s!", path[0])
			return n_.find(path[1:]...)
		}
	}

	return Node{}, errors.New("Couldn't find path! " + path.ToString())
}

func (n *Node) changeLeaves(path NodePath, operation func(*Node, NodePath) (interface{}, error)) error {
	path = append(path, n.value)

	if n.isLeaf() {
		newVal, err := operation(n, path)
		if err != nil {
			vlog("transform err at node %s", path.ToString())
			panic(err)
		}

		if newVal != n.value {
			vlog("update: '%s' '%v' -> '%v'", path.ToString(), n.value, newVal)
			n.value = newVal
		}
	}

	for _, n_ := range n.children {
		n_.changeLeaves(path, operation)
	}

	return nil
}

func (n Node) ToFlatMap() map[string]string {
	results := map[string]string{}
	n.changeLeaves([]interface{}{},
		func(n *Node, path NodePath) (interface{}, error) {
			results[path.ToString()] = fmt.Sprintf("%v", n.value)
			return n.value, nil
		})
	return results
}

func (n Node) View(kind string) (string, error) {
	switch kind {
	case "toml":
		b, err := toml.Marshal(n.ToMap())
		if err != nil {
			panic(err)
		}
		return string(b), nil
	case "keys":
		keys := []string{}
		for k, _ := range n.ToFlatMap() {
			keys = append(keys, k)
		}
		return strings.Join(keys, "\n") + "\n", nil
	case "shell":
		var b bytes.Buffer
		// todo: this should handle arrays and 1d tables
		for k, v := range n.ToFlatMap() {
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

// func (c config) Render(tmpl *template.Template, template_text string) string {
// 	t := template.Must(tmpl.Parse(template_text))
// 	result := new(bytes.Buffer)
// 	err := t.Execute(result, c)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return result.String()
// }

