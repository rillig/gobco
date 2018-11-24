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
	if len(b) > 5 {
		return len(b) > 10
	}
	return false
}`, "\n")

	expected := strings.TrimLeft(`
package main

import "fmt"

func CoverTest(a int, b string) bool {
	if gobcoCover(0, gobcoCover(1, a > 0) && gobcoCover(2, b == "positive")) {
		return true
	}
	for i, r := range b {
		if gobcoCover(3, r == a) {
			return true
		}
	}
	for i := 0; gobcoCover(4, i < len(b)); i++ {
		if gobcoCover(5, b[i] == a) {
			return true
		}
	}
	if gobcoCover(6, len(b) > 5) {
		return gobcoCover(7, len(b) > 10)
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
