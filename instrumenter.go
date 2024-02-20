package main

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

// cond is a condition from the code that is instrumented.
type cond struct {
	pos  string // for example "main.go:17:13"
	text string // for example "i > 0"
}

// exprSubst prepares to later replace '*ref' with 'expr'.
type exprSubst struct {
	ref  *ast.Expr
	expr ast.Expr
	pos  token.Pos
	text string
}

// instrumenter rewrites the code of a go package
// by instrumenting all conditions in the code.
type instrumenter struct {
	branch      bool // branch coverage, not condition coverage
	coverTest   bool // also cover the test code
	immediately bool // persist counts after each increment
	listAll     bool // also list conditions that are covered
	fset        *token.FileSet

	// Generates variable names that are unique per function.
	varname int

	// The conditions are first marked as relevant,
	// then some complex conditions are unmarked if they are redundant,
	// and finally they are instrumented in source code order.
	marked map[ast.Expr]bool

	// All conditions and their planned replacements.
	exprSubst map[ast.Expr]*exprSubst

	// Records for each statement the single place where it is referenced.
	stmtRef map[ast.Stmt]*ast.Stmt

	// All statements (expression switch and type switch)
	// and their planned replacements.
	// For simplicity of implementation,
	// a statement can only be replaced with a single other statement,
	// but not with a slice of statements.
	stmtSubst map[ast.Stmt]ast.Stmt

	hasTestMain bool

	// The conditions from the original code that were instrumented,
	// from all files from fset.
	conds []cond
}

// instrument modifies the code of the Go package from srcDir
// by adding counters for code coverage,
// writing the instrumented code to dstDir.
// If singleFile is given, only that file is instrumented.
func (i *instrumenter) instrument(srcDir, singleFile, dstDir string) bool {
	i.fset = token.NewFileSet()

	isRelevant := func(info os.FileInfo) bool {
		return singleFile == "" || info.Name() == singleFile
	}

	// Comments are needed for build tags
	// such as '//go:build 386' or '//go:embed'.
	mode := parser.ParseComments
	pkgsMap, err := parser.ParseDir(i.fset, srcDir, isRelevant, mode)
	ok(err)

	pkgs := sortedPkgs(pkgsMap)
	if len(pkgs) == 0 {
		return false
	}

	for _, pkg := range pkgs {
		forEachFile(pkg, func(name string, file *ast.File) {
			i.instrumentFile(name, file, dstDir)
		})
	}

	pkgPath, err := findPackagePath(srcDir)
	ok(err)

	i.writeGobcoFiles(dstDir, pkgPath, pkgs)
	return true
}

// findPackagePath finds import path of a package that srcDir indicates
func findPackagePath(srcDir string) (string, error) {
	moduleRoot, moduleRel, err := findInModule(srcDir)
	if err != nil {
		return "", err
	}

	// Read the content of the go.mod file
	modFilePath := filepath.Join(moduleRoot, "go.mod")
	modFileContent, err := os.ReadFile(modFilePath)
	if err != nil {
		return "", err
	}

	// Parse the content of the go.mod file
	modFile, err := modfile.Parse("", modFileContent, nil)
	if err != nil {
		return "", err
	}

	// Get the module name from the parsed go.mod file
	moduleName := modFile.Module.Mod.Path

	if moduleRel == "." {
		return moduleName, nil
	} else {
		pkgPath := fmt.Sprintf("%s/%s", moduleName, moduleRel)
		return pkgPath, nil
	}
}

func (i *instrumenter) instrumentFile(filename string, astFile *ast.File, dstDir string) {
	isTest := strings.HasSuffix(filename, "_test.go")
	if (i.coverTest || !isTest) && shouldBuild(filename) {
		i.instrumentFileNode(astFile)
	}
	if isTest {
		i.instrumentTestMain(astFile)
	}

	var out strings.Builder
	ok(printer.Fprint(&out, i.fset, astFile))
	writeFile(filepath.Join(dstDir, filepath.Base(filename)), out.String())
}

func (i *instrumenter) instrumentFileNode(f *ast.File) {
	ast.Inspect(f, i.markConds)
	ast.Inspect(f, i.findRefs)
	ast.Inspect(f, i.prepareStmts)
	ast.Inspect(f, i.replace)
}

// markConds remembers the conditions that will be instrumented later.
//
// Each expression that is syntactically a boolean condition
// is marked to be replaced later
// with a function call of the form GobcoCover(id++, cond).
//
// If the nodes were replaced directly instead of only being marked,
// the final list of wrapped conditions would not be in declaration order.
// For example, when a binary expression is visited,
// its direct operands are marked, but not any of the indirect operands.
// The indirect operands are marked in later calls to markConds.
// A direct right-hand operand would thus be marked
// before an indirect left-hand operand.
//
// Redundant conditions are not instrumented.
// Which conditions are redundant depends on the coverage mode.
//
// In condition coverage mode (the default mode),
// only atomic boolean conditions are marked,
// as the conditions for the complex conditions are redundant.
// For example, in the condition 'a && !c', only 'a' and 'c' are instrumented,
// but not the '!c' or the whole condition.
//
// In branch coverage mode,
// only the whole controlling condition is instrumented.
func (i *instrumenter) markConds(n ast.Node) bool {
	// The order of the cases matches the order in ast.Walk.
	switch n := n.(type) {

	case *ast.ParenExpr:
		if i.marked[n] {
			delete(i.marked, n)
			i.marked[n.X] = true
		}

	case *ast.UnaryExpr:
		if i.branch {
			break
		}
		if n.Op == token.NOT {
			delete(i.marked, n)
			i.marked[n.X] = true
		}

	case *ast.BinaryExpr:
		if i.branch {
			break
		}
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

// findRefs remembers, for each relevant expression or statement,
// from which single location it is referenced.
// This information is later used to replace expressions or statements
// with their instrumented counterparts.
//
// Whenever an expression or a statement from the original AST is moved
// (that is, its direct containing node or field changes),
// its reference must be updated accordingly.
//
// Like in markConds, the conditions are not visited in declaration order,
// therefore the actual replacement is done later.
func (i *instrumenter) findRefs(n ast.Node) bool {
	if n == nil {
		return true
	}

	// For each struct field and slice element,
	// remember the reference that points to it.
	//
	// Since there are many ast.Node types that have ast.Expr fields,
	// it is simpler to use reflection to find all these fields
	// instead of listing the known types and their fields explicitly.
	if node := reflect.ValueOf(n); node.Type().Kind() == reflect.Ptr {
		if typ := node.Type().Elem(); typ.Kind() == reflect.Struct {
			str := node.Elem()
			for fi, nf := 0, str.NumField(); fi < nf; fi++ {
				i.findRefsField(str.Field(fi))
			}
		}
	}

	return true
}

// findRefsField remembers, for a particular field of a node in the AST,
// from which single location it is referenced.
func (i *instrumenter) findRefsField(field reflect.Value) {
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

func (i *instrumenter) prepareStmts(n ast.Node) bool {
	switch n := n.(type) {

	case *ast.SwitchStmt:
		i.prepareSwitchStmt(n)

	case *ast.TypeSwitchStmt:
		i.prepareTypeSwitchStmt(n)

	case *ast.FuncDecl:
		i.varname = 0
	}

	return true
}

func (i *instrumenter) prepareSwitchStmt(n *ast.SwitchStmt) {
	if n.Tag == nil {
		return // Already handled in instrumenter.markConds.
	}

	// In a switch statement with a tag expression,
	// the expression is evaluated once
	// and is then compared to each expression from the case clauses.
	//
	// In the instrumented switch statement,
	// the tag expression always has boolean type,
	// and the expressions in the case clauses are replaced
	// with calls of the form 'GobcoCover(id++, tag == expr)'.
	tagExprName := i.nextVarname()
	tagExprUsed := false

	// Remember each expression from the 'case' clauses,
	// to later replace it with an expression
	// of the form 'GobcoCover(id++, tag == expr)'.
	for _, clause := range n.Body.List {
		clause := clause.(*ast.CaseClause)
		for j, expr := range clause.List {
			gen := codeGenerator{expr.Pos()}
			i.exprSubst[expr] = &exprSubst{
				&clause.List[j],
				gen.eql(tagExprName, expr),
				expr.Pos(),
				i.strEql(n.Tag, expr),
			}
			tagExprUsed = true
		}
	}

	gen := codeGenerator{n.Pos()}
	var newBody []ast.Stmt
	if n.Init != nil {
		newBody = append(newBody, n.Init)
	}
	tagRef := []ast.Expr{n.Tag}
	newBody = append(newBody, gen.defineExprs(tagExprName, tagRef))
	if !tagExprUsed {
		newBody = append(newBody, gen.use(gen.ident(tagExprName)))
	}
	newBody = append(newBody, gen.switchStmt(nil, n.Body))
	i.fixStmtRefs(newBody)

	// The initialization statements are executed in a new scope.
	// Reuse the same scope for storing the tag expression in a variable,
	// as the variable names don't overlap.
	i.stmtSubst[n] = gen.block(newBody)

	// n.Tag moves from the switch statement to an assignment,
	// so update the reference to it.
	if s := i.exprSubst[n.Tag]; s != nil {
		s.ref = &tagRef[0]
	}
}

func (i *instrumenter) prepareTypeSwitchStmt(ts *ast.TypeSwitchStmt) {
	gen := codeGenerator{ts.Switch}

	// Get access to the tag expression
	// and the optional variable name
	// from 'switch name := expr.(type) {}'.
	tagExprName := ""
	var tagExpr *ast.TypeAssertExpr
	if assign, ok := ts.Assign.(*ast.AssignStmt); ok {
		tagExprName = assign.Lhs[0].(*ast.Ident).Name
		tagExpr = assign.Rhs[0].(*ast.TypeAssertExpr)
	} else {
		tagExpr = ts.Assign.(*ast.ExprStmt).X.(*ast.TypeAssertExpr)
	}

	tag := "" // The evaluated TypeSwitchStmt.Tag

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
			if tag == "" {
				tag = i.nextVarname()
			}
			v := i.nextVarname()
			test := typeTest{typ.Pos(), v, i.strEql(tagExpr, typ)}
			tests = append(tests, test)

			posTyp := gen.reposition(typ)
			def := gen.defineIsType(v, tag, posTyp)
			assignments = append(assignments, def)
		}
	}

	// Now handle the collected type tests in a single switch statement.
	var newClauses []ast.Stmt
	for _, stmt := range ts.Body.List {
		clause := stmt.(*ast.CaseClause)

		var newList []ast.Expr
		var newBody []ast.Stmt

		if tagExprName != "" {
			if tag == "" {
				tag = i.nextVarname()
			}
			if len(clause.List) == 1 && !isNilIdent(clause.List[0]) {
				expr := gen.typeAssertExpr(tag, clause.List[0])
				def := gen.define(tagExprName, expr)
				newBody = append(newBody, def)
			} else {
				def := gen.define(tagExprName, gen.ident(tag))
				newBody = append(newBody, def)
			}

			newBody = append(newBody, gen.use(gen.ident(tagExprName)))
		}
		newBody = append(newBody, clause.Body...)
		i.fixStmtRefs(newBody)

		for range clause.List {
			test := tests[0]
			tests = tests[1:]

			gen := codeGenerator{test.pos}
			ident := gen.ident(test.varname)
			wrapped := i.callCover(ident, test.pos, test.code)
			newList = append(newList, wrapped)
		}

		gen := codeGenerator{clause.Pos()}
		newClauses = append(newClauses, gen.caseClause(newList, newBody))
	}

	if tag == "" {
		return
	}

	var newBody []ast.Stmt
	if ts.Init != nil {
		newBody = append(newBody, ts.Init)
	}
	newBody = append(newBody, gen.define(tag, tagExpr.X))
	newBody = append(newBody, assignments...)
	newBody = append(newBody, gen.switchStmt(nil, gen.block(newClauses)))
	i.fixStmtRefs(newBody)

	i.stmtSubst[ts] = gen.block(newBody)
}

func (i *instrumenter) fixStmtRefs(stmts []ast.Stmt) {
	for si, stmt := range stmts {
		i.stmtRef[stmt] = &stmts[si]
	}
}

// replace replaces each prepared node with the instrumentation code,
// in declaration order.
func (i *instrumenter) replace(n ast.Node) bool {
	switch n := n.(type) {

	case ast.Expr:
		if s := i.exprSubst[n]; s != nil {
			*s.ref = i.callCover(s.expr, s.pos, s.text)
		}

	case ast.Stmt:
		if stmt := i.stmtSubst[n]; stmt != nil {
			*i.stmtRef[n] = stmt
		}
	}

	return true
}

// callCover returns expr surrounded by a function call to GobcoCover
// and remembers the location and text of the expression,
// for later generating the table of coverage points.
//
// The position pos must point to the uninstrumented code
// that is most closely related to the instrumented condition.
// Especially for switch statements,
// the position may differ from the expression that is wrapped.
func (i *instrumenter) callCover(expr ast.Expr, pos token.Pos, code string) ast.Expr {
	assert(pos.IsValid(), "pos must refer to the code from before instrumentation")

	start := i.fset.Position(pos)
	if !strings.HasSuffix(start.Filename, ".go") {
		// don't instrument generated code, such as yacc parsers
		return expr
	}

	i.conds = append(i.conds, cond{start.String(), code})
	idx := len(i.conds) - 1

	gen := codeGenerator{pos}
	return gen.callGobcoCover(idx, expr)
}

// strEql returns the string representation of (lhs == rhs).
func (i *instrumenter) strEql(lhs ast.Expr, rhs ast.Expr) string {
	// Do not use printer.Fprint here,
	// as that would add unnecessary whitespace after the '=='
	// (due to the position information in the nodes)
	// and would also compress the space inside the operands.

	lp := needsParenthesesForEql(lhs)
	rp := needsParenthesesForEql(rhs)

	opening := map[bool]string{true: "("}
	closing := map[bool]string{true: ")"}

	return fmt.Sprintf("%s%s%s == %s%s%s",
		opening[lp], i.str(lhs), closing[lp],
		opening[rp], i.str(rhs), closing[rp])
}

func needsParenthesesForEql(expr ast.Expr) bool {
	switch expr := expr.(type) {
	case *ast.Ident,
		*ast.BasicLit,
		*ast.CompositeLit,
		*ast.ParenExpr,
		*ast.SelectorExpr,
		*ast.IndexExpr,
		*ast.SliceExpr,
		*ast.TypeAssertExpr,
		*ast.CallExpr,
		*ast.StarExpr,
		*ast.UnaryExpr,
		*ast.ArrayType,
		*ast.StructType,
		*ast.FuncType,
		*ast.InterfaceType,
		*ast.MapType,
		*ast.ChanType:
		return false
	case *ast.BinaryExpr:
		return expr.Op.Precedence() <= token.EQL.Precedence()
	}
	return true
}

func (i *instrumenter) instrumentTestMain(astFile *ast.File) {
	seenOsExit := false

	wrapOsExit := func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if fn, ok := call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := fn.X.(*ast.Ident); ok {
					if pkg.Name == "os" && fn.Sel.Name == "Exit" {
						seenOsExit = true
						gen := codeGenerator{n.Pos()}
						call.Args[0] = gen.callFinish(call.Args[0])
					}
				}
			}
		}
		return true
	}

	for _, decl := range astFile.Decls {
		if decl, ok := decl.(*ast.FuncDecl); ok {
			if decl.Recv == nil && decl.Name.Name == "TestMain" {
				i.hasTestMain = true

				ast.Inspect(decl.Body, wrapOsExit)
				assert(seenOsExit, "can only handle TestMain with explicit call to os.Exit")
			}
		}
	}
}

//go:embed templates/gobco_fixed.go
var fixedTemplate string

//go:embed templates/gobco_no_testmain_test.go
var noTestMainTemplate string

func (i *instrumenter) writeGobcoFiles(tmpDir, pkgPath string, pkgs []*ast.Package) {
	pkgname := pkgs[0].Name
	fixPkgname := func(str string) string {
		str = strings.TrimPrefix(str, "//go:build ignore\n// +build ignore\n\n")
		return strings.Replace(str, "package main\n", "package "+pkgname+"\n", 1)
	}
	writeFile(filepath.Join(tmpDir, "gobco_fixed.go"), fixPkgname(fixedTemplate))
	i.writeGobcoGo(filepath.Join(tmpDir, "gobco_variable.go"), pkgname)

	if !i.hasTestMain {
		writeFile(filepath.Join(tmpDir, "gobco_no_testmain_test.go"), fixPkgname(noTestMainTemplate))
	}

	i.writeGobcoBlackBox(pkgs, tmpDir, pkgPath)
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
		sb.WriteString(fmt.Sprintf("\t\t{%q, %q, 0, 0},\n",
			cond.pos, cond.text))
	}
	sb.WriteString("\t},\n")
	sb.WriteString("}\n")

	writeFile(filename, sb.String())
}

// writeGobcoBlackBox makes the function 'GobcoCover' available
// to black box tests (those in 'package x_test' instead of 'package x')
// by delegating to the function of the same name in the main package.
func (i *instrumenter) writeGobcoBlackBox(pkgs []*ast.Package, dstDir, pkgPath string) {
	if len(pkgs) < 2 {
		return
	}

	pkgName := filepath.Base(pkgPath)

	text := "" +
		"package " + pkgs[0].Name + "_test\n" +
		"\n" +
		"import " + pkgName + " \"" + pkgPath + "\"\n" +
		"\n" +
		"func GobcoCover(idx int, cond bool) bool {\n" +
		"\t" + "return " + pkgName + ".GobcoCover(idx, cond)\n" +
		"}\n"

	writeFile(filepath.Join(dstDir, "gobco_bridge_test.go"), text)
}

func (i *instrumenter) str(expr ast.Expr) string {
	var sb strings.Builder
	ok(printer.Fprint(&sb, i.fset, expr))
	return sb.String()
}

func (i *instrumenter) nextVarname() string {
	varname := fmt.Sprintf("gobco%d", i.varname)
	i.varname++
	return varname
}

// codeGenerator generates source code with correct position information.
// If the code were generated with [token.NoPos] instead,
// the comments would be moved to incorrect locations.
type codeGenerator struct {
	pos token.Pos
}

func (gen codeGenerator) ident(name string) *ast.Ident {
	return &ast.Ident{
		NamePos: gen.pos,
		Name:    name,
	}
}

func (gen codeGenerator) eql(x string, y ast.Expr) *ast.BinaryExpr {
	return &ast.BinaryExpr{
		X:     gen.ident(x),
		OpPos: gen.pos,
		Op:    token.EQL,
		Y:     y,
	}
}

func (gen codeGenerator) typeAssertExpr(x string, typ ast.Expr) ast.Expr {
	return &ast.TypeAssertExpr{
		X:      gen.ident(x),
		Lparen: gen.pos,
		Type:   typ,
		Rparen: gen.pos,
	}
}

func (gen codeGenerator) callFinish(arg ast.Expr) ast.Expr {
	return &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   gen.ident("gobcoCounts"),
			Sel: gen.ident("finish"),
		},
		Lparen:   gen.pos,
		Args:     []ast.Expr{arg},
		Ellipsis: token.NoPos,
		Rparen:   gen.pos,
	}
}

func (gen codeGenerator) callGobcoCover(idx int, cond ast.Expr) ast.Expr {
	return &ast.CallExpr{
		Fun:    gen.ident("GobcoCover"),
		Lparen: gen.pos,
		Args: []ast.Expr{
			&ast.BasicLit{
				ValuePos: gen.pos,
				Kind:     token.INT,
				Value:    fmt.Sprint(idx),
			},
			cond,
		},
		Rparen: gen.pos,
	}
}

func (gen codeGenerator) define(lhs string, rhs ast.Expr) *ast.AssignStmt {
	return gen.defineExprs(lhs, []ast.Expr{rhs})
}

func (gen codeGenerator) defineExprs(lhs string, rhs []ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{
		Lhs:    []ast.Expr{gen.ident(lhs)},
		TokPos: gen.pos,
		Tok:    token.DEFINE,
		Rhs:    rhs,
	}
}

// defineIsType assigns to lhs whether rhs has the given type.
func (gen codeGenerator) defineIsType(lhs string, rhs string, typ ast.Expr) ast.Stmt {
	if isNilIdent(typ) {
		return gen.define(lhs, gen.eql(rhs, gen.ident("nil")))
	}
	return &ast.AssignStmt{
		Lhs:    []ast.Expr{gen.ident("_"), gen.ident(lhs)},
		TokPos: gen.pos,
		Tok:    token.DEFINE,
		Rhs: []ast.Expr{
			&ast.TypeAssertExpr{
				X:      gen.ident(rhs),
				Lparen: gen.pos,
				Type:   typ,
				Rparen: gen.pos,
			},
		},
	}
}

func (gen codeGenerator) use(rhs ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{
		Lhs:    []ast.Expr{gen.ident("_")},
		TokPos: gen.pos,
		Tok:    token.ASSIGN,
		Rhs:    []ast.Expr{rhs},
	}
}

func (gen codeGenerator) block(stmts []ast.Stmt) *ast.BlockStmt {
	return &ast.BlockStmt{
		Lbrace: gen.pos,
		List:   stmts,
		Rbrace: gen.pos,
	}
}

func (gen codeGenerator) switchStmt(init ast.Stmt, body *ast.BlockStmt) *ast.SwitchStmt {
	return &ast.SwitchStmt{
		Switch: gen.pos,
		Init:   init,
		Tag:    nil,
		Body:   body,
	}
}

func (gen codeGenerator) caseClause(list []ast.Expr, body []ast.Stmt) *ast.CaseClause {
	return &ast.CaseClause{
		Case:  gen.pos,
		List:  list,
		Colon: gen.pos,
		Body:  body,
	}
}

// reposition returns a deep copy of e in which all token positions have been
// replaced with the code generator's position.
func (gen codeGenerator) reposition(e ast.Expr) ast.Expr {
	return subst(reflect.ValueOf(e), gen.reset).Interface().(ast.Expr)
}

func (gen codeGenerator) reset(x reflect.Value) reflect.Value {
	switch x.Interface().(type) {
	case *ast.Object, *ast.Scope:
		return reflect.Zero(x.Type())
	case token.Pos:
		return reflect.ValueOf(gen.pos)
	}
	return x
}

func subst(
	rx reflect.Value,
	pre func(reflect.Value) reflect.Value,
) reflect.Value {
	x := pre(rx)
	switch x.Kind() {

	case reflect.Interface:
		lv := reflect.New(x.Type()).Elem()
		if rv := x.Elem(); rv.IsValid() {
			lv.Set(subst(rv, pre))
		}
		return lv

	case reflect.Ptr:
		lv := reflect.New(x.Type()).Elem()
		if rv := x.Elem(); rv.IsValid() {
			lv.Set((subst(rv, pre)).Addr())
		}
		return lv

	case reflect.Slice:
		if x.IsNil() {
			return reflect.Zero(x.Type())
		}
		c := reflect.MakeSlice(x.Type(), x.Len(), x.Cap())
		for i := 0; i < x.Len(); i++ {
			c.Index(i).Set(subst(x.Index(i), pre))
		}
		return c

	case reflect.Struct:
		c := reflect.New(x.Type()).Elem()
		for i := 0; i < x.NumField(); i++ {
			c.Field(i).Set(subst(x.Field(i), pre))
		}
		return c

	default:
		// Assume that all other types can be copied trivially.
		c := reflect.New(x.Type()).Elem()
		c.Set(x)
		return c
	}
}

// sortedPkgs returns 'package x' first, followed by 'package x_test'.
func sortedPkgs(m map[string]*ast.Package) []*ast.Package {
	var pkgs []*ast.Package
	for _, pkg := range m {
		pkgs = append(pkgs, pkg)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].Name < pkgs[j].Name
	})
	return pkgs
}

func forEachFile(pkg *ast.Package, action func(string, *ast.File)) {
	var fileNames []string
	for fileName := range pkg.Files {
		fileNames = append(fileNames, fileName)
	}
	// Sort files, for deterministic output.
	sort.Strings(fileNames)

	for _, fileName := range fileNames {
		action(fileName, pkg.Files[fileName])
	}
}

func isNilIdent(e ast.Expr) bool {
again:
	if p, ok := e.(*ast.ParenExpr); ok {
		e = p.X
		goto again
	}
	ident, ok := e.(*ast.Ident)
	return ok && ident.Name == "nil"
}

func shouldBuild(filename string) bool {
	ctx := build.Context{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	m, err := ctx.MatchFile(filepath.Split(filename))
	ok(err)
	return m
}

func writeFile(filename string, content string) {
	ok(os.WriteFile(filename, []byte(content), 0o666))
}
