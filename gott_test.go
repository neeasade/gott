package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplice(t *testing.T) {
	verbose = true
	n := NewNode("root")
	n.add("subtable", "a", "{{.ok}}")
	n.add("table", "ok", "foo")
	n.add("table", "-", 0, "subtable")

	n.resolveSplices(NodePath{}, n)
	n.mustFind("table", "a", "{{.ok}}")
}

func TestNode(t *testing.T) {
	n := NewNode("root",
		NewNode("a", NewNode("b")),
		NewNode("array",
			NewNode(0, NewNode("b")),
			NewNode(1, NewNode("c")),
		),
	)

	assert.Equal(t, "b", n.mustFind("a", "b").value)

	mapExpected := map[string]interface{}{
		"root": map[string]interface{}{
			"a":     "b",
			"array": []interface{}{"b", "c"},
		},
	}

	assert.Equal(t, mapExpected, n.toMap())

	n.changeLeaves([]interface{}{},
		func(n *Node, _ NodePath) (interface{}, error) {
			return n.value.(string) + "foo", nil
		})

	assert.Equal(t, "bfoo", n.mustFind("a", "bfoo").value)
}

func TestFromMap(t *testing.T) {

	verbose = true
	data := map[string]interface{}{
		// "a":     "b",
		"array": []interface{}{"b", "c"},
	}

	n := Node{value: "r", children: []*Node{}}

	s, _ := n.view("mapString")
	vlog("arst: %s", s)

	mapToNode(&n, data, NodePath{})

	s, _ = n.view("mapString")
	vlog("arst2: %s", s)

	// n.mustFind("a", "b")
	// n.mustFind("array", 0)
	vlog("arst: %v", n.mustFind("array"))
	n.mustFind("array", 1)
}

func TestToMap(t *testing.T) {
	n := NewNode("root",
		NewNode("a",
			NewNode(0, NewNode("zero")),
			NewNode(1, NewNode(2)),
		),
		NewNode("b", NewNode("b")),
	)

	mapExpected := map[string]interface{}{
		"root": map[string]interface{}{
			"a": []interface{}{"zero", 2},
			"b": "b",
		},
	}

	assert.Equal(t, mapExpected, n.toMap())
}

func TestAdd(t *testing.T) {
	verbose = true
	n := NewNode("root")
	n.add("a", 1)
	n.add("b", 2)
	n.add("array", 0, "a")
	n.add("array", 1, "b")

	n.mustFind("array", 1, "b")

	mapExpected := map[string]interface{}{
		"root": map[string]interface{}{
			"a":     1,
			"b":     2,
			"array": []interface{}{"a", "b"},
		},
	}

	assert.Equal(t, mapExpected, n.toMap())
}
