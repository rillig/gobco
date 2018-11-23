package main

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

func Test_instrumenter_visit(t *testing.T) {

	code := strings.TrimLeft(`
package main

import "fmt"

func CoverTest(a int, b string) bool {
	if a > 0 && b == "positive" {
		return true
	}
	for i, r := range b {
		if r == a {
			return true
		}
	}
	for i := 0; i < len(b); i++ {
		if b[i] == a {
			return true
		}
	}
	return false
}`, "\n")

	expected := strings.TrimLeft(`
package main

import "fmt"

func CoverTest(a int, b string) bool {
	if gobcoCover(a > 0 && b == "positive", 0) {
		return true
	}
	for i, r := range b {
		if gobcoCover(r == a, 1) {
			return true
		}
	}
	for i := 0; gobcoCover(i < len(b), 2); i++ {
		if gobcoCover(b[i] == a, 3) {
			return true
		}
	}
	return false
}
`, "\n")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatal(err)
	}

	i := instrumenter{fset, code, nil}
	ast.Inspect(f, i.visit)

	var out strings.Builder
	err = printer.Fprint(&out, fset, f)
	if err != nil {
		t.Error(err)
	}

	if out.String() != expected {
		t.Errorf("\nexp: %q\nact: %q", expected, out.String())
	}
}
