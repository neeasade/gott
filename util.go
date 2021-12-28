package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/imdario/mergo"
	"github.com/pelletier/go-toml/v2"
)

func vlog(format string, args ...interface{}) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

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

func parseToml(tomlFiles, tomlText []string) config {
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
