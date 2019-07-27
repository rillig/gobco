package main

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"gopkg.in/check.v1"
	"strings"
)

func (s *Suite) Test_instrumenter_visit(c *check.C) {

	test := func(before, after string, conds ...cond) {
		normalize := func(s string) string {
			return strings.TrimLeft(strings.Replace(s, "\n\t\t", "\n", -1), "\n")
		}

		trimmedBefore := normalize(before)
		trimmedAfter := normalize(after)

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "test.go", trimmedBefore, 0)
		c.Check(err, check.IsNil)

		i := instrumenter{fset, trimmedBefore, nil, false, false}
		ast.Inspect(f, i.visit)

		var out strings.Builder
		err = printer.Fprint(&out, fset, f)
		c.Check(err, check.IsNil)

		c.Check(out.String(), check.Equals, trimmedAfter)

		c.Check(i.conds, check.DeepEquals, conds)
	}

	// Expressions of type bool are wrapped.
	test(
		`
		package main

		func declarations(i int) {
			_ = i > 0
			pos := i > 0
			_ = pos
		}
		`,
		`
		package main

		func declarations(i int) {
			_ = gobcoCover(0, i > 0)
			pos := gobcoCover(1, i > 0)
			_ = pos
		}
		`,
		cond{start: "test.go:4:6", code: "i > 0"},
		cond{start: "test.go:5:9", code: "i > 0"})

	// In a switch statement with an expression, the type of the
	// expression can be any comparable type, and in most cases is
	// not bool. Therefore it is not wrapped.
	test(
		`
		package main

		func switchStmt(s string) {
			switch s {
			case "one":
			}
		}
		`,
		`
		package main

		func switchStmt(s string) {
			switch s {
			case "one":
			}
		}
		`,
		nil...)

	// In a switch statement without explicit expression, each case
	// expression must be of boolean type and can therefore be wrapped.
	test(
		`
		package main

		func switchStmt(s string) {
			switch {
			case s == "one":
			case s < "a":
			}
		}
		`,
		`
		package main

		func switchStmt(s string) {
			switch {
			case gobcoCover(0, s == "one"):
			case gobcoCover(1, s < "a"):
			}
		}
		`,
		cond{start: "test.go:5:7", code: "s == \"one\""},
		cond{start: "test.go:6:7", code: "s < \"a\""})

	// Binary boolean operators are clearly identifiable and are
	// therefore wrapped.
	//
	// Copying boolean variables is not wrapped though since there
	// is no code branch involved.
	test(
		`
		package main

		func booleanOperators(a, b bool) {
			both := a && b
			either := a || b
			_, _ = both, either
		}
		`,
		`
		package main

		func booleanOperators(a, b bool) {
			both := gobcoCover(0, a) && gobcoCover(1, b)
			either := gobcoCover(2, a) || gobcoCover(3, b)
			_, _ = both, either
		}
		`,
		cond{start: "test.go:4:10", code: "a"},
		cond{start: "test.go:4:15", code: "b"},
		cond{start: "test.go:5:12", code: "a"},
		cond{start: "test.go:5:17", code: "b"})

	// A negation is not wrapped. Why not? It might make sense.
	test(
		`
		package main

		func negation(a bool) {
			_ = !!!a
		}
		`,
		`
		package main

		func negation(a bool) {
			_ = !!!a
		}
		`,
		nil...)

	// In a RangeStmt there is no obvious condition, therefore nothing
	// is wrapped. Maybe it would be possible to distinguish empty and
	// nonempty, but that would require a temporary variable, to avoid
	// computing the range expression twice.
	test(
		`
		package main

		func rangeStmt(s string) bool {
			for i, r := range b {
				if r == a {
					return true
				}
			}
			return false
		}
		`,
		`
		package main

		func rangeStmt(s string) bool {
			for i, r := range b {
				if gobcoCover(0, r == a) {
					return true
				}
			}
			return false
		}
		`,
		cond{start: "test.go:5:6", code: "r == a"})

	// The condition of a ForStmt is always a boolean expression and is
	// therefore wrapped, no matter if it is a simple or a complex
	// expression.
	test(
		`
		package main

		func forStmt(s string) bool {
			for i := 0; i < len(b); i++ {
				if b[i] == a {
					return true
				}
			}
			return false
		}
		`,
		`
		package main

		func forStmt(s string) bool {
			for i := 0; gobcoCover(0, i < len(b)); i++ {
				if gobcoCover(1, b[i] == a) {
					return true
				}
			}
			return false
		}
		`,
		cond{start: "test.go:4:14", code: "i < len(b)"},
		cond{start: "test.go:5:6", code: "b[i] == a"})

	// A ForStmt without condition can only have one outcome.
	// Therefore no branch coverage is necessary.
	test(
		`
		package main

		func forStmt() {
			for {
				break
			}
		}
		`,
		`
		package main

		func forStmt() {
			for {
				break
			}
		}
		`,
		nil...)

	// The condition from an if statement is always a boolean expression.
	// And even if the condition is a simple variable, it is wrapped.
	// This is different from arguments to function calls, where simple
	// variables are not wrapped.
	test(
		`
		package main

		func ifStmt(a int, b string, c bool) bool {
			if a > 0 && b == "positive" {
				return true
			}
			if len(b) > 5 {
				return len(b) > 10
			}
			if c {
				return true
			}
			return false
		}
		`,
		`
		package main

		func ifStmt(a int, b string, c bool) bool {
			if gobcoCover(0, gobcoCover(1, a > 0) && gobcoCover(2, b == "positive")) {
				return true
			}
			if gobcoCover(3, len(b) > 5) {
				return gobcoCover(4, len(b) > 10)
			}
			if gobcoCover(5, c) {
				return true
			}
			return false
		}
		`,
		cond{start: "test.go:4:5", code: "a > 0 && b == \"positive\""},
		cond{start: "test.go:4:5", code: "a > 0"},
		cond{start: "test.go:4:14", code: "b == \"positive\""},
		cond{start: "test.go:7:5", code: "len(b) > 5"},
		cond{start: "test.go:8:10", code: "len(b) > 10"},
		cond{start: "test.go:10:5", code: "c"})

	// Those arguments to function calls that can be clearly identified
	// as boolean expressions are wrapped. Direct boolean arguments are
	// not wrapped since, as of July 2019, gobco does not use type
	// resolution.
	test(
		`
		package main

		func callExpr(a bool, b string) bool {
			if len(b) > 0 {
				return callExpr(len(b) % 2 == 0, b[1:])
			}
			return false
		}
		`,
		`
		package main

		func callExpr(a bool, b string) bool {
			if gobcoCover(0, len(b) > 0) {
				return callExpr(gobcoCover(1, len(b)%2 == 0), b[1:])
			}
			return false
		}
		`,
		cond{start: "test.go:4:5", code: "len(b) > 0"},
		cond{start: "test.go:5:19", code: "len(b) % 2 == 0"})

	// A CallExpr without identifier is also covered. The test for an
	// identifier is only needed to filter out the calls to gobcoCover,
	// which may have been inserted by a previous transformation.
	test(
		`
		package main

		func callExpr() {
			(func(a bool) {})(1 != 2)
		}
		`,
		`
		package main

		func callExpr() {
			(func(a bool) {})(gobcoCover(0, 1 != 2))
		}
		`,
		cond{start: "test.go:4:20", code: "1 != 2"})

	// Select switches are already handled by the normal go coverage.
	// Therefore gobco doesn't do anything about them.
	test(
		`
		package main

		func callExpr(c chan int) {
			select {
			case c <- 1:
			}
		}
		`,
		`
		package main

		func callExpr(c chan int) {
			select {
			case c <- 1:
			}
		}
		`,
		nil...)
}
