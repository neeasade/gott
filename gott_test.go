package main

import "testing"
import "reflect"

func TestNode(t *testing.T) {
	verbose = true

	equal := func(expected, result interface{}) {
		if !reflect.DeepEqual(expected, result) {
			t.Fatalf("Expected vs Result:\n%v\n%v", expected, result)
		}
	}

	findEqual:= func(expected interface{}, n Node, path ...interface{}) {
		r, err := n.find(path...)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		equal(expected, r.value)
	}

	n := Node {"root",
		[]*Node{
			&Node{"a",
				[]*Node{
					&Node{"b", []*Node{}},
					&Node{"c", []*Node{}},
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

	findEqual("b", n, "a", "b")

	n.changeLeaves([]interface{}{},
		func(n *Node, _ NodePath) (interface{}, error) {
			return n.value.(string) + "foo", nil
		})

	findEqual("bfoo", n, "a", "bfoo")


	n = Node {"root",
		[]*Node{
			&Node{"a",
				[]*Node{
					&Node{"a", []*Node{}},
				},
			},
			&Node{"b",
				[]*Node{
					&Node{"b", []*Node{}},
				},
			},
		},
	}

	// toml top level
	// a = "a"
	// b = "b"
	mapExpected := map[string]interface{}{
		"root": map[string]interface{}{
			"a": "a",
			"b": "b",
		},
	}

	equal(mapExpected, n.ToMap())

	n = Node {"root",
		[]*Node{
			&Node{"a",
				[]*Node{
					&Node{0, []*Node{&Node{"zero", []*Node{}},}},
					&Node{1, []*Node{&Node{2, []*Node{}},}},
				},
			},
			&Node{"b",
				[]*Node{
					&Node{"b", []*Node{}},
				},
			},
		},
	}

	mapExpected = map[string]interface{}{
		"root": map[string]interface{}{
			"a": []interface{}{"zero", 2},
			"b": "b",
		},
	}

	equal(mapExpected, n.ToMap())

	n = Node{"root", []*Node{}}
	n.Add("a", 1)
	n.Add("b", 2)

	mapExpected = map[string]interface{}{
		"root": map[string]interface{}{
			"a": 1,
			"b": 2,
		},
	}

	equal(mapExpected, n.ToMap())
}
