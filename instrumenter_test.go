package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"strings"
	"testing"
)

// Test_instrumenter ensures that a piece of code is properly instrumented by
// sprinkling calls to gobcoCover around each interesting expression.
func Test_instrumenter(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"AssignStmt"},
		{"BinaryExpr"},
		{"BlockStmt"},
		{"CallExpr"},
		{"CaseClause"},
		{"CommClause"},
		{"CompositeLit"},
		{"DeclStmt"},
		{"DeferStmt"},
		{"CompositeLit"},
		{"CompositeLit"},
		{"CompositeLit"},
		{"Ellipsis"},
		{"ExprStmt"},
		{"ForStmt"},
		{"FuncDecl"},
		{"FuncLit"},
		{"GenDecl"},
		{"GoStmt"},
		{"IfStmt"},
		{"IncDecStmt"},
		{"IndexExpr"},
		{"KeyValueExpr"},
		{"LabeledStmt"},
		{"ListExpr"},
		{"ParenExpr"},
		{"RangeStmt"},
		{"ReturnStmt"},
		{"SelectorExpr"},
		{"SelectStmt"},
		{"SendStmt"},
		{"SliceExpr"},
		{"StarExpr"},
		{"SwitchStmt"},
		{"TypeAssertExpr"},
		{"TypeSwitchStmt"},
		{"UnaryExpr"},
		{"ValueSpec"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base := "testdata/instrumenter/" + test.name
			src := load(base + ".go")
			expected := load(base + ".gobco")

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
			if len(i.conds) > 0 {
				sb.WriteString("\n")
			}
			for _, cond := range i.conds {
				location := strings.TrimPrefix(cond.start, "test.go")
				sb.WriteString(fmt.Sprintf("// %s: %q\n", location, cond.code))
			}

			actual := sb.String()
			if actual != expected {
				err := ioutil.WriteFile(base+".gobco", []byte(actual), 0o666)
				if err != nil {
					t.Fatal(err)
				}
				t.Errorf("expected:\n%s\nactual:\n%s\n", expected, actual)
			}
		})
	}
}

func load(name string) string {
	text, err := ioutil.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(text)
}
