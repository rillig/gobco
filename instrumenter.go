package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cond is a condition that appears somewhere in the source code.
type cond struct {
	// TODO: Maybe split this field into three.
	start string // human-readable position in the file, e.g. main.go:17:13
	code  string // the source code of the condition
}

// instrumenter rewrites the code of a go package (in a temporary directory),
// and changes the source files by instrumenting them.
type instrumenter struct {
	fset        *token.FileSet
	text        string // during instrumentFile(), the text of the current file
	conds       []cond // the collected conditions from all files from fset
	firstTime   bool   // print condition when it is reached for the first time
	listAll     bool   // also list conditions that are covered
	immediately bool   // persist counts after each increment
}

// addCond remembers a condition and returns its internal ID, which is then
// used as an argument to the gobcoCover function.
func (i *instrumenter) addCond(start, code string) int {
	i.conds = append(i.conds, cond{start, code})
	return len(i.conds) - 1
}

// wrap returns the given expression surrounded by a function call to
// gobcoCover and remembers the location and text of the expression,
// for later generating the table of coverage points.
func (i *instrumenter) wrap(cond ast.Expr) ast.Expr {
	start := i.fset.Position(cond.Pos())

	if !strings.HasSuffix(start.Filename, ".go") {
		return cond // don't wrap generated code, such as yacc parsers
	}

	code := i.text[start.Offset:i.fset.Position(cond.End()).Offset]
	idx := i.addCond(start.String(), code)

	return &ast.CallExpr{
		Fun: ast.NewIdent("gobcoCover"),
		Args: []ast.Expr{
			&ast.BasicLit{Kind: token.INT, Value: fmt.Sprint(idx)},
			cond}}
}

// visit wraps the nodes of an AST to be instrumented by the coverage.
func (i *instrumenter) visit(n ast.Node) bool {
	switch n := n.(type) {

	case *ast.IfStmt:
		n.Cond = i.wrap(n.Cond)

	case *ast.ForStmt:
		if n.Cond != nil {
			n.Cond = i.wrap(n.Cond)
		}

	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			n.X = i.wrap(n.X)
			n.Y = i.wrap(n.Y)
		}

	case *ast.CallExpr:
		if ident, ok := n.Fun.(*ast.Ident); !ok || ident.Name != "gobcoCover" {
			i.visitExprs(n.Args)
		}

	case *ast.ReturnStmt:
		i.visitExprs(n.Results)

	case *ast.AssignStmt:
		i.visitExprs(n.Rhs)

	case *ast.SwitchStmt:
		// This handles only switch {} statements, but not switch expr {}.
		// The latter would be more complicated since the expression would
		// have to be saved to a temporary variable and then be compared
		// to each expression from each branch. It should be doable though.

		if n.Tag == nil {
			for _, body := range n.Body.List {
				i.visitExprs(body.(*ast.CaseClause).List)
			}
		}

	case *ast.SelectStmt:
		// Note: select statements are already handled by go cover.
	}

	return true
}

// visitExprs wraps the given expression list for coverage.
func (i *instrumenter) visitExprs(exprs []ast.Expr) {
	for idx, expr := range exprs {
		switch expr := expr.(type) {
		case *ast.BinaryExpr:
			if expr.Op.Precedence() == token.EQL.Precedence() {
				exprs[idx] = i.wrap(expr)
			}
		}
	}
}

// instrument reads the given file or directory and instruments the code for
// branch coverage. It then writes the instrumented code into tmpName.
func (i *instrumenter) instrument(srcName, tmpName string, isDir bool) {
	i.fset = token.NewFileSet()

	srcDir := srcName
	tmpDir := tmpName
	if !isDir {
		srcDir = filepath.Dir(srcDir)
		tmpDir = filepath.Dir(tmpDir)
	}

	isRelevant := func(info os.FileInfo) bool {
		if isDir {
			return !strings.HasSuffix(info.Name(), "_test.go")
		} else {
			return info.Name() == filepath.Base(srcName)
		}
	}

	pkgs, err := parser.ParseDir(i.fset, srcDir, isRelevant, 0)
	i.check(err)

	for _, pkg := range pkgs {
		var filenames []string
		for filename := range pkg.Files {
			filenames = append(filenames, filename)
		}
		sort.Strings(filenames)

		for _, filename := range filenames {
			i.instrumentFile(filename, pkg.Files[filename], tmpDir)
		}
	}

	for pkgname := range pkgs {
		i.writeGobcoFiles(tmpDir, pkgname)
	}
}

func (i *instrumenter) instrumentFile(filename string, astFile *ast.File, tmpDir string) {
	fileBytes, err := ioutil.ReadFile(filename)
	i.check(err)
	i.text = string(fileBytes)

	ast.Inspect(astFile, i.visit)

	fd, err := os.Create(filepath.Join(tmpDir, filepath.Base(filename)))
	i.check(err)
	i.check(printer.Fprint(fd, i.fset, astFile))
	i.check(fd.Close())
}

func (i *instrumenter) writeGobcoFiles(tmpDir string, pkgname string) {
	i.writeFile(filepath.Join(tmpDir, "gobco_fixed.go"), []byte(gobco_fixed_go))
	i.writeGobcoGo(filepath.Join(tmpDir, "gobco_variable.go"), pkgname)

	i.writeFile(filepath.Join(tmpDir, "gobco_fixed_test.go"), []byte(gobco_fixed_test_go))
	i.writeGobcoTestGo(filepath.Join(tmpDir, "gobco_variable_test.go"), pkgname, false) // TODO
}

func (i *instrumenter) writeGobcoGo(filename, pkgname string) {
	var sb bytes.Buffer

	fmt.Fprintln(&sb, "package "+pkgname)
	fmt.Fprintln(&sb)
	fmt.Fprintln(&sb, "var gobcoOpts = gobcoOptions{")
	fmt.Fprintf(&sb, "\tfirstTime:   %v,\n", i.firstTime)
	fmt.Fprintf(&sb, "\timmediately: %v,\n", i.immediately)
	fmt.Fprintf(&sb, "\tlistAll:     %v,\n", i.listAll)
	fmt.Fprintln(&sb, "}")
	fmt.Fprintln(&sb)
	fmt.Fprintln(&sb, "var gobcoCounts = gobcoStats{")
	fmt.Fprintln(&sb, "\tconds: []gobcoCond{")
	for _, cond := range i.conds {
		fmt.Fprintf(&sb, "\t\t{%q, %q, 0, 0},\n", cond.start, cond.code)
	}
	fmt.Fprintln(&sb, "\t},")
	fmt.Fprintln(&sb, "}")

	i.writeFile(filename, sb.Bytes())
}

func (i *instrumenter) writeGobcoTestGo(filename, pkgname string, hasTestMain bool) {
	var sb bytes.Buffer

	fmt.Fprintf(&sb, "package %s\n", pkgname)
	fmt.Fprintln(&sb)
	fmt.Fprintln(&sb, "import (")
	fmt.Fprintln(&sb, "\t\"os\"")
	fmt.Fprintln(&sb, "\t\"testing\"")
	fmt.Fprintln(&sb, ")")
	fmt.Fprintln(&sb)

	if hasTestMain {
		panic("not yet implemented")
	} else {
		fmt.Fprintln(&sb, "func TestMain(gobcoM *testing.M) {")
		fmt.Fprintln(&sb, "\tm := gobcoTestingM{gobcoM}")
		fmt.Fprintln(&sb, "\tos.Exit(m.Run())")
		fmt.Fprintln(&sb, "}")
	}

	i.writeFile(filename, sb.Bytes())
}

func (i *instrumenter) writeFile(filename string, content []byte) {
	i.check(ioutil.WriteFile(filename, content, 0666))
}

func (*instrumenter) check(e error) {
	if e != nil {
		panic(e)
	}
}
