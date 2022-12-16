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
	"reflect"
	"runtime"
	"sort"
	"strings"
)

// cond is a condition that appears somewhere in the source code.
type cond struct {
	start string // human-readable position in the file, e.g. "main.go:17:13"
	code  string // the source code of the condition
}

// exprSubst describes that an expression node will later be replaced with
// another expression.
type exprSubst struct {
	ref  *ast.Expr
	expr ast.Expr
	pos  token.Pos
	text string
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
	marked      map[ast.Node]bool
	exprSubst   map[ast.Expr]*exprSubst
	stmtRef     map[ast.Stmt]*ast.Stmt
	stmtSubst   map[ast.Stmt]ast.Stmt

	text    string // during instrumentFile(), the text of the current file
	varname int    // to produce unique local variable names
}

// instrument modifies the code of the Go package from srcDir by adding
// counters for code coverage, writing the instrumented code to dstDir.
// If base is given, only that file is instrumented.
func (i *instrumenter) instrument(srcDir, base, dstDir string) {
	i.fset = token.NewFileSet()

	isRelevant := func(info os.FileInfo) bool {
		return base == "" || info.Name() == base
	}

	// Comments are needed for build tags such as '//go:build 386' or
	// '//go:embed'.
	mode := parser.ParseComments
	pkgs, err := parser.ParseDir(i.fset, srcDir, isRelevant, mode)
	if err != nil {
		panic(err)
	}

	for pkgname, pkg := range pkgs {
		// Sort files, for deterministic output.
		var files []string
		for file := range pkg.Files {
			files = append(files, file)
		}
		sort.Strings(files)

		for _, filename := range files {
			i.instrumentFile(filename, pkg.Files[filename], dstDir)
		}

		// XXX: What if the directory contains multiple packages?
		//  pkg and pkg_test
		i.writeGobcoFiles(dstDir, pkgname)
	}
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
		i.instrumentFileNode(astFile)
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

func (i *instrumenter) instrumentFileNode(f *ast.File) {
	ast.Inspect(f, i.markConds)
	ast.Inspect(f, i.findRefs)
	ast.Inspect(f, i.prepareStmts)
	ast.Inspect(f, i.replace)
}

// markConds remembers the conditions that will be wrapped.
//
// Each expression that is syntactically a boolean condition is marked to be
// replaced later with a function call of the form gobcoCover(id++, cond).
//
// If the nodes were replaced directly instead of only being marked,
// the final list of wrapped conditions would not be in declaration order.
// For example, when a binary expression is visited,
// its direct operands are marked, but not any of the indirect operands.
// The indirect operands are marked in later calls to markConds.
// A direct right-hand operand would thus
// be marked before an indirect left-hand operand.
//
// To avoid wrapping complex conditions redundantly, unmark them.
// For example, after the whole file is visited,
// in a condition 'a && !c', only 'a' and 'c' are marked, but not '!' or '&&'.
func (i *instrumenter) markConds(n ast.Node) bool {
	// The order of the cases matches the order in ast.Walk.
	switch n := n.(type) {

	case *ast.ParenExpr:
		if i.marked[n] {
			delete(i.marked, n)
			i.marked[n.X] = true
		}

	case *ast.UnaryExpr:
		if n.Op == token.NOT {
			delete(i.marked, n)
			i.marked[n.X] = true
		}

	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			delete(i.marked, n)
			i.marked[n.X] = true
			i.marked[n.Y] = true
		}
		if n.Op.Precedence() == token.EQL.Precedence() {
			i.marked[n] = true
		}

	case *ast.IfStmt:
		i.marked[n.Cond] = true

	case *ast.SwitchStmt:
		if n.Tag == nil {
			for _, clause := range n.Body.List {
				for _, expr := range clause.(*ast.CaseClause).List {
					i.marked[expr] = true
				}
			}
		}

	case *ast.ForStmt:
		if n.Cond != nil {
			i.marked[n.Cond] = true
		}

	case *ast.GenDecl:
		if n.Tok == token.CONST {
			return false
		}
	}

	return true
}

// findRefs saves for each marked condition where in the AST it is referenced.
// Since the AST is a tree, there is only ever one such reference.
//
// Like in markConds, the conditions are not visited in declaration order,
// therefore the actual wrapping is done later.
func (i *instrumenter) findRefs(n ast.Node) bool {
	if n == nil {
		return true
	}

	// In each struct field, remember the reference that points there.
	//
	// Since many ast.Node types have ast.Expr fields,
	// it is simpler to use reflection to find all these fields.
	if node := reflect.ValueOf(n); node.Type().Kind() == reflect.Ptr {
		if typ := node.Type().Elem(); typ.Kind() == reflect.Struct {
			str := node.Elem()
			for fi, nf := 0, str.NumField(); fi < nf; fi++ {
				field := str.Field(fi)

				switch val := field.Interface().(type) {

				case ast.Expr:
					expr := val
					if i.marked[expr] {
						delete(i.marked, expr)
						ref := field.Addr().Interface().(*ast.Expr)
						i.exprSubst[expr] = &exprSubst{
							ref, expr, expr.Pos(), i.str(expr),
						}
					}

				case []ast.Expr:
					for ei, expr := range val {
						if i.marked[expr] {
							delete(i.marked, expr)
							i.exprSubst[expr] = &exprSubst{
								&val[ei], expr, expr.Pos(), i.str(expr),
							}
						}
					}

				case ast.Stmt:
					if field.Type() == reflect.TypeOf((*ast.Stmt)(nil)).Elem() {
						i.stmtRef[val] = field.Addr().Interface().(*ast.Stmt)
					}

				case []ast.Stmt:
					for si, stmt := range val {
						i.stmtRef[stmt] = &val[si]
					}
				}
			}
		}
	}

	return true
}

func (i *instrumenter) prepareStmts(n ast.Node) bool {
	switch n := n.(type) {

	case *ast.SwitchStmt:
		i.visitSwitchStmt(n)

	case *ast.TypeSwitchStmt:
		i.visitTypeSwitchStmt(n)

	case *ast.FuncDecl:
		i.varname = 0
	}

	return true
}

func (i *instrumenter) visitSwitchStmt(n *ast.SwitchStmt) {
	if n.Tag == nil {
		return // Already handled in instrumenter.markConds.
	}

	var gen codeGenerator

	// In a switch statement with an expression, the expression is
	// evaluated once and is then compared to each expression from the
	// case clauses.
	//
	// In the instrumented switch statement, the tag expression always has
	// boolean type, and the expressions in the case clauses are instrumented
	// to calls of the form 'gobcoCover(id++, tag == expr)'.
	tagExprName := i.nextVarname()
	tagExprUsed := false

	// Convert each expression from the 'case' clauses to an expression of
	// the form 'gobcoCover(id, tag == expr)'.
	for _, clause := range n.Body.List {
		clause := clause.(*ast.CaseClause)
		for j, expr := range clause.List {
			i.exprSubst[expr] = &exprSubst{
				&clause.List[j],
				gen.eql(
					gen.ident(tagExprName),
					expr,
				),
				expr.Pos(),
				i.strEql(n.Tag, expr),
			}
			tagExprUsed = true
		}
	}

	var newBody []ast.Stmt
	if n.Init != nil {
		newBody = append(newBody, n.Init)
	}
	tagRef := []ast.Expr{n.Tag}
	newBody = append(newBody, gen.defineExprs(tagExprName, tagRef))
	if !tagExprUsed {
		newBody = append(newBody, gen.use(gen.ident(tagExprName)))
	}
	newBody = append(newBody, &ast.SwitchStmt{
		Body: gen.block(n.Body.List),
	})

	// The initialization statements are executed in a new scope.
	// Use this scope for storing the tag expression in a variable
	// as well, as the variable names don't overlap.
	i.stmtSubst[n] = &ast.BlockStmt{
		Lbrace: n.Switch,
		List:   newBody,
		Rbrace: n.Switch,
	}

	// n.Tag is the only expression node whose reference is not preserved
	// in the instrumented tree, so update it.
	if s := i.exprSubst[n.Tag]; s != nil {
		s.ref = &tagRef[0]
	}
}

func (i *instrumenter) visitTypeSwitchStmt(ts *ast.TypeSwitchStmt) {

	var gen codeGenerator

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

	// evaluatedTagExpr := switch.tagExpr
	evaluatedTagExpr := i.nextVarname()
	evaluatedTagExprUsed := false

	// Collect the type tests from all case clauses,
	// to keep the following switch statement simple and uniform.
	type typeTest struct {
		pos     token.Pos
		varname string
		code    string
	}
	var tests []typeTest
	var assignments []ast.Stmt
	for _, stmt := range ts.Body.List {
		for _, typ := range stmt.(*ast.CaseClause).List {
			v := i.nextVarname()
			tests = append(tests, typeTest{
				typ.Pos(),
				v,
				i.strEql(tagExpr, typ),
			})

			if ident, ok := typ.(*ast.Ident); ok && ident.Name == "nil" {
				assignments = append(assignments, gen.define(
					v,
					gen.eql(
						gen.ident(evaluatedTagExpr),
						gen.ident("nil"),
					),
				))
			} else {
				assignments = append(assignments, gen.defineIsType(
					v,
					gen.ident(evaluatedTagExpr),
					typ,
				))
			}
			evaluatedTagExprUsed = true
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
				newBody = append(newBody, gen.define(
					tagExprName,
					&ast.TypeAssertExpr{
						X:    gen.ident(evaluatedTagExpr),
						Type: singleType,
					},
				))
			} else {
				newBody = append(newBody, gen.define(
					tagExprName,
					gen.ident(evaluatedTagExpr),
				))
			}
			evaluatedTagExprUsed = true

			newBody = append(newBody, gen.use(gen.ident(tagExprName)))
		}
		newBody = append(newBody, clause.Body...)

		for range clause.List {
			test := tests[0]
			tests = tests[1:]

			ident := gen.ident(test.varname)
			wrapped := i.wrapText(ident, test.pos, test.code)
			newList = append(newList, wrapped)
		}

		newClauses = append(newClauses, gen.caseClause(newList, newBody))
	}

	var newBody []ast.Stmt
	if evaluatedTagExprUsed {
		newBody = append(newBody, gen.define(
			evaluatedTagExpr,
			tagExpr.X,
		))
	} else {
		newBody = append(newBody, gen.use(tagExpr.X))
	}
	newBody = append(newBody, assignments...)
	newBody = append(newBody, &ast.SwitchStmt{
		Body: gen.block(newClauses),
	})

	if ts.Init != nil {
		i.stmtSubst[ts] = &ast.SwitchStmt{
			Switch: ts.Switch,
			Init:   ts.Init,
			Body: gen.block(
				[]ast.Stmt{
					gen.caseClause(nil, newBody),
				},
			),
		}
	} else {
		i.stmtSubst[ts] = &ast.BlockStmt{
			Lbrace: ts.Switch,
			List:   newBody,
		}
	}
}

// replace replaces each prepared node with the instrumentation code,
// in declaration order.
func (i *instrumenter) replace(n ast.Node) bool {
	switch n := n.(type) {

	case ast.Expr:
		if s := i.exprSubst[n]; s != nil {
			*s.ref = i.wrapText(s.expr, s.pos, s.text)
		}

	case ast.Stmt:
		if stmt := i.stmtSubst[n]; stmt != nil {
			*i.stmtRef[n] = stmt
		}
	}

	return true
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

	start := i.fset.Position(pos)
	if !strings.HasSuffix(start.Filename, ".go") {
		return cond // don't wrap generated code, such as yacc parsers
	}

	idx := i.addCond(start.String(), code)

	var gen codeGenerator
	return gen.callGobcoCover(idx, pos, cond)
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
			var gen codeGenerator
			*arg = &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   gen.ident("gobcoCounts"),
					Sel: gen.ident("finish")},
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
	varname := fmt.Sprintf("gobco%d", i.varname)
	i.varname++
	return varname
}

type codeGenerator struct{}

func (gen *codeGenerator) ident(name string) *ast.Ident {
	return &ast.Ident{
		Name: name, // TODO: NamePos
	}
}

func (gen *codeGenerator) eql(x ast.Expr, y ast.Expr) *ast.BinaryExpr {
	return &ast.BinaryExpr{
		X:  x,
		Op: token.EQL, // TODO: OpPos
		Y:  y,
	}
}

func (gen *codeGenerator) callGobcoCover(idx int, pos token.Pos, cond ast.Expr) ast.Expr {
	return &ast.CallExpr{
		Fun: &ast.Ident{
			Name:    "gobcoCover",
			NamePos: pos,
		},
		Lparen: pos,
		Args: []ast.Expr{
			&ast.BasicLit{
				ValuePos: pos,
				Kind:     token.INT,
				Value:    fmt.Sprint(idx),
			},
			cond,
		},
		Rparen: pos,
	}
}

func (gen *codeGenerator) define(lhs string, rhs ast.Expr) *ast.AssignStmt {
	return gen.defineExprs(lhs, []ast.Expr{rhs})
}

func (gen *codeGenerator) defineExprs(lhs string, rhs []ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{gen.ident(lhs)},
		Tok: token.DEFINE, // TODO: TokPos
		Rhs: rhs,
	}
}

// defineIsType generates code for testing whether rhs has the given type.
func (gen *codeGenerator) defineIsType(lhs string, rhs, typ ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{gen.ident("_"), gen.ident(lhs)},
		Tok: token.DEFINE, // TODO: TokPos
		Rhs: []ast.Expr{
			&ast.TypeAssertExpr{
				X:    rhs,
				Type: typ, // TODO: Lparen, Rparen
			},
		},
	}
}

func (gen *codeGenerator) use(rhs ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{gen.ident("_")},
		Tok: token.ASSIGN, // TODO: TokPos
		Rhs: []ast.Expr{rhs},
	}
}

func (gen *codeGenerator) block(stmts []ast.Stmt) *ast.BlockStmt {
	return &ast.BlockStmt{
		List: stmts, // TODO: Lbrace, Rbrace
	}
}

func (gen *codeGenerator) caseClause(list []ast.Expr, body []ast.Stmt) *ast.CaseClause {
	return &ast.CaseClause{
		Case:  token.NoPos, // TODO
		List:  list,
		Colon: token.NoPos, // TODO
		Body:  body,
	}
}
