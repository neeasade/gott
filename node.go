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

type NodePath []interface{} // collection of strings and ints leading to a node

type Node struct {
	// value is a string, int, date (ie a non-map, non-array)
	// could be a leaf value, or an identifier (map string key or array index)
	value    interface{}
	children []*Node
}

func NewNode(value interface{}, children_ ...*Node) *Node {
	var children = []*Node{}
	return &Node{value: value, children: append(children, children_...)}
}

func (n Node) isLeaf() bool {
	return len(n.children) == 0
}

func (n *Node) walk(path NodePath, operation func(NodePath) error) error {
	path = append(path, n.value)

	err := operation(path)
	vlog("walk: %s", path.ToString())
	if err != nil {
		vlog("walk err at node %s", path.ToString())
		return err
	}

	for _, n_ := range n.children {
		n_.walk(path, operation)
	}

	return nil
}

func (n *Node) changeLeaves(path NodePath, operation func(NodePath) (interface{}, error)) error {
	path = append(path, n.value)

	if n.isLeaf() {
		newVal, err := operation(path)
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

func (n *Node) copy() *Node {
	// lol
	ret := NewNode(n.value)
	n.changeLeaves(NodePath{},
		func(path NodePath) (interface{}, error) {
			ret.add(path[1:]...)
			return path.last(), nil
		})
	return ret
}

func (n *Node) remove(path_ ...interface{}) {
	path := NodePath(path_)
	children := []*Node{}

	var parent *Node
	if (len(path) == 1) {
		parent = n
	} else {
		parent = n.mustFind(path[:len(path)-1]...)
	}

	for _, c := range parent.children {
		if c.value != path.last() {
			children = append(children, c)
		}
	}
	parent.children = children
}

func (n *Node) add(path_ ...interface{}) {
	path := NodePath(path_)

	if len(path) == 0 {
		return
	}

	child, err := n.find(path[0])

	if err == nil {
		child.add(path[1:]...)
	} else {
		var new *Node = NewNode(path[0])
		vlog("add: making new node %v ON %s", new.value, n.value)
		new.add(path[1:]...)
		n.children = append(n.children, new)
	}
}

func (root *Node) toMap() map[string]interface{} {
	result := map[string]interface{}{}

	root.changeLeaves([]interface{}{},
		func(path NodePath) (interface{}, error) {
			// vlog("map from leaf: %s", path.ToString())
			path = path[1:]

			var base interface{} = map[string]interface{}{}
			var runningVal interface{} = base

			runningNode := root

			for _, p := range path {
				// vlog("base: %v", base)

				next, err := runningNode.find(p)
				if err != nil {
					panic(err)
				}

				asMap, isMap := runningVal.(map[string]interface{})
				asArray, isArray := runningVal.([]interface{})

				cString, cStringp := runningNode.value.(string)
				// cInt, cIntp  := runningNode.value.(int)
				_, cIntp := runningNode.value.(int)

				_, nStringp := next.value.(string)
				_, nIntp := next.value.(int)

				if next.isLeaf() {
					// vlog("next is leaf")
					if isMap && cStringp {
						// vlog("is leaf map: %s %s", cString, next.value)
						asMap[cString] = next.value
						// runningVal.(map[string]interface{})[cString] = next.value
						break
					}
					if isArray && cIntp {
						// vlog("is leaf array: %v %v", cInt, next.value)
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

			// vlog("resulting map: %v", base)

			if err := mergo.Merge(&result, base, mergo.WithAppendSlice); err != nil {
				panic(err)
			}

			return path.last(), nil
		})

	return result
}

// doesn't include surrounding {{}}/()
func (path NodePath) ToIndexCall() string {
	result := "index . "
	for _, v := range path {
		switch v := v.(type) {
		case string:
			result = result + fmt.Sprintf(" \"%s\" ", v)
		case int:
			result = result + fmt.Sprintf(" %d ", v)
		}
	}

	return result
}

func (path NodePath) last() interface{} {
	return path[len(path)-1]
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
		result = result[:len(result)-1]
	}
	return result
}

func toPath(s string) NodePath {
	path := []interface{}{}
	for _, v := range strings.Split(s, ".") {
		i, err := strconv.Atoi(v)
		if err == nil {
			path = append(path, i)
		} else {
			path = append(path, v)
		}
	}
	return path
}

func (n *Node) find(path_ ...interface{}) (*Node, error) {
	if len(path_) == 1 {
		_, isArray := path_[0].([]interface{})
		if isArray {
			panic("passed array into find! splice it")
		}
	}

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

	return &Node{}, errors.New("Couldn't find path! " + path.ToString())
}

func (n *Node) mustFind(path_ ...interface{}) *Node {
	r, err := n.find(path_...)
	if err != nil {
		panic(err)
	}
	return r
}

func (n Node) toFlatMap() map[string]string {
	results := map[string]string{}
	n.changeLeaves(NodePath{},
		func(path NodePath) (interface{}, error) {
			// remove "root", value node
			results[path[1:len(path)-1].ToString()] = fmt.Sprintf("%v", n.value)
			return path.last(), nil
		})
	return results
}

func (n Node) view(kind string) (string, error) {
	switch kind {
	case "mapString":
		result := ""
		for k, v := range n.toFlatMap() {
			result = result + fmt.Sprintf("\n%s: %s", k, v)
		}
		return result, nil
	case "toml":
		b, err := toml.Marshal(n.toRenderMap())
		if err != nil {
			panic(err)
		}
		return string(b), nil
	case "keys":
		keys := []string{}
		for k, _ := range n.toFlatMap() {
			keys = append(keys, k)
		}
		return strings.Join(keys, "\n") + "\n", nil
	case "shell":
		var b bytes.Buffer
		// todo: this should handle arrays and 1d tables
		for k, v := range n.toFlatMap() {
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

func (n Node) toRenderMap() map[string]interface{} {
	// ehhhhh
	return n.toMap()["root"].(map[string]interface{})
}

func (n Node) render(tmpl *template.Template, text string) (string, error) {
	t := template.Must(tmpl.Parse(text))
	result := new(bytes.Buffer)
	m := n.toRenderMap()

	vlog("-----render map: ")
	for k, v := range m {
		vlog("%s: %v", k, v)
	}
	// vlog("render map: %v", m)

	vlog("rendering (inner): %s", text)
	err := t.Execute(result, m)
	return result.String(), err
}

func (n Node) mustRender(tmpl *template.Template, text string) (string) {
	r, err := n.render(tmpl, text)
	if err != nil {
		panic(err)
	}
	return r
}

func (n *Node) promote(path NodePath) {
	child := n.mustFind(path...)
	n.children = append(n.children, child.children...)
}

