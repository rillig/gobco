package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
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

	testInstrumenter := func(name string, branch bool, ext string) {
		dir := "testdata/instrumenter"
		base := dir + "/" + name

		expectedBytes, err := os.ReadFile(base + ext)
		if err != nil {
			expectedBytes = nil
		}
		expected := string(expectedBytes)

		fset := token.NewFileSet()
		mode := parser.ParseComments
		relevant := func(info fs.FileInfo) bool {
			n := info.Name()
			return strings.HasPrefix(n, name) ||
				strings.HasPrefix(n, "zzz")
		}
		pkgs, err := parser.ParseDir(fset, dir, relevant, mode)
		if err != nil {
			t.Fatal(err)
		}

		i := instrumenter{
			branch,
			false,
			false,
			false,
			false,
			fset,
			map[*ast.Package]*types.Package{},
			map[ast.Expr]types.Type{},
			nil,
			0,
			map[ast.Expr]bool{},
			map[ast.Expr]*exprSubst{},
			map[ast.Stmt]*ast.Stmt{},
			map[ast.Stmt]ast.Stmt{},
			false,
			nil,
		}
		fileName := filepath.Clean(base + ".go")
		f := pkgs["instrumenter"].Files[fileName]
		assert(f != nil, fileName)
		i.resolveTypes(pkgs)
		i.typePkg = i.pkg[pkgs["instrumenter"]]
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
			location := strings.TrimPrefix(cond.pos, fileName)
			sb.WriteString(fmt.Sprintf("// %s: %q\n",
				location, cond.text))
		}
		actual := sb.String()

		if actual != expected {
			err := os.WriteFile(base+ext, []byte(actual), 0o666)
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("updated %s", base+ext)
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testInstrumenter(test.name, true, ".branch")
			testInstrumenter(test.name, false, ".cond")
		})
	}
}
