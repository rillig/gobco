package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"path"
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
	fset    *token.FileSet
	text    string // during instrument(), the text of the current file
	conds   []cond // the collected conditions from all files from fset
	options options
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
// branch coverage. It writes the instrumented code back into the same files.
func (i *instrumenter) instrument(arg string, isDir bool) {
	i.fset = token.NewFileSet()

	dir := arg
	if !isDir {
		dir = filepath.Dir(dir)
	}

	isRelevant := func(info os.FileInfo) bool {
		if isDir {
			return !strings.HasSuffix(info.Name(), "_test.go")
		} else {
			return info.Name() == path.Base(filepath.ToSlash(arg))
		}
	}
	pkgs, err := parser.ParseDir(i.fset, dir, isRelevant, 0)
	i.check(err)

	for _, pkg := range pkgs {
		var filenames []string
		for filename := range pkg.Files {
			filenames = append(filenames, filename)
		}
		sort.Strings(filenames)

		for _, filename := range filenames {
			fileBytes, err := ioutil.ReadFile(filename)
			i.check(err)
			i.text = string(fileBytes)

			ast.Inspect(pkg.Files[filename], i.visit)

			// FIXME: Renaming files is not the job of the instrumenter.
			i.check(os.Rename(filename, filename+".gobco.tmp"))

			fd, err := os.Create(filename)
			i.check(err)
			i.check(printer.Fprint(fd, i.fset, pkg.Files[filename]))
			i.check(fd.Close())
		}
	}

	for pkgname, pkg := range pkgs {
		for filename := range pkg.Files {
			i.writeGobcoGo(filepath.Join(filepath.Dir(filename), "gobco.go"), pkgname)
			i.writeGobcoTestGo(filepath.Join(filepath.Dir(filename), "gobco_test.go"), pkgname)
			return
		}
	}
}

func (i *instrumenter) writeGobcoGo(filename, pkgname string) {
	f, err := os.Create(filename)
	i.check(err)

	// TODO: Instead of formatting the coverage data in gobcoPrintCoverage,
	// it should rather be written to a file in an easily readable format,
	// such as JSON or CSV.

	tmpl := `package @package@

import (
	"fmt"
	"os"
)

type gobcoCond struct {
	start      string
	code       string
	trueCount  int
	falseCount int
}

func gobcoCover(idx int, cond bool) bool {
	if cond {
		if @firstTime@ && gobcoConds[idx].trueCount == 0 {
			fmt.Fprintf(os.Stderr, "%s: condition %q is true for the first time.\n", gobcoConds[idx].start, gobcoConds[idx].code)
		}
		gobcoConds[idx].trueCount++
	} else {
		if @firstTime@ && gobcoConds[idx].falseCount == 0 {
			fmt.Fprintf(os.Stderr, "%s: condition %q is false for the first time.\n", gobcoConds[idx].start, gobcoConds[idx].code)
		}
		gobcoConds[idx].falseCount++
	}
	return cond
}

func gobcoPrintCoverage(listAll bool) {
	cnt := 0
	for _, c := range gobcoConds {
		if c.trueCount > 0 {
			cnt++
		}
		if c.falseCount > 0 {
			cnt++
		}
	}
	fmt.Printf("Branch coverage: %d/%d\n", cnt, len(gobcoConds)*2)

	for _, cond := range gobcoConds {
		switch {
		case cond.trueCount == 0 && cond.falseCount > 1:
			fmt.Printf("%s: condition %q was %d times false but never true\n", cond.start, cond.code, cond.falseCount)
		case cond.trueCount == 0 && cond.falseCount == 1:
			fmt.Printf("%s: condition %q was once false but never true\n", cond.start, cond.code)

		case cond.falseCount == 0 && cond.trueCount > 1:
			fmt.Printf("%s: condition %q was %d times true but never false\n", cond.start, cond.code, cond.trueCount)
		case cond.falseCount == 0 && cond.trueCount == 1:
			fmt.Printf("%s: condition %q was once true but never false\n", cond.start, cond.code)

		case cond.trueCount == 0 && cond.falseCount == 0:
			fmt.Printf("%s: condition %q was never evaluated\n", cond.start, cond.code)

		case listAll:
			fmt.Printf("%s: condition %q was %d times true and %d times false\n",
				cond.start, cond.code, cond.trueCount, cond.falseCount)
		}
	}
}
`

	strings.NewReplacer(
		"@package@", pkgname,
		"@firstTime@", fmt.Sprintf("%v", i.options.firstTime),
	).WriteString(f, tmpl)

	fmt.Fprintln(f, `var gobcoConds = [...]gobcoCond{`)
	for _, cond := range i.conds {
		fmt.Fprintf(f, "\t{%q, %q, 0, 0},\n", cond.start, cond.code)
	}
	fmt.Fprintln(f, "}")

	i.check(f.Close())
}

func (i *instrumenter) writeGobcoTestGo(filename, pkgname string) {
	f, err := os.Create(filename)
	i.check(err)

	tmpl := `package @package@

import (
	"flag"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	flag.Parse()
	exitCode := m.Run()
	gobcoPrintCoverage(@listAll@)
	os.Exit(exitCode)
}
`
	strings.NewReplacer(
		"@package@", pkgname,
		"@listAll@", fmt.Sprintf("%v", i.options.listAll),
	).WriteString(f, tmpl)

	i.check(f.Close())
}

// recover original files
//
// TODO: leave this cleanup procedure as optional with --work flag
func (i *instrumenter) cleanUp(arg string) {
	i.fset.Iterate(func(file *token.File) bool {
		filename := file.Name()
		i.check(os.Remove(filename))
		i.check(os.Rename(filename+".gobco.tmp", filename))

		return true
	})

	i.fset.Iterate(func(file *token.File) bool {
		dir := filepath.Dir(file.Name())
		i.check(os.Remove(filepath.Join(dir, "gobco.go")))
		i.check(os.Remove(filepath.Join(dir, "gobco_test.go")))
		return false
	})
}

func (i *instrumenter) check(e error) {
	if e != nil {
		panic(e)
	}
}
