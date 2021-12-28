package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The go template default selection mechanism kinda sucks
// you can't used dashes or numbers, even if they are keys in a table
// invalid: {{.wow.0}} {{.wow-ok}}
// but we want that (mostly dashes). so we'll take every selection and turn it into an index function call.
func identTransform(v string, c config, path []string) (string, error) {
	identRe := regexp.MustCompile("({{)[^{}\\.]*((\\.[a-zA-Z0-9-]+)+)[^{}]*(}})")
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

	matches := identRe.FindAllStringSubmatchIndex(fmt.Sprintf("%v", v), -1)
	// matches and submatches are identified by byte index pairs within the input string:
	// result[2*n:2*n+1] identifies the indexes of the nth submatch. The pair for n==0 identifies the
	// match of the entire expression
	for _, groups := range matches {
		toString := func(n int) string {
			return v[delta+groups[2*n] : delta+groups[2*n+1]]
		}
		fullMatch := toString(0)
		ident := toString(2)
		start := groups[2*2] + delta
		end := groups[2*2+1] + delta
		length := end - start

		// vlog("fullmatch: %s", fullMatch)

		// you're always replacing at the ident location, it's just a question of adding the ()
		addingBraces := strings.ReplaceAll(fullMatch, " ", "") != "{{"+ident+"}}"

		parts := strings.Split(ident[1:], ".")
		new := "index . "
		for _, p := range parts {
			pi, err := strconv.Atoi(p)
			if err == nil {
				new = fmt.Sprintf("%s %d", new, pi)
			} else {
				new = fmt.Sprintf("%s \"%s\"", new, p)
			}
		}

		if addingBraces {
			new = fmt.Sprintf("(%s)", new)
		}

		v = v[:start] + new + v[end:]
		delta = delta + len(new) - length
	}

	return v, nil
}

func qualifyTransform(v string, c config, path []string) (string, error) {
	identRe := regexp.MustCompile("({{| )((\\.[a-zA-Z0-9-]+)+)")
	matches := identRe.FindAllStringSubmatchIndex(fmt.Sprintf("%v", v), -1)
	parent, _, _ := c.Dig(path[0 : len(path)-1])

	if len(matches) > 0 {
		name := strings.Join(path, ".")
		vlog("\nqualifying .%s: %s", name, v)
	}

	delta := 0
	for _, groups := range matches {
		toString := func(n int) string {
			return v[delta+groups[2*n] : delta+groups[2*n+1]]
		}
		start := groups[2*2] + delta
		end := groups[2*2+1] + delta
		length := end - start

		matchPath := strings.Split(toString(2)[1:], ".")
		matchKey := matchPath[0]

		vlog("parent: %v", parent)
		vlog("looking at: .%s", strings.Join(path, "."))
		vlog("matchKey, matchPath: %s, %v", matchKey, matchPath)

		disqualify := func(cond bool, message string) bool {
			if cond {
				vlog("disqualified: %s", message)
			}
			return cond
		}

		_, in_parent := parent[matchKey]
		_, digIsMap, digErr := c.Dig(matchPath)

		if disqualify(matchKey == path[len(path)-1], "self") ||
			disqualify(!in_parent, "not present in parent") ||
			disqualify(digErr == nil && !digIsMap, "matchPath exists in map, and is a value") {
			continue
		}

		new := "." + strings.Join(append(path[0:len(path)-1], matchPath...), ".")
		v = v[:start] + new + v[end:]
		delta = delta + len(new) - length
	}
	return v, nil
}