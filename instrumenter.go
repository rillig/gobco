package main

import (
	_ "embed"
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
	coverTest   bool // also cover the test code
	immediately bool // persist counts after each increment
	listAll     bool // also list conditions that are covered

	fset        *token.FileSet
	conds       []cond // the collected conditions from all files from fset
	hasTestMain bool
	// true to skip only this node but still visit its children,
	// false to skip the complete node;
	// to prevent instrumented code from being instrumented again
	skip map[ast.Node]bool

	text  string // during instrumentFile(), the text of the current file
	exprs int    // counter to generate unique variables for switch expressions
}

// instrument modifies the code of the Go package in srcDir by adding counters
// for code coverage, writing the instrumented code to dstDir.
// If base is given, only that file is instrumented.
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

	// Sorting the filenames is mainly for convenience during debugging.
	// It also affects the names of temporary variables; see nextVarname.
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
// by replacing it with a function call like gobcoCover(id++, cond), where id
// is an auto-generated ID and cond is the condition to be instrumented.
//
// The nodes are instrumented in preorder, as in that mode, the location
// information of the tokens is available, for looking up the text of the
// expressions that are instrumented. The instrumentation is done in-place,
// which means that descending further into the tree may meet some expression
// that is already instrumented. To prevent endless recursion, function calls
// to gobcoCover are not instrumented further.
func (i *instrumenter) visit(n ast.Node) bool {

	// For the list of possible nodes, see [ast.Walk].

	if skip, ok := i.skip[n]; ok {
		return skip
	}

	// XXX: Intuitively, the binary expression 'i > 0' should be instrumented
	// in 'case *ast.BinaryExpr' rather than on one level further out the AST.
	// This isn't done, to prevent endless recursion when replacing:
	//  i > 0
	//  gobcoCover(0, i > 0)
	//  gobcoCover(0, gobcoCover(1, i > 0))
	//  gobcoCover(0, gobcoCover(1, gobcoCover(2, i > 0)))

	// TODO: Try wrapping the standard ast.Inspect with a callback that can
	//  _replace_ the node, rather than only modifying its fields. This may
	//  lead to simpler code for instrumenting switch statements, as well as
	//  being easier to understand, as each expression can be directly
	//  instrumented in instrumenter.visitExpr.
	//  .
	//  On the other hand, this would mean that the instrumenter needs to keep
	//  track that the condition of an if statement always needs to be
	//  instrumented.

	// TODO: Try whether a two-phase approach leads to an implementation that
	//  is easier to understand.
	//  .
	//  Phase 1 would mark all nodes that need to be instrumented, remembering
	//  their string representation.
	//  .
	//  Phase 2 would then replace these nodes with their instrumented
	//  code, in the form 'gobcoCover(id++, expr)'.
	//  .
	//  It may be though that this approach only works for expressions, and
	//  that it becomes more complicated to understand how statements like
	//  'if' and 'switch' are instrumented.

	// Instrument the "entry points", which are those nodes that contain
	// expressions. If the expressions have type boolean, wrap them
	// directly, otherwise scan them for nested boolean expressions.
	//
	// The order of the cases matches the order in ast.Walk.
	switch n := n.(type) {

	case *ast.CallExpr:
		if !i.isGobcoCoverCall(n) {
			// TODO: i.visitExpr(&n.Fun)
			i.visitExprs(n.Args)
		}

	case *ast.UnaryExpr:
		if n.Op == token.NOT {
			n.X = i.wrap(n.X)
		}

	case *ast.BinaryExpr:
		// In '&&' and '||' nodes, it suffices to instrument the
		// terminal conditions, as the outcome of the whole condition
		// depends on the terminal condition that is evaluated last.
		if n.Op == token.LAND || n.Op == token.LOR {
			if lhs, ok := n.X.(*ast.BinaryExpr); ok && lhs.Op == n.Op {
				// Skip this node, it will be visited later.
			} else {
				n.X = i.wrap(n.X)
			}
			n.Y = i.wrap(n.Y)
		}

		// Comparison operators such as '==' are not handled here but
		// in instrumenter.visitExprs because when instrumenting them,
		// the node type would have to be changed to *ast.Call.

	case *ast.ExprStmt:
		i.visitExpr(&n.X)

	case *ast.SendStmt:
		i.visitExpr(&n.Chan)
		i.visitExpr(&n.Value)

	case *ast.IncDecStmt:
		i.visitExpr(&n.X)

	case *ast.AssignStmt:
		i.visitExprs(n.Lhs)
		i.visitExprs(n.Rhs)

	case *ast.GoStmt:
		// TODO: i.visitExpr(&n.Call)

	case *ast.DeferStmt:
		// TODO: i.visitExpr(&n.Call)

	case *ast.ReturnStmt:
		i.visitExprs(n.Results)

	case *ast.IfStmt:
		n.Cond = i.wrap(n.Cond)

	case *ast.SwitchStmt:
		i.visitSwitchStmt(n)

	case *ast.TypeSwitchStmt:
		i.visitTypeSwitchStmt(n)

	case *ast.SelectStmt:
		// Note: select statements are already handled by go cover.

	case *ast.ForStmt:
		if n.Cond != nil {
			n.Cond = i.wrap(n.Cond)
		}

	case *ast.RangeStmt:
		if n.Key != nil {
			// TODO: i.visitExpr(&n.Key)
		}
		if n.Value != nil {
			// TODO: i.visitExpr(&n.Value)
		}
		// TODO: i.visitExpr(&n.X)

	case *ast.ValueSpec:
		// TODO: i.visitExprs(n.Values)
	}

	return true
}

func (i *instrumenter) visitSwitchStmt(n *ast.SwitchStmt) {
	tag := n.Tag
	body := n.Body.List

	// A switch statement without an expression compares each expression
	// from its case clauses with true. The initialization statement is
	// not modified.
	if tag == nil {
		for _, body := range n.Body.List {
			list := body.(*ast.CaseClause).List
			for idx, cond := range list {
				if !i.isGobcoCoverCall(cond) {
					list[idx] = i.wrap(cond)
				}
			}
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
	tagExprName := i.nextVarname()

	if n.Init == nil {
		// Convert 'switch expr {}' to 'switch gobco0 := expr; {}'.
		n.Tag = nil

		n.Init = &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(tagExprName)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{tag}}

	} else {
		// The initialization statements are executed in a new scope,
		// so convert the existing 'switch' statement to an empty one,
		// just to have this scope.
		//
		// The same scope is used for storing the tag expression in a
		// variable, as the variable names don't overlap.
		*n = ast.SwitchStmt{Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.CaseClause{
				Body: []ast.Stmt{
					n.Init,
					&ast.AssignStmt{
						Lhs: []ast.Expr{ast.NewIdent(tagExprName)},
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
			eq := ast.BinaryExpr{
				X:  ast.NewIdent(tagExprName),
				Op: token.EQL,
				Y:  expr,
			}
			eqlStr := i.strEql(tag, expr)
			clause.List[j] = i.wrapText(&eq, expr.Pos(), eqlStr)
		}
	}
}

// visitTypeSwitchStmt instruments a type switch statement;
// see testdata/instrumenter/TypeSwitchStmt.go.
func (i *instrumenter) visitTypeSwitchStmt(ts *ast.TypeSwitchStmt) {

	// The body of the outer switch statement,
	// containing a few assignments to capture the tag expression,
	// followed by an ordinary switch statement.
	var newBody []ast.Stmt

	// Get access to the tag expression and the optional variable
	// name from 'switch name := expr.(type) {}'.
	tagExprName := ""
	var tagExpr *ast.TypeAssertExpr
	if assign, ok := ts.Assign.(*ast.AssignStmt); ok {
		tagExprName = assign.Lhs[0].(*ast.Ident).Name
		tagExpr = assign.Rhs[0].(*ast.TypeAssertExpr)
	} else {
		tagExpr = ts.Assign.(*ast.ExprStmt).X.(*ast.TypeAssertExpr)
	}

	// tmp0 := switch.tagExpr
	tmp0 := i.nextVarname()
	newBody = append(newBody, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(tmp0)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{tagExpr.X},
	})
	newBody = append(newBody, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("_")},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent(tmp0)},
	})

	// Collect the type tests from all case clauses in local variables,
	// so that the following switch statement can easily and uniformly
	// access them.
	type localVar struct {
		varname string
		code    string
	}
	var vars []localVar
	for _, stmt := range ts.Body.List {
		clause := stmt.(*ast.CaseClause)
		for _, typ := range clause.List {
			v := i.nextVarname()
			vars = append(vars, localVar{
				varname: v,
				code:    i.strEql(tagExpr, typ),
			})

			if ident, ok := typ.(*ast.Ident); ok && ident.Name == "nil" {
				newBody = append(newBody, &ast.AssignStmt{
					Lhs: []ast.Expr{
						ast.NewIdent(v),
					},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						i.skipExpr(&ast.BinaryExpr{
							X:  ast.NewIdent(tmp0),
							Op: token.EQL,
							Y:  ast.NewIdent("nil"),
						}, false),
					},
				})
			} else {
				newBody = append(newBody, &ast.AssignStmt{
					Lhs: []ast.Expr{
						ast.NewIdent("_"),
						ast.NewIdent(v),
					},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.TypeAssertExpr{
							X:    ast.NewIdent(tmp0),
							Type: typ,
						},
					},
				})
			}
		}
	}

	// Now handle the collected type tests in a single switch statement.
	var newClauses []ast.Stmt
	for _, stmt := range ts.Body.List {
		clause := stmt.(*ast.CaseClause)

		var newList []ast.Expr
		var newBody []ast.Stmt

		var singleType ast.Expr
		if len(clause.List) == 1 {
			ident, ok := clause.List[0].(*ast.Ident)
			if !(ok && ident.Name == "nil") {
				singleType = clause.List[0]
			}
		}

		if tagExprName != "" {
			if singleType != nil {
				// tagExprName := tmp0.(singleType)
				newBody = append(newBody, &ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(tagExprName)},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{&ast.TypeAssertExpr{
						X:    ast.NewIdent(tmp0),
						Type: singleType,
					}},
				})
			} else {
				// tagExprName := tmp0
				newBody = append(newBody, &ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(tagExprName)},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{ast.NewIdent(tmp0)},
				})
			}
			// _ = tagExprName
			newBody = append(newBody, &ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent("_")},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{ast.NewIdent(tagExprName)},
			})
		}
		newBody = append(newBody, clause.Body...)

		for range clause.List {
			localVar := vars[0]
			vars = vars[1:]

			ident := ast.NewIdent(localVar.varname)
			wrapped := i.wrapText(ident, tagExpr.Pos(), localVar.code)
			newList = append(newList, i.skipExpr(wrapped, false))
		}

		newClauses = append(newClauses, &ast.CaseClause{
			List: newList,
			Body: newBody,
		})
	}
	newBody = append(newBody, &ast.SwitchStmt{
		Body: &ast.BlockStmt{
			List: newClauses,
		},
	})

	// The type of the outer type switch statement cannot be changed.
	// Replace the tag expression with a dummy expression,
	// and match that expression in a 'default' clause.
	ts.Assign = &ast.ExprStmt{
		X: &ast.TypeAssertExpr{
			X: &ast.CallExpr{
				Fun: &ast.InterfaceType{
					Methods: &ast.FieldList{},
				},
				Args: []ast.Expr{
					&ast.BasicLit{
						Kind:  token.INT,
						Value: "0",
					},
				},
			},
		},
	}
	ts.Body = &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.CaseClause{
				Body: newBody,
			},
		},
	}
}

// visitExprs wraps the given expression list for coverage.
func (i *instrumenter) visitExprs(exprs []ast.Expr) {
	for idx := range exprs {
		i.visitExpr(&exprs[idx])
	}
}

// visitExpr wraps boolean expressions in a call to gobcoCover, thereby
// counting how often these expressions are evaluated.
func (i *instrumenter) visitExpr(exprPtr *ast.Expr) {
	if i.shouldSkip(*exprPtr) {
		return
	}

	// Handle all expression nodes.
	// The order of the cases matches the order in ast.Walk.
	switch expr := (*exprPtr).(type) {

	case *ast.CompositeLit:
		i.visitExprs(expr.Elts)

	case *ast.ParenExpr:
		i.visitExpr(&expr.X)

	case *ast.SelectorExpr:
		// TODO: i.visitExpr(&expr.X)

	case *ast.IndexExpr:
		i.visitExpr(&expr.X)
		i.visitExpr(&expr.Index)

	case *ast.SliceExpr:
		// TODO: i.visitExpr(&expr.X)
		if expr.Low != nil {
			// TODO: i.visitExpr(&expr.Low)
		}
		if expr.High != nil {
			// TODO: i.visitExpr(&expr.High)
		}
		if expr.Max != nil {
			// TODO: i.visitExpr(&expr.Max)
		}

	case *ast.TypeAssertExpr:
		// TODO: i.visitExpr(&expr.X)

	case *ast.CallExpr:
		// TODO: i.visitExpr(&expr.Fun)
		// TODO: i.visitExprs(&expr.Args)

	case *ast.StarExpr:
		// TODO: i.visitExpr(&expr.X)

	case *ast.UnaryExpr:
		i.visitExpr(&expr.X)

	case *ast.BinaryExpr:
		if expr.Op.Precedence() == token.EQL.Precedence() {
			*exprPtr = i.wrap(expr)
		}
		// TODO: i.visitExpr(&expr.X)
		// TODO: i.visitExpr(&expr.Y)

	case *ast.KeyValueExpr:
		i.visitExpr(&expr.Key)
		i.visitExpr(&expr.Value)
	}
}

// wrap returns the given expression surrounded by a function call to
// gobcoCover and remembers the location and text of the expression,
// for later generating the table of coverage points.
func (i *instrumenter) wrap(cond ast.Expr) ast.Expr {
	if _, ok := cond.(*ast.UnaryExpr); ok {
		return cond
	}
	return i.wrapText(cond, cond.Pos(), i.str(cond))
}

// wrapText returns the expression cond surrounded by a function call to
// gobcoCover and remembers the location and text of the expression,
// for later generating the table of coverage points.
//
// The position pos must point to the uninstrumented code that is most closely
// related to the instrumented condition. Especially for switch statements, the
// position may differ from the expression that is wrapped.
func (i *instrumenter) wrapText(cond ast.Expr, pos token.Pos, code string) ast.Expr {
	if !pos.IsValid() {
		panic("pos must refer to the code from before instrumentation")
	}
	if i.shouldSkip(cond) {
		return cond
	}

	origStart := i.fset.Position(pos)
	if !strings.HasSuffix(origStart.Filename, ".go") {
		return cond // don't wrap generated code, such as yacc parsers
	}

	start := origStart
	idx := i.addCond(start.String(), code)

	cover := ast.NewIdent("gobcoCover")
	cover.NamePos = pos
	return &ast.CallExpr{
		Fun: cover,
		Args: []ast.Expr{
			&ast.BasicLit{Kind: token.INT, Value: fmt.Sprint(idx)},
			cond}}
}

func (i *instrumenter) isGobcoCoverCall(expr ast.Expr) bool {
	if call, ok := expr.(*ast.CallExpr); ok {
		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "gobcoCover" {
			return true
		}
	}
	return false
}

// addCond remembers a condition and returns its internal ID, which is then
// used as an argument to the gobcoCover function.
func (i *instrumenter) addCond(start, code string) int {
	i.conds = append(i.conds, cond{start, code})
	return len(i.conds) - 1
}

// strEql returns the string representation of (lhs == rhs).
func (i *instrumenter) strEql(lhs ast.Expr, rhs ast.Expr) string {
	// Do not use printer.Fprint here, as that would add unnecessary
	// whitespace after the '==' and would also compress the space
	// around the left-hand operand.

	needsParentheses := func(expr ast.Expr) bool {
		switch expr := expr.(type) {
		case *ast.Ident,
			*ast.SelectorExpr,
			*ast.BasicLit,
			*ast.IndexExpr,
			*ast.CompositeLit,
			*ast.UnaryExpr,
			*ast.CallExpr,
			*ast.TypeAssertExpr,
			*ast.ParenExpr:
			return false
		case *ast.BinaryExpr:
			return expr.Op.Precedence() <= token.EQL.Precedence()
		}
		return true
	}

	lp := needsParentheses(lhs)
	rp := needsParentheses(rhs)

	opening := map[bool]string{true: "("}
	closing := map[bool]string{true: ")"}

	return fmt.Sprintf("%s%s%s == %s%s%s",
		opening[lp], i.str(lhs), closing[lp],
		opening[rp], i.str(rhs), closing[rp])
}

func (i *instrumenter) skipExpr(expr ast.Expr, onlyThis bool) ast.Expr {
	i.skip[expr] = onlyThis
	return expr
}

func (i *instrumenter) skipStmt(stmt ast.Stmt, onlyThis bool) ast.Stmt {
	i.skip[stmt] = onlyThis
	return stmt
}

func (i *instrumenter) shouldSkip(n ast.Node) bool {
	_, skip := i.skip[n]
	return skip
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

//go:embed templates/gobco_fixed.go
var fixedTemplate string

//go:embed templates/gobco_fixed_test.go
var fixedTestTemplate string

//go:embed templates/gobco_variable_test.go
var variableTestTemplate string

func (i *instrumenter) writeGobcoFiles(tmpDir string, pkgname string) {
	fixPkgname := func(str string) string {
		return strings.Replace(str, "package main\n", "package "+pkgname+"\n", 1)
	}
	i.writeFile(filepath.Join(tmpDir, "gobco_fixed.go"), fixPkgname(fixedTemplate))
	i.writeGobcoGo(filepath.Join(tmpDir, "gobco_variable.go"), pkgname)

	i.writeFile(filepath.Join(tmpDir, "gobco_fixed_test.go"), fixPkgname(fixedTestTemplate))
	if !i.hasTestMain {
		i.writeFile(filepath.Join(tmpDir, "gobco_variable_test.go"), fixPkgname(variableTestTemplate))
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
	// If the below expression panics due to end.Offset being 0,
	// this means that expr is not entirely from the original code.
	return i.text[start.Offset:end.Offset]
}

func (i *instrumenter) nextVarname() string {
	varname := fmt.Sprintf("gobco%d", i.exprs)
	i.exprs++
	return varname
}
