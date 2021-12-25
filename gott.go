package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/gob"
	"flag"
	"fmt"
	// "io"
	"log"
	"os"
	// "os/exec"
	// "regexp"
	"strings"
	"html/template"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/Masterminds/sprig"
	"github.com/imdario/mergo"
)

// this could probably be a type alias
type config struct {
    data map[string]interface{}
}

// return a copy of self with path promoted to top level data
func (c config) Promote(path []string) config {
	// fake a copy
	result := map[string]interface{}{}
	mergo.Merge(&result, c.data)

	var dig map[string]interface{} = c.data
	for _, key := range path {
		dig = dig[key].(map[string]interface{})
	}

	if err := mergo.Merge(&result, dig); err != nil {
		panic(err)
	}

	return config{data: result}
}

func (c config) Render(template_text string) string {
	t := template.Must(template.New("base").Funcs(sprig.FuncMap()).Parse(template_text))

	result := new(bytes.Buffer)
	err := t.Execute(result, c.data)

	if err != nil {
		panic(err)
	}
	return result.String()
}

func walk(m map[string]interface{}, config config, path []string) map[string]interface{} {
	for k, v := range m {
		// path =
		switch v := v.(type) {
		// case []interface{}:
			// for i, v := range v {
			// 	// m[k][i] = walk(v, config, path)
			// }
		case map[string]interface{}:
			m[k] = walk(v, config, append(path, k))
		case string:
			m[k] = config.Promote(path).Render(v)
		}
	}
	return m
}

func (c config) Process() {

}

func act(config map[string]string, renderTargets []string, action, queryString string) {
	switch action {
	case "toml":
		b, err := toml.Marshal(config)
		if err != nil {
			panic(err)
		}
		fmt.Print(string(b))
	case "keys":
		for k, _ := range config {
			// todo: build a stringified path version
			fmt.Println(k)
		}
	case "shell":
		for k, v := range config {
			k = strings.ReplaceAll(k, ".", "_")

			// meh on this replace value
			k = strings.ReplaceAll(k, "-", "_")

			v = strings.ReplaceAll(v, "'", "'\\''")
			fmt.Printf("%s='%s'\n", k, v)
		}
	}

	// for _, file := range renderTargets {
	// 	// bytes, err := os.ReadFile(file)
	// 	// if err != nil {
	// 	// 	log.Fatalf("file not found: %s", f)
	// 	// }
	// 	// fmt.Println(mustache(config, string(bytes)))
	// }

	if queryString != "" {
		if result, ok := config[queryString]; ok {
			fmt.Println(result)
		} else {
			log.Fatal("query not found")
		}
	}
}

func reverse(input []string) []string {
    var output []string

    for i := len(input) - 1; i >= 0; i-- {
        output = append(output, input[i])
    }

    return output
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

func getConfig(tomlFiles, tomlText []string) map[string]string {
	config := map[string]string{}

	sumStrings := append(tomlFiles, tomlText...)
	sumBytes := md5.Sum([]byte(strings.Join(sumStrings, "")))
	sumInt := binary.BigEndian.Uint64(sumBytes[:])
	cache_file := os.Getenv("HOME") + "/.cache/gott/" + fmt.Sprintf("%v", sumInt)

	cached := true

	cacheInfo, err := os.Stat(cache_file)
	if err != nil {
		cached = false
	}

	cached = false

	if cached {
		cache_chan := make(chan map[string]string)

		go func() {
			decoded_config := map[string]string{}
			b, err := os.ReadFile(cache_file)
			d := gob.NewDecoder(bytes.NewReader(b))
			err = d.Decode(&decoded_config)
			if err != nil {
				panic(err)
			}
			cache_chan <- decoded_config
		}()

		checkTime := func(f string, cache_time time.Time) chan bool {
			c := make(chan bool)
			go func() {
				info, err := os.Stat(f)
				if err != nil {
					log.Fatalf("err when stating file '%s': %s", f, err)
				}
				c <- cache_time.After(info.ModTime())
			}()
			return c
		}

		timeChannels := make([]chan bool, len(tomlFiles))
		cacheTime := cacheInfo.ModTime()
		for i, f := range tomlFiles {
			timeChannels[i] = checkTime(f, cacheTime)
		}

		for _, c := range timeChannels {
			cached = cached && <-c
			if ! cached {
				break
			}
		}

		if cached {
			return <-cache_chan
		}
	}

	// for key, value := range config {
	// 	config[key] = render(config, value, parent(key, "."))
	// }

	b := new(bytes.Buffer)
	e := gob.NewEncoder(b)
	err = e.Encode(config)
	if err != nil {
		panic(err)
	}

	var perm os.FileMode = 0o644
	// todo: this should be fs/parent
	// os.MkdirAll(parent(cache_file, "/"), 0700)

	// todo: yell on fail
	os.WriteFile(cache_file, b.Bytes(), perm)
	// todo: cache eviction/cleanup

	return config
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
	var tomlFiles, promotions, renderTargets, tomlText arrayFlag
	var action, queryString, narrow string

	flag.Var(&tomlFiles, "t", "Add a toml file to consider")
	flag.Var(&tomlText, "T", "Add raw toml to consider")
	flag.Var(&promotions, "p", "Promote a namespace to the top level")
	flag.Var(&renderTargets, "r", "Render a file")
	flag.StringVar(&action, "o", "", "Output type <shell|toml>")
	flag.StringVar(&queryString, "q", "", "Query for a value (implicit surrounding @{})")
	flag.StringVar(&narrow, "n", "", "Narrow the namespaces to consider")

	flag.Parse()

	// test.data = map[string]interface{}{"wow": map[string]interface{}{"really": "ok"}}

	test := config{}
	test.data = parseToml(tomlFiles, tomlText)

	// result := test.Render("{{env \"HOME\"}}")
	// result := test.Render("{{.font.family}}")

	walk(test.data, test, []string{})

	// fmt.Printf("%v\n", test.data)
	// fmt.Printf("%v\n", newOne.data)

	result := test.Render("{{.font.config}}")
	println(result)

	os.Exit(0)


	// flow should look something like

	// result, err := trycache
	// else

	config := getConfig(tomlFiles, tomlText)

	// for _, p := range promotions {
	// 	promoteNamespace(config, p)
	// }

	// if narrow != "" {
	// 	// config = narrowToNamespace(config, narrow)
	// }

	act(config, renderTargets, action, queryString)
}
