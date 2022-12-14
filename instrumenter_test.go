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

			goBytes, err := ioutil.ReadFile(base + ".go")
			if err != nil {
				panic(err)
			}
			src := string(goBytes)

			gobcoBytes, err := ioutil.ReadFile(base + ".gobco")
			if err != nil {
				panic(err)
			}
			expected := string(gobcoBytes)

			fset := token.NewFileSet()
			// TODO: Add parser.ParseComments, to get rid of the "sync"
			//  statements in testdata/instrumenter/SwitchStmt.go.
			//  .
			//  As of 2022-11-12, simply using parser.ParseComments moves some
			//  comments around to places they don't belong, for example in
			//  AssignStmt.go. See rewriteFile in go/cmd/fmt/rewrite.go for
			//  instrumenting the code while retaining the comment positions.
			//  That code uses [ast.CommentMap] as well, but is more
			//  complicated.
			f, err := parser.ParseFile(fset, "test.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}

			i := instrumenter{
				false,
				false,
				false,
				fset,
				nil,
				false,
				map[ast.Node]bool{},
				map[ast.Expr]*wrapCondAction{},
				map[ast.Stmt]*ast.Stmt{},
				map[ast.Stmt]func() ast.Stmt{},
				nil,
				src,
				0,
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
