package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type options struct {
	firstTime bool
	listAll   bool
}

type gobco struct {
	firstTime bool
	listAll   bool
	exitCode  int
}

func (g *gobco) parseCommandLine(strings []string) {
	// TODO: parse arguments and options
}

func (g *gobco) prepareTmpEnv() {
	// TODO: create temporary GOPATH
	// TODO: copy each file or directory from the arguments to the GOPATH
	// TODO:   during copy, ignore everything from .gitignore,
	// TODO:   but include all *.go files (possibly generated)
}

func (g *gobco) instrument() {
	// TODO: instrument all packages and files
}

func (g *gobco) runGoTest() {
	// TODO: export GOPATH="$tmpdir$separator$GOPATH"
	// TODO: os.Chdir(gopath)
	// TODO: run go test, generating coverage data into temporary files
}

func (g *gobco) printOutput() {
	// TODO: print the data from the temporary files in a human-readable format
}

var exit = os.Exit

func mainNew(args []string) {
	var g gobco
	g.parseCommandLine(args)
	g.prepareTmpEnv()
	g.instrument()
	g.runGoTest()
	g.printOutput()
	exit(g.exitCode)
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

	var options options
	for _, arg := range args {
		if arg == "-list-all" {
			options.listAll = true
		} else if arg == "-first-time" {
			options.firstTime = true
		} else {
			cover(arg, opts, options)
		}
	}
}

func cover(arg string, opts []string, options options) {
	st, err := os.Stat(arg)
	isDir := err == nil && st.Mode().IsDir()

	// move original files to temporary and instrument the files
	instrumenter := instrumenter{options: options}
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
