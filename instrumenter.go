package main

import (
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

// instrument modifies the code of the Go package in dir by adding counters for
// code coverage. If base is given, only that file is instrumented.
func (i *instrumenter) instrument(srcDir, base, dstDir string) {
	i.fset = token.NewFileSet()

	isRelevant := func(info os.FileInfo) bool {
		return base == "" || info.Name() == base
	}

	pkgs, err := parser.ParseDir(i.fset, srcDir, isRelevant, parser.ParseComments)
	if err != nil {
		panic(err)
	}

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
	if err != nil {
		panic(err)
	}
	i.text = string(fileBytes)

	shouldBuild := func() bool {
		ctx := build.Context{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
		ok, err := ctx.MatchFile(path.Dir(filename), path.Base(filename))
		if err != nil {
			panic(err)
		}
		return ok
	}

	isTest := strings.HasSuffix(filename, "_test.go")
	if (i.coverTest || !isTest) && shouldBuild() {
		ast.Inspect(astFile, i.visit)
	}
	if isTest {
		i.instrumentTestMain(astFile)
	}

	var out strings.Builder
	err = printer.Fprint(&out, i.fset, astFile)
	if err != nil {
		panic(err)
	}
	i.writeFile(filepath.Join(dstDir, filepath.Base(filename)), out.String())
}

// visit wraps the nodes of an AST to be instrumented by the coverage.
//
// Each expression that is syntactically a boolean expression is instrumented
// by replacing it with a function call like gobcoCover(id, expr), where id
// is an auto-generated ID and expr is the condition to be instrumented.
//
// The nodes are instrumented in preorder, as in that mode, the location
// information of the tokens is available, for looking up the text of the
// expressions that are instrumented. The instrumentation is done in-place,
// which means that descending further into the tree may meet some expression
// that is part of the instrumentation code. To prevent endless recursion,
// function calls to gobcoCover are not instrumented further.
//
// XXX: Intuitively, the binary expression 'i > 0' should be instrumented in
// 'case *ast.BinaryExpr'. This isn't done, to prevent endless recursion when
// replacing:
//  i > 0
//  gobcoCover(0, i > 0)
//  gobcoCover(0, gobcoCover(1, i > 0))
//  gobcoCover(0, gobcoCover(1, gobcoCover(2, i > 0)))
func (i *instrumenter) visit(n ast.Node) bool {

	// For the list of possible nodes, see [ast.Walk].
	// TODO: Sort the nodes like in ast.Walk.
	// TODO: Check that all subexpressions are covered by the switch.

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

func (i *instrumenter) visitSwitch(n *ast.SwitchStmt) {
	tag := n.Tag
	body := n.Body.List

	// A switch statement without an expression compares each expression
	// from its case clauses with true. The initialization statement is
	// not modified.
	if tag == nil {
		for _, body := range n.Body.List {
			i.visitExprs(body.(*ast.CaseClause).List)
		}
		return
	}

	// In a switch statement with an expression, the expression is
	// evaluated once and is then compared to each expression from the
	// case clauses. But first, the initialization statement needs to be
	// executed.
	//
	// In the instrumented switch statement, the tag expression always has
	// boolean type, and the expressions in the case clauses are instrumented
	// to calls to 'gobcoCover(id, tag == expr)'.
	var varname *ast.Ident

	if n.Init == nil {
		// Convert 'switch cond {}' to 'switch gobco0 := cond; {}'.
		n.Tag = nil

		varname = i.nextVarname()
		n.Init = &ast.AssignStmt{
			Lhs: []ast.Expr{varname},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{tag}}
	} else {
		varname = i.nextVarname()

		// The initialization statements are executed in a new scope, so
		// convert the 'switch' statement to an empty one, just to have this
		// scope. The same scope is used for storing the tag expression in
		// a variable, as the names don't overlap.
		*n = ast.SwitchStmt{Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.CaseClause{
				List: []ast.Expr{ast.NewIdent("true")},
				Body: []ast.Stmt{
					n.Init,
					&ast.AssignStmt{
						Lhs: []ast.Expr{varname},
						Tok: token.DEFINE,
						Rhs: []ast.Expr{n.Tag},
					},
					&ast.SwitchStmt{
						Body: &ast.BlockStmt{List: body},
					},
				},
			},
		}}}
	}

	// Convert each expression from the 'case' clauses to an expression of
	// the form 'gobcoCover(id, tag == expr)'.
	for _, clause := range body {
		clause := clause.(*ast.CaseClause)
		for j, expr := range clause.List {
			eq := ast.BinaryExpr{X: varname, Op: token.EQL, Y: expr}
			eqlStr := i.strEql(tag, expr)
			clause.List[j] = i.wrapText(&eq, expr, eqlStr)
		}
	}
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

// wrap returns the given expression surrounded by a function call to
// gobcoCover and remembers the location and text of the expression,
// for later generating the table of coverage points.
func (i *instrumenter) wrap(cond ast.Expr) ast.Expr {
	if _, ok := cond.(*ast.UnaryExpr); ok {
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

	cover := ast.NewIdent("gobcoCover")
	cover.NamePos = orig.Pos()
	return &ast.CallExpr{
		Fun: cover,
		Args: []ast.Expr{
			&ast.BasicLit{Kind: token.INT, Value: fmt.Sprint(idx)},
			cond}}
}

// addCond remembers a condition and returns its internal ID, which is then
// used as an argument to the gobcoCover function.
func (i *instrumenter) addCond(start, code string) int {
	i.conds = append(i.conds, cond{start, code})
	return len(i.conds) - 1
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
	fixPkgname := func(str string) string {
		return strings.Replace(str, "package main\n", "package "+pkgname+"\n", 1)
	}
	i.writeFile(filepath.Join(tmpDir, "gobco_fixed.go"), fixPkgname(gobco_fixed_go))
	i.writeGobcoGo(filepath.Join(tmpDir, "gobco_variable.go"), pkgname)

	i.writeFile(filepath.Join(tmpDir, "gobco_fixed_test.go"), fixPkgname(gobco_fixed_test_go))
	if !i.hasTestMain {
		i.writeFile(filepath.Join(tmpDir, "gobco_variable_test.go"), fixPkgname(gobco_variable_test_go))
	}
}

func (i *instrumenter) writeGobcoGo(filename, pkgname string) {
	var sb strings.Builder

	sb.WriteString("package " + pkgname + "\n")
	sb.WriteString("\n")
	sb.WriteString("var gobcoOpts = gobcoOptions{\n")
	sb.WriteString(fmt.Sprintf("\timmediately: %v,\n", i.immediately))
	sb.WriteString(fmt.Sprintf("\tlistAll:     %v,\n", i.listAll))
	sb.WriteString("}\n")
	sb.WriteString("\n")
	sb.WriteString("var gobcoCounts = gobcoStats{\n")
	sb.WriteString("\tconds: []gobcoCond{\n")
	for _, cond := range i.conds {
		sb.WriteString(fmt.Sprintf("\t\t{%q, %q, 0, 0},\n", cond.start, cond.code))
	}
	sb.WriteString("\t},\n")
	sb.WriteString("}\n")

	i.writeFile(filename, sb.String())
}

func (i *instrumenter) writeFile(filename string, content string) {
	err := ioutil.WriteFile(filename, []byte(content), 0666)
	if err != nil {
		panic(err)
	}
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
