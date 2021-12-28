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

// the most esteemed type that exists
type config map[string]interface{}

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

func (c config) Promote(path []string) config {
	result := config{}
	mergo.Merge(&result, c)

	zoom, _, _ := c.Dig(path)
	if err := mergo.Merge(&c, zoom); err != nil {
		panic(err)
	}
	return c
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
