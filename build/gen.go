package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"regexp"
)

func generate(srcdir string, pattern string, dstfile string, pkgname string) {
	check := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	infos, err := ioutil.ReadDir(srcdir)
	check(err)

	var sb bytes.Buffer
	fmt.Fprintf(&sb, "package %s\n", pkgname)
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "// generated from %s/%s\n", srcdir, pattern)
	sb.WriteString("\n")
	sb.WriteString("const (\n")

	for _, info := range infos {
		basename := info.Name()

		matched, err := path.Match(pattern, basename)
		check(err)

		if matched {
			content, err := ioutil.ReadFile(filepath.Join(srcdir, basename))
			check(err)

			varname := regexp.MustCompile(`\W`).ReplaceAllString(basename, "_")
			fmt.Fprintf(&sb, "\t%s = %q\n", varname, content)
		}
	}
	sb.WriteString(")\n")

	err = ioutil.WriteFile(dstfile, []byte(sb.String()), 0666)
	check(err)

}

func main() {
	generate("templates", "gobco_*.go", "templates.go", "main")
}
