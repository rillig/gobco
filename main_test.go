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

func declarations(i int) {
	_ = i > 0
	pos := i > 0
	_ = pos
}

func switchStmt(s string) {
	switch s {
	case "one":
	}

	switch {
	case s == "one":
	case s < "a":
	}
}

func booleanOperators(a, b bool) {
	both := a && b
	either := a || b
	_, _ = both, either
}

func forStmt(s string) bool {
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
}

func ifStmt(a int, b string) bool {
	if a > 0 && b == "positive" {
		return true
	}
	if len(b) > 5 {
		return len(b) > 10
	}
	return false
}

func callExpr(a bool, b string) bool {
	if len(b) > 0 {
		return callExpr(len(b) % 2 == 0, b[1:])
	}
	return false
}`, "\n")

	expected := strings.TrimLeft(`
package main

import "fmt"

func declarations(i int) {
	_ = gobcoCover(0, i > 0)
	pos := gobcoCover(1, i > 0)
	_ = pos
}

func switchStmt(s string) {
	switch s {
	case "one":
	}

	switch {
	case gobcoCover(2, s == "one"):
	case gobcoCover(3, s < "a"):
	}
}

func booleanOperators(a, b bool) {
	both := gobcoCover(4, a) && gobcoCover(5, b)
	either := gobcoCover(6, a) || gobcoCover(7, b)
	_, _ = both, either
}

func forStmt(s string) bool {
	for i, r := range b {
		if gobcoCover(8, r == a) {
			return true
		}
	}

	for i := 0; gobcoCover(9, i < len(b)); i++ {
		if gobcoCover(10, b[i] == a) {
			return true
		}
	}

	return false
}

func ifStmt(a int, b string) bool {
	if gobcoCover(11, gobcoCover(12, a > 0) && gobcoCover(13, b == "positive")) {
		return true
	}
	if gobcoCover(14, len(b) > 5) {
		return gobcoCover(15, len(b) > 10)
	}
	return false
}

func callExpr(a bool, b string) bool {
	if gobcoCover(16, len(b) > 0) {
		return callExpr(gobcoCover(17, len(b)%2 == 0), b[1:])
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
