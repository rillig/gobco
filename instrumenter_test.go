package main

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"gopkg.in/check.v1"
	"strings"
	"testing"
)

type Suite struct{}

func Test(t *testing.T) {
	check.Suite(new(Suite))
	check.TestingT(t)
}

func (s *Suite) Test_instrumenter_visit(c *check.C) {

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
	c.Check(err, check.IsNil)

	i := instrumenter{fset, code, nil, options{}}
	ast.Inspect(f, i.visit)

	var out strings.Builder
	err = printer.Fprint(&out, fset, f)
	c.Check(err, check.IsNil)

	c.Check(out.String(), check.Equals, expected)

	c.Check(i.conds, check.DeepEquals, []cond{
		{start: "test.go:6:6", code: "i > 0"},
		{start: "test.go:7:9", code: "i > 0"},
		{start: "test.go:17:7", code: "s == \"one\""},
		{start: "test.go:18:7", code: "s < \"a\""},
		{start: "test.go:23:10", code: "a"},
		{start: "test.go:23:15", code: "b"},
		{start: "test.go:24:12", code: "a"},
		{start: "test.go:24:17", code: "b"},
		{start: "test.go:30:6", code: "r == a"},
		{start: "test.go:35:14", code: "i < len(b)"},
		{start: "test.go:36:6", code: "b[i] == a"},
		{start: "test.go:45:5", code: "a > 0 && b == \"positive\""},
		{start: "test.go:45:5", code: "a > 0"},
		{start: "test.go:45:14", code: "b == \"positive\""},
		{start: "test.go:48:5", code: "len(b) > 5"},
		{start: "test.go:49:10", code: "len(b) > 10"},
		{start: "test.go:55:5", code: "len(b) > 0"},
		{start: "test.go:56:19", code: "len(b) % 2 == 0"}})
}
