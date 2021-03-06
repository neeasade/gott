package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"strconv"

	"os/exec"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/imdario/mergo"
	"github.com/pelletier/go-toml/v2"
)

func mapToNode(n *Node, value interface{}, path NodePath) {
	nested, is_map := value.(map[string]interface{})
	arrayVal, is_array := value.([]interface{})
	if is_map {
		for key, _ := range nested {
			mapToNode(n, nested[key], append(path, key))
		}
	} else if is_array {
		for index, _ := range arrayVal {
			mapToNode(n, arrayVal[index], append(path, index))
		}
	} else {
		// results[namespace+key] = fmt.Sprintf("%v", value)
		p := NodePath(append(path, value))
		vlog("adding path to root node: %v", p.ToString())
		n.add(p...)
	}
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

		result := string(out)
		if result[len(result)-1] == '\n' {
			result = result[0:]
		}

		return result
	}

	funcMap["sh"] = func(command string, args ...string) string {
		args = append([]string{"-c", command, "--"}, args...)
		out, err := exec.Command("bash", args...).Output()

		exitStatus := true
		if werr, ok := err.(*exec.ExitError); ok {
			if s := werr.Error(); s != "0" {
				exitStatus = false
			}
		}

		// if err != nil {
		// 	glog.Fatal(err)
		// }

		if strings.TrimSpace(string(out)) == "" {
			return strconv.FormatBool(exitStatus)
		} else {
			return string(out)
		}
	}

	funcMap["eq"] = func(a, b interface{}) bool { return a == b }

	if verbose {
		vlog("loaded funcmap functions are:")
		for k, _ := range funcMap {
			vlog(k)
		}
	}

	// todo: (toInt64 is a "cast" wrapper)
	// funcMap["inc"] = func(i interface{}) int64 { return toInt64(i) + 1 }
	// funcMap["dec"] = func(i interface{}) int64 { return toInt64(i) - 1 }

	return template.New("").Option("missingkey=zero").Funcs(funcMap)
}

func parseToml(tomlFiles, tomlText []string) map[string]interface{} {
	result := map[string]interface{}{}

	for _, file := range tomlFiles {
		bytes, err := os.ReadFile(file)
		if err != nil {
			glog.Fatalf("err reading TOML file: %s", file)
		}
		// "prepend"
		tomlText = append([]string{string(bytes)}, tomlText...)
	}

	for _, text := range tomlText {
		var parsed map[string]interface{}
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

