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

	hasTestMain bool
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
		return isDir || info.Name() == filepath.Base(srcName)
	}

	pkgs, err := parser.ParseDir(i.fset, srcDir, isRelevant, parser.ParseComments)
	i.check(err)

	for pkgname, pkg := range pkgs {
		i.instrumentPackage(pkgname, pkg, tmpDir)
	}
}

func (i *instrumenter) instrumentPackage(pkgname string, pkg *ast.Package, tmpDir string) {

	// Sorting the filenames is only for convenience during debugging.
	// It doesn't have any effect on the generated code.
	var filenames []string
	for filename := range pkg.Files {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		i.instrumentFile(filename, pkg.Files[filename], tmpDir)
	}

	i.writeGobcoFiles(tmpDir, pkgname)
}

func (i *instrumenter) instrumentFile(filename string, astFile *ast.File, tmpDir string) {
	fileBytes, err := ioutil.ReadFile(filename)
	i.check(err)
	i.text = string(fileBytes)

	if strings.HasSuffix(filename, "_test.go") {
		i.instrumentTestMain(astFile)
	} else {
		ast.Inspect(astFile, i.visit)
	}

	var out bytes.Buffer
	i.check(printer.Fprint(&out, i.fset, astFile))
	i.writeFile(filepath.Join(tmpDir, filepath.Base(filename)), out.Bytes())
}

func (i *instrumenter) instrumentTestMain(astFile *ast.File) {
	seenOsExit := false

	isOsExit := func(n ast.Node) (bool, *ast.Expr) {
		if call, ok := n.(*ast.CallExpr); ok {
			if fn, ok := call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := fn.X.(*ast.Ident); ok {
					if pkg.Name == "os" && fn.Sel.Name == "Exit" {
						seenOsExit = true
						return true, &call.Args[0]
					}
				}
			}
		}
		return false, nil
	}

	visit := func(n ast.Node) bool {
		if ok, arg := isOsExit(n); ok {
			*arg = &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent("gobcoCounts"),
					Sel: ast.NewIdent("finish")},
				Args: []ast.Expr{*arg}}
		}
		return true
	}

	for _, decl := range astFile.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Recv == nil && decl.Name.Name == "TestMain" {
				i.hasTestMain = true

				ast.Inspect(decl.Body, visit)
				if !seenOsExit {
					panic("gobco: can only handle TestMain with explicit call to os.Exit")
				}
			}
		}
	}
}

func (i *instrumenter) writeGobcoFiles(tmpDir string, pkgname string) {
	fixPkgname := func(str string) []byte {
		return []byte(strings.Replace(str, "package main\n", "package "+pkgname+"\n", 1))
	}
	i.writeFile(filepath.Join(tmpDir, "gobco_fixed.go"), fixPkgname(gobco_fixed_go))
	i.writeGobcoGo(filepath.Join(tmpDir, "gobco_variable.go"), pkgname)

	i.writeFile(filepath.Join(tmpDir, "gobco_fixed_test.go"), fixPkgname(gobco_fixed_test_go))
	if !i.hasTestMain {
		i.writeFile(filepath.Join(tmpDir, "gobco_variable_test.go"), fixPkgname(gobco_variable_test_go))
	}
}

func (i *instrumenter) writeGobcoGo(filename, pkgname string) {
	var sb bytes.Buffer

	_, _ = fmt.Fprintln(&sb, "package "+pkgname)
	_, _ = fmt.Fprintln(&sb)
	_, _ = fmt.Fprintln(&sb, "var gobcoOpts = gobcoOptions{")
	_, _ = fmt.Fprintf(&sb, "\tfirstTime:   %v,\n", i.firstTime)
	_, _ = fmt.Fprintf(&sb, "\timmediately: %v,\n", i.immediately)
	_, _ = fmt.Fprintf(&sb, "\tlistAll:     %v,\n", i.listAll)
	_, _ = fmt.Fprintln(&sb, "}")
	_, _ = fmt.Fprintln(&sb)
	_, _ = fmt.Fprintln(&sb, "var gobcoCounts = gobcoStats{")
	_, _ = fmt.Fprintln(&sb, "\tconds: []gobcoCond{")
	for _, cond := range i.conds {
		_, _ = fmt.Fprintf(&sb, "\t\t{%q, %q, 0, 0},\n", cond.start, cond.code)
	}
	_, _ = fmt.Fprintln(&sb, "\t},")
	_, _ = fmt.Fprintln(&sb, "}")

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
