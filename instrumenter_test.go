package main

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
)

// Test_instrumenter ensures that a piece of code is properly instrumented by
// sprinkling calls to gobcoCover around each interesting expression.
func Test_instrumenter(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		instrumented string
		conds        []cond
	}{
		// TODO: AssignStmt
		// TODO: BinaryExpr
		// TODO: BlockStmt
		// TODO: CallExpr
		// TODO: CaseClause
		// TODO: CommClause
		// TODO: CompositeLit
		// TODO: DeclStmt
		// TODO: DeferStmt
		// TODO: Ellipsis
		// TODO: ExprStmt
		// TODO: ForStmt
		// TODO: FuncDecl
		// TODO: FuncLit
		// TODO: GenDecl
		// TODO: GoStmt
		// TODO: IfStmt
		// TODO: IncDecStmt
		// TODO: IndexExpr
		// TODO: KeyValueExpr
		// TODO: LabeledStmt
		// TODO: ListExpr
		// TODO: ParenExpr
		// TODO: RangeStmt
		// TODO: ReturnStmt
		// TODO: SelectorExpr
		// TODO: SelectStmt
		// TODO: SendStmt
		// TODO: SliceExpr
		// TODO: StarExpr

		{
			name: "switch",
			conds: []cond{
				{"test.go:10:7", "expr == 5"},
				// TODO: {"test.go:12:7", "cond"},
				{"test.go:23:7", "s == \"one\""},
				{"test.go:24:3", "s == \"two\""},
				{"test.go:25:3", "s == \"three\""},
				{"test.go:32:7", "s + \"suffix\" == \"one\""},
				{"test.go:33:3", "s + \"suffix\" == \"two\""},
				{"test.go:34:3", "s + \"suffix\" == \"\" + s"},
				{"test.go:44:7", "s + \"suffix\" == \"prefix.a.suffix\""},
				{"test.go:48:7", "s + \"suffix\" == \"prefix.a.suffix\""},
				{"test.go:55:7", "s == \"one\""},
				{"test.go:63:7", "cond == true"},
				{"test.go:74:7", "expr == 5"},
			},
		},

		// TODO: TypeAssertExpr
		// TODO: TypeSwitchStmt
		// TODO: UnaryExpr
		// TODO: ValueSpec

		{
			// Comparison expressions have return type boolean and are
			// therefore instrumented.
			"comparisons",
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
			[]cond{
				{start: "test.go:4:6", code: "i > 0"},
				{start: "test.go:5:9", code: "i > 0"},
			},
		},

		// Binary boolean operators are clearly identifiable and are
		// therefore wrapped.
		//
		// Copying boolean variables is not wrapped though since there
		// is no code branch involved.
		//
		// Also, gobco only looks at the parse tree without any type resolution.
		// Therefore it cannot decide whether a variable is boolean or not.
		{
			"boolean binary expr",
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
			[]cond{
				{start: "test.go:4:10", code: "a"},
				{start: "test.go:4:15", code: "b"},
				{start: "test.go:5:12", code: "a"},
				{start: "test.go:5:17", code: "b"},
			},
		},

		// To avoid double negation, only the innermost expression of a
		// negation is instrumented.
		//
		// Note: The operands of the && are in the "wrong" order because of the
		// order in which the AST nodes are visited. First the two direct
		// operands of the && expression, then each operand further down.
		{
			"negation",
			`
				package main

				func negation(a, b, c bool) {
					_ = !!!a
					_ = !b && c
				}
			`,
			`
				package main

				func negation(a, b, c bool) {
					_ = !!!gobcoCover(0, a)
					_ = !gobcoCover(2, b) && gobcoCover(1, c)
				}
			`,
			[]cond{
				{start: "test.go:4:9", code: "a"},
				{start: "test.go:5:12", code: "c"},
				{start: "test.go:5:7", code: "b"},
			},
		},

		// In a RangeStmt there is no obvious condition, therefore nothing
		// is wrapped. Maybe it would be possible to distinguish empty and
		// nonempty, but that would require a temporary variable, to avoid
		// computing the range expression twice.
		//
		// Code that wants to have this check in a specific place can just
		// manually add a statement before the range statement:
		//  _ = len(b) > 0
		{
			"for range",
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
			[]cond{
				{start: "test.go:5:6", code: "r == a"},
			},
		},

		// The condition of a ForStmt is always a boolean expression and is
		// therefore instrumented, no matter if it is a simple or a complex
		// expression.
		{
			"for cond",
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
			[]cond{
				{start: "test.go:4:14", code: "i < len(b)"},
				{start: "test.go:5:6", code: "b[i] == a"},
			},
		},

		// A ForStmt without condition can only have one outcome.
		// Therefore no branch coverage is necessary.
		{
			"forever",
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
			nil,
		},

		// The condition from an if statement is always a boolean expression.
		// And even if the condition is a simple variable, it is wrapped.
		// This is different from arguments to function calls, where simple
		// variables are not wrapped.
		{
			"if cond",
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
			[]cond{
				{start: "test.go:4:5", code: "a > 0 && b == \"positive\""},
				{start: "test.go:4:5", code: "a > 0"},
				{start: "test.go:4:14", code: "b == \"positive\""},
				{start: "test.go:7:5", code: "len(b) > 5"},
				{start: "test.go:8:10", code: "len(b) > 10"},
				{start: "test.go:10:5", code: "c"},
			},
		},

		// Those arguments to function calls that can be clearly identified
		// as boolean expressions are wrapped. Direct boolean arguments are
		// not wrapped since, as of July 2019, gobco does not use type
		// resolution.
		{
			"function call",
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
			[]cond{
				{start: "test.go:4:5", code: "len(b) > 0"},
				{start: "test.go:5:19", code: "len(b) % 2 == 0"},
			},
		},

		// A CallExpr without identifier is also covered. The test for an
		// identifier is only needed to filter out the calls to gobcoCover,
		// which may have been inserted by a previous instrumentation.
		{
			"function call without identifier",
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
			[]cond{
				{start: "test.go:4:20", code: "1 != 2"},
			},
		},

		// Before gobco-0.10.2, conditionals on the left-hand side of an assignment
		// statement were not instrumented. It's probably an edge case but may
		// nevertheless occur in practice.
		{
			"assignment",
			`
				package main

				func assignLeft(i int) {
					m := make(map[bool]string)
					m[i > 0] = "yes"
				}
			`,
			`
				package main

				func assignLeft(i int) {
					m := make(map[bool]string)
					m[gobcoCover(0, i > 0)] = "yes"
				}
			`,
			[]cond{
				{start: "test.go:5:4", code: "i > 0"},
			},
		},

		// Select statements are already handled by the normal go coverage.
		// Therefore gobco doesn't instrument them.
		{
			"select",
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
			nil,
		},

		{
			"unary operator",
			`
				package main

				func callExpr(i int) {
					if -i > 0 {
					}
				}
			`,
			`
				package main

				func callExpr(i int) {
					if gobcoCover(0, -i > 0) {
					}
				}
			`,
			[]cond{
				{start: "test.go:4:5", code: "-i > 0"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			src := load(test.src, test.name + ".go")
			expectedInstrumented := load(test.instrumented, test.name + ".gobco")

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}

			i := instrumenter{fset, src, nil, 0, false, false, false, false}
			ast.Inspect(f, i.visit)

			var sb strings.Builder
			err = printer.Fprint(&sb, fset, f)
			if err != nil {
				t.Fatal(err)
			}

			instrumented := sb.String()
			if instrumented != expectedInstrumented {
				t.Errorf("expected:\n%s\nactual:\n%s\n", expectedInstrumented, instrumented)
			}

			if !reflect.DeepEqual(i.conds, test.conds) {
				t.Errorf("\nexpected: %#v\nactual:   %#v\n", test.conds, i.conds)
			}
		})
	}
}

func load(s string, name string) string {
	if s == "" {
		text, err := ioutil.ReadFile("testdata/instrumenter/"+name)
		if err != nil {
			panic(err)
		}
		return string(text)
	}

	s = strings.TrimPrefix(s, "\n")
	s = strings.TrimSuffix(s, "\n")

	lines := strings.SplitAfter(s, "\n")
	if strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	indent := lines[0]
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			indent = longestCommonPrefix(indent, indentation(line))
		}
	}

	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString(strings.TrimPrefix(line, indent))
	}
	return sb.String()
}

func indentation(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
}

func longestCommonPrefix(a, b string) string {
	for i := range a {
		if i >= len(b) || a[i] != b[i] {
			return a[:i]
		}
	}
	return a
}
