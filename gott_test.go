package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNode(t *testing.T) {
	verbose = true

	n := Node{"root",
		[]*Node{
			&Node{"a",
				[]*Node{
					&Node{"b", []*Node{}},
				},
			},
			&Node{"array",
				[]*Node{
					&Node{0,
						[]*Node{
							&Node{"b", []*Node{}},
						},
					},
					&Node{1,
						[]*Node{
							&Node{"c", []*Node{}},
						},
					},
				},
			},
		},
	}

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

func TestToMap(t *testing.T) {
	n := Node{"root",
		[]*Node{
			&Node{"a",
				[]*Node{
					&Node{0, []*Node{&Node{"zero", []*Node{}}}},
					&Node{1, []*Node{&Node{2, []*Node{}}}},
				},
			},
			&Node{"b",
				[]*Node{
					&Node{"b", []*Node{}},
				},
			},
		},
	}

	mapExpected := map[string]interface{}{
		"root": map[string]interface{}{
			"a": []interface{}{"zero", 2},
			"b": "b",
		},
	}

	assert.Equal(t, mapExpected, n.toMap())
}

func TestAdd(t *testing.T) {
	n := Node{"root", []*Node{}}
	n.add("a", 1)
	n.add("b", 2)

	mapExpected := map[string]interface{}{
		"root": map[string]interface{}{
			"a": 1,
			"b": 2,
		},
	}

	assert.Equal(t, mapExpected, n.toMap())
}
