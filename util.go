package main

import (
	"fmt"
	"io"
	"strings"
	"os"
	"strconv"
	"os/exec"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/imdario/mergo"
	"github.com/pelletier/go-toml/v2"
)

// "path" is a slice of strings+ints leading to a node in config
// todo: see about using type alises to define these methods on strings/interface{}?
func toPath(s string) []interface{} {
	path := []interface{}{}
	for _, v := range strings.Split(s, ".") {
		// todo: this should account for ints
		i, err := strconv.Atoi(v)
		if err == nil {
			path = append(path, i)
		} else {
			path = append(path, v)
		}
	}
	return path
}

// turn a path into a flat string like a.0.foo-bar
// nb: you can't define methods on interface{} (which makes sense)
func toString(path []interface{}) string {
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
		result = result[0:len(result)-1]
	}
	return result
}

func vlog(format string, args ...interface{}) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func makeTemplate() *template.Template {
	funcMap := (sprig.TxtFuncMap())
	funcMap["shpipe"] = func(command, value string) string {
		cmd := exec.Command("bash", "-c", command)
		stdin, _ := cmd.StdinPipe()

		go func() {
			defer stdin.Close()
			io.WriteString(stdin, value)
		}()

		out, err := cmd.CombinedOutput()
		if err != nil {
			glog.Fatal(err)
		}

		return string(out)
	}

	funcMap["sh"] = func(command string) string {
		out, err := exec.Command("bash", "-c", command).Output()
		if err != nil {
			glog.Fatal(err)
		}
		return string(out)
	}

	funcMap["eq"] = func(a, b interface{}) bool { return a == b }

	// if verbose {
	// 	vlog("loaded funcmap functions are:")
	// 	for k, _ := range funcMap {
	// 		vlog(k)
	// 	}
	// }

	// todo: (toInt64 is a "cast" wrapper)
	// funcMap["inc"] = func(i interface{}) int64 { return toInt64(i) + 1 }
	// funcMap["dec"] = func(i interface{}) int64 { return toInt64(i) - 1 }

	return template.New("").Option("missingkey=zero").Funcs(funcMap)
}

func parseToml(tomlFiles, tomlText []string) config {
	result := config{}

	for _, file := range tomlFiles {
		bytes, err := os.ReadFile(file)
		if err != nil {
			glog.Fatalf("err reading TOML file: %s", file)
		}
		// "prepend"
		tomlText = append([]string{string(bytes)}, tomlText...)
	}

	for _, text := range tomlText {
		// var parsed map[string]interface{}
		var parsed config
		err := toml.Unmarshal([]byte(text), &parsed)
		if err != nil {
			panic(err)
		}
		if err := mergo.Merge(&result, parsed); err != nil {
			panic(err)
		}
	}

	return result
}
