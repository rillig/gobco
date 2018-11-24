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

type branch struct {
	start string // human-readable position in the file, e.g. main.go:17:13
	code  string // the code for the condition of the branch
}

type instrumenter struct {
	fset     *token.FileSet
	text     string   // during instrument(), the text of the current file
	branches []branch // the collected branches from all files from fset
}

func (i *instrumenter) addBranch(start, code string) int {
	i.branches = append(i.branches, branch{start, code})
	return len(i.branches) - 1
}

func (i *instrumenter) newCounter(cond ast.Expr) *ast.CallExpr {
	start := i.fset.Position(cond.Pos())
	code := i.text[start.Offset:i.fset.Position(cond.End()).Offset]
	branchIdx := i.addBranch(start.String(), code)

	return &ast.CallExpr{
		Fun: ast.NewIdent("gobcoCover"),
		Args: []ast.Expr{
			cond,
			&ast.BasicLit{
				Kind:  token.INT,
				Value: fmt.Sprint(branchIdx),
			},
		},
	}
}

func (i *instrumenter) visit(n ast.Node) bool {
	switch x := n.(type) {
	// TODO: We need to handle ast.CaseClause if it is bool
	// TODO: We need to handle go routine related things
	// such as ast.SelectStmt
	case *ast.IfStmt:
		x.Cond = i.newCounter(x.Cond)
	case *ast.ForStmt:
		if x.Cond != nil {
			x.Cond = i.newCounter(x.Cond)
		}
	}
	return true
}

func (i *instrumenter) instrument(arg string, isDir bool) {
	// Create the AST by parsing src.
	i.fset = token.NewFileSet() // positions are relative to fset

	dir := arg
	if !isDir {
		dir = filepath.Dir(dir)
	}

	pkgs, err := parser.ParseDir(i.fset, dir, func(info os.FileInfo) bool {
		if isDir {
			return !strings.HasSuffix(info.Name(), "_test.go")
		} else {
			return info.Name() == path.Base(filepath.ToSlash(arg))
		}
	}, 0)
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

pkg:
	for pkgname, pkg := range pkgs {
		for filename := range pkg.Files {
			i.writeGobcoTest(filepath.Join(filepath.Dir(filename), "gobco_test.go"), pkgname)
			break pkg
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

type gobcoBranch struct {
	start      string
	code       string
	trueCount  int
	falseCount int
}

func gobcoCover(cond bool, idx int) bool {
	if cond {
		gobcoBranches[idx].trueCount++
	} else {
		gobcoBranches[idx].falseCount++
	}
	return cond
}

func gobcoPrintCoverage() {
	cnt := 0
	for _, c := range gobcoBranches {
		if c.trueCount > 0 {
			cnt++
		}
		if c.falseCount > 0 {
			cnt++
		}
	}
	fmt.Printf("Branch coverage: %d/%d\n", cnt, len(gobcoBranches)*2)

	for _, branch := range gobcoBranches {
		if branch.trueCount == 0 {
			fmt.Printf("%s: branch %q was never true\n", branch.start, branch.code)
		}
		if branch.falseCount == 0 {
			fmt.Printf("%s: branch %q was never false\n", branch.start, branch.code)
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

	fmt.Fprintln(f, `var gobcoBranches = [...]gobcoBranch{`)
	for _, branch := range i.branches {
		fmt.Fprintf(f, "\t{%q, %q, 0, 0},\n", branch.start, branch.code)
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

	// Parse the flag and get file or directory name
	arg := os.Args[1]
	st, err := os.Stat(arg)
	isDir := err == nil && st.Mode().IsDir()

	// move original files to temporary and instrument the files
	instrumenter := &instrumenter{}
	instrumenter.instrument(arg, isDir)

	// run go test
	goTest := exec.Command("go", "test", "-vet=off")
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
