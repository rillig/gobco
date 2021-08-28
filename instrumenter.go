package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// cond is a condition that appears somewhere in the source code.
type cond struct {
	// TODO: Maybe split this field into three.
	start string // human-readable position in the file, e.g. "main.go:17:13"
	code  string // the source code of the condition
}

// instrumenter rewrites the code of a go package (in a temporary directory),
// and changes the source files by instrumenting them.
type instrumenter struct {
	fset        *token.FileSet
	text        string // during instrumentFile(), the text of the current file
	conds       []cond // the collected conditions from all files from fset
	exprs       int    // counter to generate unique variables for switch expressions
	listAll     bool   // also list conditions that are covered
	immediately bool   // persist counts after each increment
	coverTest   bool   // also cover the test code

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
	if cond, ok := cond.(*ast.UnaryExpr); ok && cond.Op == token.NOT {
		return cond
	}
	return i.wrapText(cond, cond, i.str(cond))
}

// wrap returns the expression cond surrounded by a function call to
// gobcoCover and remembers the location and text of the expression,
// for later generating the table of coverage points.
//
// The expression orig is the one from the actual code, and in case of
// switch statements may differ from cond, which is the expression to
// wrap.
func (i *instrumenter) wrapText(cond, orig ast.Expr, code string) ast.Expr {
	origStart := i.fset.Position(orig.Pos())
	if orig.Pos().IsValid() && !strings.HasSuffix(origStart.Filename, ".go") {
		return cond // don't wrap generated code, such as yacc parsers
	}

	start := i.fset.Position(orig.Pos())
	idx := i.addCond(start.String(), code)

	return &ast.CallExpr{
		Fun: ast.NewIdent("gobcoCover"),
		Args: []ast.Expr{
			&ast.BasicLit{Kind: token.INT, Value: fmt.Sprint(idx)},
			cond}}
}

func (i *instrumenter) visitSwitch(n *ast.SwitchStmt) {
	tag := n.Tag
	if tag == nil {
		for _, body := range n.Body.List {
			i.visitExprs(body.(*ast.CaseClause).List)
		}
		return
	}

	var varname *ast.Ident
	if n.Init == nil {
		n.Tag = nil

		varname = i.nextVarname()
		n.Init = &ast.AssignStmt{
			Lhs: []ast.Expr{varname},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{tag}}
	} else {
		init, ok := n.Init.(*ast.AssignStmt)
		if !ok || len(init.Lhs) != len(init.Rhs) {
			return
		}

		prevTag := n.Tag
		varname = i.nextVarname()
		n.Tag = varname

		init.Lhs = append(init.Lhs, varname)
		init.Tok = token.DEFINE
		init.Rhs = append(init.Rhs, prevTag)
	}

	for _, clause := range n.Body.List {
		clause := clause.(*ast.CaseClause)
		for j, expr := range clause.List {
			eq := ast.BinaryExpr{X: varname, Op: token.EQL, Y: expr}
			eqlStr := i.strEql(tag, expr)
			clause.List[j] = i.wrapText(&eq, expr, eqlStr)
		}
	}
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
		// See also instrumenter.visitExprs.

	case *ast.UnaryExpr:
		if n.Op == token.NOT {
			n.X = i.wrap(n.X)
		}

	case *ast.CallExpr:
		if ident, ok := n.Fun.(*ast.Ident); !ok || ident.Name != "gobcoCover" {
			i.visitExprs(n.Args)
		}

	case *ast.ReturnStmt:
		i.visitExprs(n.Results)

	case *ast.AssignStmt:
		i.visitExprs(n.Lhs)
		i.visitExprs(n.Rhs)

	case *ast.SwitchStmt:
		i.visitSwitch(n)

	case *ast.TypeSwitchStmt:
		// TODO

	case *ast.SelectStmt:
		// Note: select statements are already handled by go cover.
	}

	return true
}

// strEql returns the string representation of (lhs == rhs).
func (i *instrumenter) strEql(lhs ast.Expr, rhs ast.Expr) string {

	needsParentheses := func(expr ast.Expr) bool {
		switch expr := expr.(type) {
		case *ast.Ident,
			*ast.SelectorExpr,
			*ast.BasicLit,
			*ast.IndexExpr,
			*ast.CompositeLit,
			*ast.UnaryExpr,
			*ast.CallExpr,
			*ast.ParenExpr:
			return false
		case *ast.BinaryExpr:
			return expr.Op.Precedence() <= token.EQL.Precedence()
		}
		return true
	}

	lp := needsParentheses(lhs)
	rp := needsParentheses(rhs)

	condStr := func(cond bool, yes string) string {
		if cond {
			return yes
		}
		return ""
	}

	return fmt.Sprintf("%s%s%s == %s%s%s",
		condStr(lp, "("), i.str(lhs), condStr(lp, ")"),
		condStr(rp, "("), i.str(rhs), condStr(rp, ")"))
}

// visitExprs wraps the given expression list for coverage.
func (i *instrumenter) visitExprs(exprs []ast.Expr) {
	for idx := range exprs {
		i.visitExpr(&exprs[idx])
	}
}

// visitExpr wraps comparison expressions in a call to gobcoCover, thereby
// counting how often these expressions are evaluated.
func (i *instrumenter) visitExpr(exprPtr *ast.Expr) {
	switch expr := (*exprPtr).(type) {
	// FIXME: What about the other types of expression?
	case *ast.BinaryExpr:
		if expr.Op.Precedence() == token.EQL.Precedence() {
			*exprPtr = i.wrap(expr)
		}
	case *ast.IndexExpr:
		i.visitExpr(&expr.Index)
	}
}

// instrument modifies the code of the Go package in dir by adding counters for
// code coverage. If base is given, only that file is instrumented.
func (i *instrumenter) instrument(srcDir, base, dstDir string) {
	i.fset = token.NewFileSet()

	isRelevant := func(info os.FileInfo) bool {
		return base == "" || info.Name() == base
	}

	pkgs, err := parser.ParseDir(i.fset, srcDir, isRelevant, parser.ParseComments)
	i.check(err)

	for pkgname, pkg := range pkgs {
		i.instrumentPackage(pkgname, pkg, dstDir)
	}
}

func (i *instrumenter) instrumentPackage(pkgname string, pkg *ast.Package, dstDir string) {

	// Sorting the filenames is only for convenience during debugging.
	// It doesn't have any effect on the generated code.
	var filenames []string
	for filename := range pkg.Files {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		i.instrumentFile(filename, pkg.Files[filename], dstDir)
	}

	i.writeGobcoFiles(dstDir, pkgname)
}

func (i *instrumenter) instrumentFile(filename string, astFile *ast.File, dstDir string) {
	fileBytes, err := ioutil.ReadFile(filename)
	i.check(err)
	i.text = string(fileBytes)

	shouldBuild := func() bool {
		ctx := build.Context{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
		ok, err := ctx.MatchFile(path.Dir(filename), path.Base(filename))
		i.check(err)
		return ok
	}

	isTest := strings.HasSuffix(filename, "_test.go")
	if (i.coverTest || !isTest) && shouldBuild() {
		ast.Inspect(astFile, i.visit)
	}
	if isTest {
		i.instrumentTestMain(astFile)
	}

	var out bytes.Buffer
	i.check(printer.Fprint(&out, i.fset, astFile))
	i.writeFile(filepath.Join(dstDir, filepath.Base(filename)), out.Bytes())
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

func (i *instrumenter) str(expr ast.Expr) string {
	start := i.fset.Position(expr.Pos())
	end := i.fset.Position(expr.End())
	return i.text[start.Offset:end.Offset]
}

func (i *instrumenter) nextVarname() *ast.Ident {
	varname := fmt.Sprintf("gobco%d", i.exprs)
	i.exprs++
	return ast.NewIdent(varname)
}

func (*instrumenter) check(e error) {
	if e != nil {
		panic(e)
	}
}
