package main

import (
	"bytes"
	"errors"
	"crypto/md5"
	"encoding/binary"
	"encoding/gob"
	"flag"
	"fmt"
	"path/filepath"
	"log"
	"os"
	"strings"
	"text/template"

	"golang.org/x/sync/errgroup"
	"github.com/pelletier/go-toml/v2"
	"github.com/Masterminds/sprig"
	"github.com/imdario/mergo"
)

type config map[string]interface{}

func flattenMap(results map[string]string, m map[string]interface{}, namespace string) {
	if namespace != "" {
		namespace = namespace + "."
	}

	for key, value := range m {
		nested, is_map := value.(map[string]interface{})
		_, is_array := value.([]interface{})
		if is_map {
			flattenMap(results, nested, namespace+key)
		} else if is_array {
			// do nothing (string array indexes sounds gross)
			// for index, _ := range arrayVal {
			// 	index_string := fmt.Sprintf("[%i]", index)
			// 	flattenMap(nested, namespace + key + index_string, results)
			// }
		} else {
			results[namespace+key] = fmt.Sprintf("%v", value)
		}
	}
}

func (c config) Flatten() map[string]string {
	results := map[string]string{}
	flattenMap(results, c, "")
	return results
}

func (c config) Render(template_text string) (string, error) {
	t := template.Must(template.New("base").Funcs(sprig.TxtFuncMap()).Parse(template_text))
	result := new(bytes.Buffer)
	err := t.Execute(result, c)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

// Have a config render itself
func realizeConfig(subConfig config, rootConfig config) error {
	// realizedConfig := map[string]interface{}{}
	for key, value := range subConfig {
		switch value.(type) {
		case map[string]interface{}:
			err := realizeConfig(value.(map[string]interface{}), rootConfig)
			if err != nil {
				return err
			}

		case string:
			realizedValue, err := subConfig.Render(value.(string))
			if err == nil {
				subConfig[key] = realizedValue
				continue
			}

			realizedValue, err = rootConfig.Render(value.(string))
			if err == nil {
				subConfig[key] = realizedValue
				continue
			}

			return errors.New("failed to render template for key")

		default:
			subConfig[key] = value
		}
	}

	return nil
}

func parseToml(tomlFiles, tomlText []string) map[string]interface{} {
	result := map[string]interface{}{}

	// reverse tomlFiles
	for left, right := 0, len(tomlFiles)-1; left < right; left, right = left+1, right-1 {
		tomlFiles[left], tomlFiles[right] = tomlFiles[right], tomlFiles[left]
	}

	for _, file := range tomlFiles {
		bytes, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("TOML file not found: %s", file)
		}
		// "prepend"
		tomlText = append([]string{string(bytes)}, tomlText...)
	}

	for _, text := range tomlText {
		var parsed map[string]interface{}
		err := toml.Unmarshal([]byte(text), &parsed)
		if err != nil {
			// NB: need to match against length
			// see https://stackoverflow.com/questions/27252152/how-to-check-if-a-slice-has-a-given-index-in-go
			// if file := tomlFiles[i]; {
			// 	println("err in toml file: %s", file)
			// }
			panic(err)
		}
		if err := mergo.Merge(&result, parsed); err != nil {
			panic(err)
		}
	}

	return result
}

func getCachedConfig(tomlFiles, tomlText []string) (chan config, string, error) {
	sumStrings := append(tomlFiles, tomlText...)
	sumBytes := md5.Sum([]byte(strings.Join(sumStrings, "")))
	sumInt := binary.BigEndian.Uint64(sumBytes[:])
	cacheFile := os.Getenv("HOME") + "/.cache/gott/" + fmt.Sprintf("%v", sumInt)

	cacheInfo, cacheErr := os.Stat(cacheFile)
	if cacheErr != nil {
		return nil, cacheFile, errors.New("no cache file")
	}

	g := new(errgroup.Group)
	gob.Register(map[string]interface{}{})

	cacheChan := make(chan config)

	go func() {
		decoded := config{}
		b, err := os.ReadFile(cacheFile)
		d := gob.NewDecoder(bytes.NewReader(b))
		err = d.Decode(&decoded)
		if err != nil {
			panic(err)
		}
		cacheChan <- decoded
	}()

	cacheTime := cacheInfo.ModTime()
	for _, f := range tomlFiles {
		g.Go(func() error {
			info, _ := os.Stat(f)
			if info.ModTime().After(cacheTime) {
				return errors.New("toml file changed")
			}
			return nil
		})
	}

	return cacheChan, cacheFile, g.Wait()
}

// add an array type for flag
type arrayFlag []string

func (i *arrayFlag) String() string {
	return ""
}

func (i *arrayFlag) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	var tomlFiles, renderTargets, tomlText arrayFlag
	var action, queryString, queryStringPlain string

	flag.Var(&tomlFiles, "t", "Add a toml file to consider")
	flag.Var(&tomlText, "T", "Add raw toml to consider")
	flag.Var(&renderTargets, "r", "Render a file")
	flag.StringVar(&action, "o", "", "Output type <shell|toml>")
	flag.StringVar(&queryString, "q", "", "Render a string (implicit surrounding {{}})")
	flag.StringVar(&queryStringPlain, "R", "", "Render a string")

	flag.Parse()

	cacheChan, cacheFile, cacheErr := getCachedConfig(tomlFiles, tomlText)

	config := config{}

	if cacheErr == nil {
		config = <-cacheChan
	} else {
		config = parseToml(tomlFiles, tomlText)
		err := realizeConfig(config, config)
		if err != nil {
			panic(err)
		}
	}

	switch action {
	case "toml":
		b, err := toml.Marshal(config)
		if err != nil {
			panic(err)
		}
		fmt.Print(string(b))
	case "keys":
		for k, _ := range config.Flatten() {
			fmt.Println(k)
		}
	case "shell":
		for k, v := range config.Flatten() {
			k = strings.ReplaceAll(k, ".", "_")
			// // meh on this replace value
			k = strings.ReplaceAll(k, "-", "_")
			v = strings.ReplaceAll(v, "'", "'\\''")
			fmt.Printf("%s='%s'\n", k, v)
		}
	}

	for _, file := range renderTargets {
		bytes, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("render file not found: %s", file)
		}
		fmt.Println(config.Render(string(bytes)))
	}

	if queryString != "" {
		fmt.Println(config.Render("{{" + queryString + "}}"))
	}

	if queryStringPlain != "" {
		fmt.Println(config.Render(queryString))
	}

	if cacheErr != nil {
		gob.Register(map[string]interface{}{})
		// gob.Register(config)
		b := new(bytes.Buffer)
		e := gob.NewEncoder(b)
		err := e.Encode(config)
		if err != nil {
			panic(err)
		}

		os.MkdirAll(filepath.Dir(cacheFile), os.ModePerm)
		os.WriteFile(cacheFile, b.Bytes(), os.ModePerm)
	}
	// todo: cache eviction/cleanup
}
