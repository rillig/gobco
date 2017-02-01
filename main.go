package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
)

var branchId int

func getBranchId () int {
	ret := branchId
	branchId++
	return ret
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func newCounter(cond ast.Expr) *ast.CallExpr {
		return &ast.CallExpr{
			Fun: ast.NewIdent("inst"),
			Args: []ast.Expr{
				cond,
				&ast.BasicLit{
					Kind:  token.INT,
					Value: fmt.Sprint(getBranchId()),
				},
			},
		}
}

func visit(n ast.Node) bool {
	switch x := n.(type) {
	// TODO: We need to handle ast.CaseCluase
	// TODO: We need to handle go routine related things
	// such as ast.SelectStmt
	case *ast.IfStmt:
		x.Cond = newCounter(x.Cond)
	case *ast.ForStmt:
		x.Cond = newCounter(x.Cond)
	}
	return true
}

func instrument(name string) {
	// Create the AST by parsing src.
	fset := token.NewFileSet() // positions are relative to fset
	f, err := parser.ParseFile(fset, name, nil, 0)
	if err != nil {
		panic(err)
	}
	fmtImport := &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: fmt.Sprintf("%q", "fmt"),
		},
	}
	f.Imports = append(f.Imports, fmtImport)
	f.Decls = append([]ast.Decl{&ast.GenDecl{
		Tok: token.IMPORT,
		Specs: []ast.Spec{
			fmtImport,
		},
	}}, f.Decls...)
	// Inspect the AST and print all identifiers and literals.
	ast.Inspect(f, visit)
	//ast.Print(fset, f)

	//fd := os.Stdout
	cmd := exec.Command("mv", name, name+".tmp")
	err = cmd.Run()
	check(err)
	fd, err := os.Create(name)
	check(err)
	printer.Fprint(fd, fset, f)
	fmt.Fprintf(fd, `
func inst(cond bool, idx int) bool {
  if cond {
    Cov.tCount[idx] = 1
  } else {
    Cov.fCount[idx] = 1
  }
  return cond
}

func PrintCoverage() {
  cnt := 0
  for _, c := range Cov.tCount {
    if c == 1 {
      cnt++
    }
  }
  for _, c := range Cov.fCount {
    if c == 1 {
      cnt++
    }
  }
  fmt.Println("Branch Coverage:", cnt, "/", len(Cov.tCount)*2)
}

var Cov = struct {
  tCount [%d]int
  fCount [%d]int
}{}
  `, branchId, branchId)
}

func main() {

	// Parse the flag and get file name

	// instrument
	instrument(os.Args[1])
	// move original file to temporary
	// run go test
	err := os.Chdir(filepath.Dir(os.Args[1]))
	check(err)
	out, _ := exec.Command("go", "test").Output()
	fmt.Printf("%s\n", out)

	// recover original file and clean up
	fname := filepath.Base(os.Args[1])
	cmd := exec.Command("rm", fname)
	err = cmd.Run()
	check(err)
	cmd = exec.Command("mv", fname+".tmp", fname)
	err = cmd.Run()
	check(err)
	// TODO: leave this cleanup procedure as optional with --work flag
}
