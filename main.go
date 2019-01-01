package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type cond struct {
	start string // human-readable position in the file, e.g. main.go:17:13
	code  string // the source code of the condition
}

type instrumenter struct {
	fset  *token.FileSet
	text  string // during instrument(), the text of the current file
	conds []cond // the collected conditions from all files from fset
}

func (i *instrumenter) addCond(start, code string) int {
	i.conds = append(i.conds, cond{start, code})
	return len(i.conds) - 1
}

func (i *instrumenter) cover(cond ast.Expr) ast.Expr {
	start := i.fset.Position(cond.Pos())
	code := i.text[start.Offset:i.fset.Position(cond.End()).Offset]
	idx := i.addCond(start.String(), code)

	return &ast.CallExpr{
		Fun: ast.NewIdent("gobcoCover"),
		Args: []ast.Expr{
			&ast.BasicLit{Kind: token.INT, Value: fmt.Sprint(idx)},
			cond}}
}

func (i *instrumenter) visit(n ast.Node) bool {
	switch n := n.(type) {

	case *ast.IfStmt:
		n.Cond = i.cover(n.Cond)

	case *ast.ForStmt:
		if n.Cond != nil {
			n.Cond = i.cover(n.Cond)
		}

	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			n.X = i.cover(n.X)
			n.Y = i.cover(n.Y)
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

func (i *instrumenter) visitExprs(exprs []ast.Expr) {
	for idx, expr := range exprs {
		switch expr := expr.(type) {
		case *ast.BinaryExpr:
			if expr.Op.Precedence() == token.EQL.Precedence() {
				exprs[idx] = i.cover(expr)
			}
		}
	}
}

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
	check(err)

	for _, pkg := range pkgs {
		var filenames []string
		for filename := range pkg.Files {
			filenames = append(filenames, filename)
		}
		sort.Strings(filenames)

		for _, filename := range filenames {
			fileBytes, err := ioutil.ReadFile(filename)
			check(err)
			i.text = string(fileBytes)

			ast.Inspect(pkg.Files[filename], i.visit)

			err = os.Rename(filename, filename+".gobco.tmp")
			check(err)

			fd, err := os.Create(filename)
			check(err)
			err = printer.Fprint(fd, i.fset, pkg.Files[filename])
			check(err)
			err = fd.Close()
			check(err)
		}
	}

	for pkgname, pkg := range pkgs {
		for filename := range pkg.Files {
			i.writeGobcoTest(filepath.Join(filepath.Dir(filename), "gobco_test.go"), pkgname)
			return
		}
	}
}

func (i *instrumenter) writeGobcoTest(filename, pkgname string) {
	f, err := os.Create(filename)
	check(err)

	fmt.Fprintf(f, "package %s\n", pkgname)
	f.WriteString(`

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

type gobcoCond struct {
	start      string
	code       string
	trueCount  int
	falseCount int
}

func gobcoCover(idx int, cond bool) bool {
	if cond {
		gobcoConds[idx].trueCount++
	} else {
		gobcoConds[idx].falseCount++
	}
	return cond
}

func gobcoPrintCoverage() {
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
		}
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	exitCode := m.Run()
	gobcoPrintCoverage()
	os.Exit(exitCode)
}
`)

	fmt.Fprintln(f, `var gobcoConds = [...]gobcoCond{`)
	for _, cond := range i.conds {
		fmt.Fprintf(f, "\t{%q, %q, 0, 0},\n", cond.start, cond.code)
	}
	fmt.Fprintln(f, "}")

	err = f.Close()
	check(err)
}

// recover original files
//
// TODO: leave this cleanup procedure as optional with --work flag
func (i *instrumenter) cleanUp(arg string) {
	i.fset.Iterate(func(file *token.File) bool {
		filename := file.Name()
		err := os.Remove(filename)
		check(err)
		err = os.Rename(filename+".gobco.tmp", filename)
		check(err)

		return true
	})

	i.fset.Iterate(func(file *token.File) bool {
		err := os.Remove(filepath.Join(filepath.Dir(file.Name()), "gobco_test.go"))
		check(err)

		return false
	})
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {

	var opts []string // everything before the --
	var args []string // everything after the --

	i := 1
	if len(os.Args) > 1 && os.Args[1] != "" && os.Args[1][0] == '-' {
		for ; i < len(os.Args) && os.Args[i] != "--"; i++ {
			opts = append(opts, os.Args[i])
		}
		if i < len(os.Args) {
			i++
		}
	}
	args = os.Args[i:]

	if len(args) == 0 {
		args = []string{"."}
	}

	for _, arg := range args {
		cover(arg, opts)
	}
}

func cover(arg string, opts []string) {
	st, err := os.Stat(arg)
	isDir := err == nil && st.Mode().IsDir()

	// move original files to temporary and instrument the files
	instrumenter := &instrumenter{}
	instrumenter.instrument(arg, isDir)

	var goTestArgs []string
	goTestArgs = append(goTestArgs, "test")
	// The -v is necessary to produce any output at all.
	// Without it, most of the log output is suppressed.
	goTestArgs = append(goTestArgs, "-v")
	goTestArgs = append(goTestArgs, opts...)
	goTestArgs = append(goTestArgs, arg)

	goTest := exec.Command("go", goTestArgs...)
	goTest.Stdout = os.Stdout
	goTest.Stderr = os.Stderr
	goTest.Dir = arg

	if !isDir {
		goTest.Dir = filepath.Dir(goTest.Dir)
	}

	err = goTest.Run()
	if err != nil {
		log.Println(err)
	}

	instrumenter.cleanUp(arg)
}
