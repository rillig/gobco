package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Test_instrumenter ensures that a piece of code is properly instrumented by
// sprinkling calls to GobcoCover around each interesting expression.
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
		{"Comment"},
		{"CompositeLit"},
		{"DeclStmt"},
		{"DeferStmt"},
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

	testInstrumenter := func(name string, ext string) {
		base := "testdata/instrumenter/" + name

		goBytes, err := os.ReadFile(base + ".go")
		if err != nil {
			t.Fatal(err)
		}
		src := string(goBytes)

		expectedBytes, err := os.ReadFile(base + ext)
		if err != nil {
			expectedBytes = nil
		}
		expected := string(expectedBytes)

		fset := token.NewFileSet()
		mode := parser.ParseComments
		f, err := parser.ParseFile(fset, "test.go", src, mode)
		if err != nil {
			t.Fatal(err)
		}

		i := instrumenter{
			false,
			false,
			false,
			fset,
			0,
			map[ast.Node]bool{},
			map[ast.Expr]*exprSubst{},
			map[ast.Stmt]*ast.Stmt{},
			map[ast.Stmt]ast.Stmt{},
			false,
			nil,
		}
		i.instrumentFileNode(f)

		var sb strings.Builder
		err = printer.Fprint(&sb, fset, f)
		if err != nil {
			t.Fatal(err)
		}
		if len(i.conds) > 0 {
			sb.WriteString("\n")
		}
		for _, cond := range i.conds {
			location := strings.TrimPrefix(cond.pos, "test.go")
			sb.WriteString(fmt.Sprintf("// %s: %q\n",
				location, cond.text))
		}
		actual := sb.String()

		if actual != expected {
			err := os.WriteFile(base+ext, []byte(actual), 0o666)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("expected:\n%s\nactual:\n%s\n", expected, actual)
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testInstrumenter(test.name, ".cond")
		})
	}
}
