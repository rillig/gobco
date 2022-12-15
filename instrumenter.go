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
	"strings"
)

// cond is a condition that appears somewhere in the source code.
type cond struct {
	start string // human-readable position in the file, e.g. "main.go:17:13"
	code  string // the source code of the condition
}

type wrapCondAction struct {
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
	exprAction  map[ast.Expr]*wrapCondAction
	stmtRef     map[ast.Stmt]*ast.Stmt
	stmtGen     map[ast.Stmt]func() ast.Stmt

	text    string // during instrumentFile(), the text of the current file
	varname int    // to produce unique local variable names
}

// instrument modifies the code of the Go package in srcDir by adding counters
// for code coverage, writing the instrumented code to dstDir.
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
		for filename, file := range pkg.Files {
			i.instrumentFile(filename, file, dstDir)
		}
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
// To avoid wrapping complex conditions redundantly, these are unmarked.
// For example, after the whole file is visited,
// in a condition 'a && !c', only 'a' and 'c' are marked, but not '!' or '&&'.
func (i *instrumenter) markConds(n ast.Node) bool {
	// The order of the cases matches the order in ast.Walk.
	switch n := n.(type) {

	case *ast.ParenExpr:
		if i.marked[n] {
			i.marked[n.X] = true
			delete(i.marked, n)
		}

	case *ast.UnaryExpr:
		if n.Op == token.NOT {
			i.marked[n.X] = true
			delete(i.marked, n)
		}

	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			i.marked[n.X] = true
			i.marked[n.Y] = true
			delete(i.marked, n)
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
						i.exprAction[expr] = &wrapCondAction{
							ref, expr, expr.Pos(), i.str(expr),
						}
					}

				case []ast.Expr:
					for ei, expr := range val {
						ref, expr := &val[ei], expr
						if i.marked[expr] {
							delete(i.marked, expr)
							i.exprAction[expr] = &wrapCondAction{
								ref, expr, expr.Pos(), i.str(expr),
							}
						}
					}

				case ast.Stmt:
					if field.Type() == reflect.TypeOf((*ast.Stmt)(nil)).Elem() {
						i.stmtRef[val] = field.Addr().Interface().(*ast.Stmt)
					}

				case []ast.Stmt:
					for si, stmt := range val {
						ref, stmt := &val[si], stmt
						i.stmtRef[stmt] = ref
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

	// In a switch statement with an expression, the expression is
	// evaluated once and is then compared to each expression from the
	// case clauses. But first, the initialization statement needs to be
	// executed.
	//
	// In the instrumented switch statement, the tag expression always has
	// boolean type, and the expressions in the case clauses are instrumented
	// to calls to 'gobcoCover(id, tag == expr)'.
	tagExprName := i.nextVarname()
	tagExprUsed := false

	// Convert each expression from the 'case' clauses to an expression of
	// the form 'gobcoCover(id, tag == expr)'.
	for _, clause := range n.Body.List {
		clause := clause.(*ast.CaseClause)
		for j, expr := range clause.List {
			ref := &clause.List[j]
			eq := &ast.BinaryExpr{
				X:  ast.NewIdent(tagExprName),
				Op: token.EQL,
				Y:  expr,
			}
			pos := expr.Pos()
			eqlStr := i.strEql(n.Tag, expr)
			i.exprAction[expr] = &wrapCondAction{ref, eq, pos, eqlStr}
			tagExprUsed = true
		}
	}

	latePatchDst := -1
	var newBody []ast.Stmt
	if n.Init != nil {
		newBody = append(newBody, n.Init)
	}
	latePatchDst = len(newBody)
	newBody = append(newBody,
		&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(tagExprName)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{n.Tag},
		},
	)
	if !tagExprUsed {
		newBody = append(newBody,
			&ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent("_")},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{ast.NewIdent(tagExprName)},
			},
		)
	}
	newBody = append(newBody,
		&ast.SwitchStmt{
			Body: &ast.BlockStmt{List: n.Body.List},
		},
	)

	// The initialization statements are executed in a new scope,
	// so convert the existing 'switch' statement to an empty one,
	// just to have this scope.
	//
	// The same scope is used for storing the tag expression in a
	// variable, as the variable names don't overlap.
	i.stmtGen[n] = func() ast.Stmt {
		return &ast.SwitchStmt{
			Switch: n.Switch,
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.CaseClause{
						Body: newBody,
					},
				},
			},
		}
	}

	// n.Tag is the only expression node whose reference is not preserved
	// in the instrumented tree, so update it.
	if a := i.exprAction[n.Tag]; a != nil {
		a.ref = &newBody[latePatchDst].(*ast.AssignStmt).Rhs[0]
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

	// evaluatedTagExpr := switch.tagExpr
	evaluatedTagExpr := i.nextVarname()
	newBody = append(newBody, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(evaluatedTagExpr)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{tagExpr.X},
	})
	newBody = append(newBody, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("_")},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent(evaluatedTagExpr)},
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
						&ast.BinaryExpr{
							X:  ast.NewIdent(evaluatedTagExpr),
							Op: token.EQL,
							Y:  ast.NewIdent("nil"),
						},
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
							X:    ast.NewIdent(evaluatedTagExpr),
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
				// tagExprName := evaluatedTagExpr.(singleType)
				newBody = append(newBody, &ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(tagExprName)},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{&ast.TypeAssertExpr{
						X:    ast.NewIdent(evaluatedTagExpr),
						Type: singleType,
					}},
				})
			} else {
				// tagExprName := evaluatedTagExpr
				newBody = append(newBody, &ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(tagExprName)},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{ast.NewIdent(evaluatedTagExpr)},
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
			newList = append(newList, wrapped)
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

// replace wraps each marked node with the instrumentation code,
// in declaration order.
func (i *instrumenter) replace(n ast.Node) bool {
	switch n := n.(type) {

	case ast.Expr:
		if a := i.exprAction[n]; a != nil {
			*a.ref = i.wrapText(a.expr, a.pos, a.text)
		}

	case ast.Stmt:
		if gen, ok := i.stmtGen[n]; ok {
			*i.stmtRef[n] = gen()
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
	varname := fmt.Sprintf("gobco%d", i.varname)
	i.varname++
	return varname
}
