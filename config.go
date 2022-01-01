package main

import (
	// "bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	// "text/template"

	"github.com/imdario/mergo"
	// "github.com/pelletier/go-toml/v2"
)

// func Scratch() {
// 	map[string]interface{}
// }

func (root Node) toMap() map[string]interface{} {
	result := map[string]interface{}{}

	root.changeLeaves([]interface{}{},
		func(n *Node, path []interface{}) (interface{}, error) {
			vlog("map from leaf: %s", toString(path))
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
				// cInt, cIntp  := runningNode.value.(int)

				// nString, nStringp := next.value.(string)
				_, nStringp := next.value.(string)
				// nInt, nIntp  := next.value.(int)
				_, nIntp  := next.value.(int)

				if next.isLeaf() {
					vlog("next is leaf")
					if isMap && cStringp {
						vlog("is leaf: %s %s", cString, next.value)
						asMap[cString] = next.value
						// runningVal.(map[string]interface{})[cString] = next.value
						break
					}
					if isArray {
						asArray = append(asArray, next.value)
						break
					}
				}

				var nextVal interface{}

				if nStringp {
					nextVal = map[string]interface{}{}
				} else if nIntp {
					nextVal = []interface{}{}
				}

				if isMap {
					asMap[cString] = nextVal
					runningVal = asMap[cString]
				} else if isArray {
					asArray = append(asArray, nextVal)
					runningVal = asArray[0]
				}

				runningNode = next
			}

			vlog("resulting map: %v", base)
			if err := mergo.Merge(&result, base); err != nil {
				panic(err)
			}

			return n.value, nil
		})

	return result
}

// func NodeToMap(n Node, path []interface{}, result interface{}) interface{} {
// }

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
	children []*Node
}

func (n Node) isLeaf() bool {
	return len(n.children) == 0
}

func (n Node) find(path ...interface{}) (Node, error) {
	if len(path) == 0 {
		return n, nil
	}

	vlog("find: %s", toString(path))

	for _, n_ := range n.children {
		vlog("find: compare %v == %v ?", path[0], n_.value)
		if n_.value == path[0] {
			vlog("find: found %s!", path[0])
			return n_.find(path[1:]...)
		}
	}

	return Node{}, errors.New("Couldn't find path! " + toString(path))
}

func (n *Node) changeLeaves(path []interface{}, operation func(*Node, []interface{}) (interface{}, error)) error {
	path = append(path, n.value)

	if n.isLeaf() {
		newVal, err := operation(n, path)
		if err != nil {
			vlog("transform err at node %s", toString(path))
			panic(err)
		}

		if newVal != n.value {
			vlog("update: '%s' '%v' -> '%v'", toString(path), n.value, newVal)
			n.value = newVal
		}
	}

	for _, n_ := range n.children {
		n_.changeLeaves(path, operation)
	}

	return nil
}

// func (c config) View(kind string) (string, error) {
// 	switch kind {
// 	case "toml":
// 		b, err := toml.Marshal(c)
// 		if err != nil {
// 			panic(err)
// 		}
// 		return string(b), nil
// 	case "keys":
// 		keys := []string{}
// 		for k, _ := range c.Flatten() {
// 			keys = append(keys, k)
// 		}
// 		return strings.Join(keys, "\n") + "\n", nil
// 	case "shell":
// 		var b bytes.Buffer
// 		// todo: this should handle arrays and 1d tables
// 		for k, v := range c.Flatten() {
// 			k = strings.ReplaceAll(k, ".", "_")
// 			// todo: meh on this replace value
// 			k = strings.ReplaceAll(k, "-", "_")
// 			v = strings.ReplaceAll(v, "'", "'\\''")
// 			fmt.Fprintf(&b, "%s='%s'\n", k, v)
// 		}
// 		return b.String(), nil
// 	}
// 	return "", errors.New("invalid view requested")
// }

// func (c config) Render(tmpl *template.Template, template_text string) string {
// 	t := template.Must(tmpl.Parse(template_text))
// 	result := new(bytes.Buffer)
// 	err := t.Execute(result, c)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return result.String()
// }

