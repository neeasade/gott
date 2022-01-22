package main

import (
	"fmt"
	"regexp"
	"strings"
)

func (n *Node) resolveSplices(path NodePath, rootNode *Node) error {
	path = append(path, n.value)

	if n.value == "-" {
		parentPath := path[1 : len(path)-1]
		parent := rootNode.mustFind(parentPath...)

		for _, c := range n.children {
			splicePath := toPath(c.children[0].value.(string))
			parent.children = append(parent.children, rootNode.mustFind(splicePath...).children...)
		}

		// remove self
		spliceIndex := 0
		for i, c := range parent.children {
			if c.value == "-" {
				spliceIndex = i
			}
		}
		parent.children = append(parent.children[:spliceIndex], parent.children[spliceIndex+1:]...)
	}

	for _, n_ := range n.children {
		n_.resolveSplices(path, rootNode)
	}

	return nil
}

// The go template default selection mechanism kinda sucks
// you can't used dashes or numbers, even if they are keys in a table
// invalid: {{.wow.0}} {{.wow-ok}}
// but we want that (mostly dashes). so we'll take every selection and turn it into an index function call.
func identTransform(v string) (string, error) {
	identRe := regexp.MustCompile(`({{)[^{}\.]*((\.[a-zA-Z0-9-_]+)+)[^{}]*(}})`)
	// reference
	// match: {{sub .a.some-test 1}}
	// Match groups :
	// 0	-	{{
	// 1	-	.a.some-test
	// 2	-	.some-test
	// 3	-	}}

	// have to be a little delicate -- tracking moving point of the edit as we replace larger
	// strings at indexes
	delta := 0

	matches := identRe.FindAllStringSubmatchIndex(v, -1)
	// matches and submatches are identified by byte index pairs within the input string:
	// result[2*n:2*n+1] identifies the indexes of the nth submatch. The pair for n==0 identifies the
	// match of the entire expression
	for _, groups := range matches {
		toString := func(n int) string {
			return v[delta+groups[2*n] : delta+groups[2*n+1]]
		}
		fullMatch := toString(0)
		vlog("identTransform: %v", fullMatch)
		ident := toString(2)
		start := groups[2*2] + delta
		end := groups[2*2+1] + delta
		length := end - start

		// you're always replacing at the ident location, it's just a question of adding the ()
		addingBraces := strings.ReplaceAll(fullMatch, " ", "") != "{{"+ident+"}}"

		new := toPath(ident[1:]).ToIndexCall()
		if addingBraces {
			new = fmt.Sprintf("(%s)", new)
		}
		vlog("identTransform: new %v", new)

		v = v[:start] + new + v[end:]
		delta = delta + len(new) - length
	}

	return v, nil
}

func qualifyTransform(n *Node, path NodePath, rootNode Node) (interface{}, error) {
	if fmt.Sprintf("%T", n.value) != "string" {
		return n.value, nil
	}
	path = path[1:]

	v := n.value.(string)
	identRe := regexp.MustCompile("({{| )((\\.[a-zA-Z0-9-]+)+)")
	matches := identRe.FindAllStringSubmatchIndex(fmt.Sprintf("%v", v), -1)
	parentPath := path[0 : len(path)-1]

	if len(matches) > 0 {
		vlog("qualifying: .%s: '%s'", parentPath.ToString(), v)
	}

	delta := 0
	for _, groups := range matches {
		toString := func(n int) string {
			return v[delta+groups[2*n] : delta+groups[2*n+1]]
		}
		start := groups[2*2] + delta
		end := groups[2*2+1] + delta
		length := end - start

		matchPath := toPath(toString(2)[1:])
		matchKey := matchPath[0]

		// vlog("parent: %v", parent)
		// vlog("looking at: .%s", path.ToString())
		vlog("matchKey, matchPath: %s, %v", matchKey, matchPath)

		disqualify := func(cond bool, message string) bool {
			if cond {
				vlog("disqualified: %s", message)
			}
			return cond
		}

		_, err := rootNode.mustFind(parentPath...).find(matchPath...)
		inParent := err != nil

		u, digErr := rootNode.find(matchPath...)
		isValue := len(u.children) == 1

		if disqualify(matchKey == path[len(path)-1], "self") ||
			disqualify(!inParent, "not present in parent") ||
			disqualify(digErr == nil && isValue, "matchPath exists in map") {
			continue
		}

		// it's a sibling! qualify it:
		newPath := append(parentPath[0:len(parentPath)-1], matchPath...)
		new := "." + newPath.ToString()

		v = v[:start] + new + v[end:]
		delta = delta + len(new) - length
	}
	if len(matches) > 0 {
		vlog("qualify result: .%s: '%s'", path.ToString(), v)
	}
	return v, nil
}
