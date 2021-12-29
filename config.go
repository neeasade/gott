package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/imdario/mergo"
	"github.com/pelletier/go-toml/v2"
)

func (c config) Flatten() map[string]string {
	results := map[string]string{}

	add := func(n interface{}, c config, path []interface{}) (interface{}, error) {
		results[toString(path)] = fmt.Sprintf("%v", n)
		return n, nil
	}

	c.Transform(c, []interface{}{}, add)
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

func (c config) Promote(path []interface{}) config {
	result := config{}
	mergo.Merge(&result, c)

	// todo: this should handle or complain about non-map results
	zoom, _ := Dig(c, path)

	if err := mergo.Merge(&c, zoom); err != nil {
		panic(err)
	}
	return c
}

func Set(n interface{}, path []interface{}, value interface{}) (interface{}, error) {
	if len(path) == 0 {
		return nil, errors.New("shouldn't be reached (path 0)")
	}

	switch n := n.(type) {
	case config:
		// there is no fallthrough in type switches
		if len(path) == 1 {
			n[path[0].(string)] = value
			return nil, nil
		} else {
			return Set(n[path[0].(string)], path[1:], value)
		}
	case map[string]interface{}:
		if len(path) == 1 {
			n[path[0].(string)] = value
			return nil, nil
		} else {
			return Set(n[path[0].(string)], path[1:], value)
		}
	case []interface{} :
		if len(path) == 1 {
			n[path[0].(int)] = value
			return nil, nil
		} else {
			return Set(n[path[0].(int)], path[1:], value)
		}
	default:
		vlog("weird type: %T", n)
		return n, errors.New("Tried to set weird type")
	}

	return nil, errors.New("shouldn't be reached")
}

func Dig(n interface{}, pathIn interface{}) (interface{}, error) {
	path := pathIn.([]interface{})

	if len(path) == 0 {
		return n, nil
	}

	switch n := n.(type) {
	    case map[string]interface{}:
		return Dig(n[path[0].(string)], path[1:])
	case []interface{} :
		// todo -- coerce to int?
		return Dig(n[path[0].(int)], path[1:])
	    default:
		return n, errors.New("not reached")
	}
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

func (c config) Transform(n interface{}, path []interface{}, operation func(interface{}, config, []interface{}) (interface{}, error)) {
	// vlog("\ntransforming at %s: %T", toString(path), n)
	switch n := n.(type) {
	case config:
		for k, v := range n {
			// vlog("nesting: %s into %s", toString(path), toString(append(path, k)))
			c.Transform(v, append(path, k), operation)
		}
	case map[string]interface{}:
		for k, v := range n {
			c.Transform(v, append(path, k), operation)
		}
	case []interface{}:
		for i, v := range n {
			c.Transform(v, append(path, i), operation)
		}
	default:
		// hack: go up a level, set it to something
		result, err := operation(n, c, path)
		if err != nil {
			panic(err)
		}
		if n != result {
			vlog("updating node '%s': %v -> %v", toString(path), n, result)
			// _, err = Set(c, path[0:len(path)-1], result)
			_, err = Set(c, path, result)
			if err != nil {
				panic(err)
			}
		}
	}
}
