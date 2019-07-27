package main

import (
	"flag"
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const version = "0.9.0"

type gobco struct {
	firstTime bool
	listAll   bool
	keep      bool
	verbose   bool
	version   bool

	goTestOpts []string
	// The files or directories to cover, relative to the current directory.
	srcItems []string
	// The files or directories to cover, relative to the temporary GOPATH.
	tmpItems []string

	tmpdir string

	exitCode int
}

func (g *gobco) parseCommandLine(args []string) {
	flags := flag.NewFlagSet(filepath.Base(args[0]), flag.ExitOnError)
	flags.BoolVar(&g.firstTime, "first-time", false, "print each condition to stderr when it is reached the first time")
	help := flags.Bool("help", false, "print the available command line options")
	flags.BoolVar(&g.keep, "keep", false, "don't remove the temporary working directory")
	flags.BoolVar(&g.listAll, "list-all", false, "at finish, print also those conditions that are fully covered")
	flags.Var(newSliceFlag(&g.goTestOpts), "test", "pass a command line `option` to \"go test\", such as -vet=off")
	flags.BoolVar(&g.verbose, "verbose", false, "show progress messages")
	ver := flags.Bool("version", false, "print the gobco version")

	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "usage: %s [options] package...\n", flags.Name())
		flags.PrintDefaults()
	}

	err := flags.Parse(args[1:])
	g.check(err)

	if *help {
		flags.Usage()
		os.Exit(0)
	}
	if *ver {
		fmt.Println(version)
		os.Exit(0)
	}

	items := flags.Args()
	if len(items) == 0 {
		items = []string{"."}
	}

	for _, item := range items {
		g.srcItems = append(g.srcItems, item)
		g.tmpItems = append(g.tmpItems, g.rel(item))
	}

	if len(items) > 1 {
		panic("gobco: checking multiple packages doesn't work yet")
	}
}

// rel returns the path of the argument, relative to the current GOPATH,
// using forward slashes.
func (g *gobco) rel(arg string) string {
	base := strings.Split(os.Getenv("GOPATH"), string(filepath.ListSeparator))[0]
	if base == "" {
		home, err := userHomeDir()
		g.check(err)
		base = filepath.Join(home, "go")
	}

	abs, err := filepath.Abs(arg)
	g.check(err)

	rel, err := filepath.Rel(base, abs)
	g.check(err)

	if strings.HasPrefix(rel, "..") {
		g.check(fmt.Errorf("argument %q (%q) must be inside %q", arg, rel, base))
	}

	return filepath.ToSlash(rel)
}

func (g *gobco) prepareTmpEnv() {
	base := os.TempDir()
	tmpdir, err := uuid.NewRandom()
	g.check(err)

	g.tmpdir = filepath.Join(base, "gobco-"+tmpdir.String())

	err = os.MkdirAll(g.tmpdir, 0777)
	g.check(err)

	if g.verbose {
		log.Printf("The temporary working directory is %s", g.tmpdir)
	}

	for i, srcItem := range g.srcItems {
		tmpItem := g.tmpItems[i]

		info, err := os.Stat(srcItem)
		isDir := err == nil && info.IsDir()

		// TODO: Research how "package/..." is handled by other go commands.
		if isDir {
			g.prepareTmpDir(srcItem, tmpItem)
		} else {
			g.prepareTmpFile(srcItem, tmpItem)
		}
	}
}

func (g *gobco) prepareTmpDir(srcItem string, tmpItem string) {
	infos, err := ioutil.ReadDir(srcItem)
	g.check(err)

	g.check(os.MkdirAll(filepath.Join(g.tmpdir, tmpItem), 0777))

	for _, info := range infos {
		name := info.Name()
		relevant := strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "*.s")
		if !relevant {
			continue
		}

		// The other *.go files are copied there by gobco.instrument().

		srcPath := filepath.Join(srcItem, info.Name())
		dstPath := filepath.Join(g.tmpdir, tmpItem, info.Name())
		g.check(copyFile(srcPath, dstPath))

		if g.verbose {
			log.Printf("Copied %s to %s", srcPath, filepath.Join(tmpItem, info.Name()))
		}
	}
}

func (g *gobco) prepareTmpFile(srcItem string, tmpItem string) {
	srcFile := srcItem
	dstFile := filepath.Join(g.tmpdir, tmpItem)

	g.check(os.MkdirAll(filepath.Dir(dstFile), 0777))
	g.check(copyFile(srcFile, dstFile))
}

func (g *gobco) instrument() {
	var instrumenter instrumenter
	instrumenter.firstTime = g.firstTime
	instrumenter.listAll = g.listAll

	for i, srcItem := range g.srcItems {
		st, err := os.Stat(srcItem)
		isDir := err == nil && st.Mode().IsDir()

		instrumenter.instrument(srcItem, filepath.Join(g.tmpdir, g.tmpItems[i]), isDir)

		if g.verbose {
			log.Printf("Instrumented %s to %s", srcItem, g.tmpItems[i])
		}
	}
}

func (g *gobco) runGoTest() {
	var args []string
	args = append(args, "test")
	// The -v is necessary to produce any output at all.
	// Without it, most of the log output is suppressed.
	args = append(args, "-v")
	args = append(args, g.goTestOpts...)
	for _, item := range g.tmpItems {
		args = append(args, strings.TrimPrefix(item, "src/"))
	}

	gopathEnv := fmt.Sprintf("GOPATH=%s%c%s", g.tmpdir, filepath.ListSeparator, os.Getenv("GOPATH"))

	goTest := exec.Command("go", args...)
	goTest.Stdout = os.Stdout
	goTest.Stderr = os.Stderr
	goTest.Dir = g.tmpdir
	goTest.Env = append(os.Environ(), gopathEnv)

	if g.verbose {
		log.Printf("Running %q in %q",
			strings.Join(append([]string{"go"}, args...), " "),
			goTest.Dir)
	}

	err := goTest.Run()
	if err != nil {
		g.exitCode = 1
		log.Println(err)
	}

	// TODO: Make the instrumenter generate a JSON file instead of printing
	//  printing the output directly.
}

func (g *gobco) cleanUp() {
	if g.keep {
		fmt.Fprintf(os.Stderr, "gobco: the temporary files are in %s", g.tmpdir)
	} else {
		_ = os.RemoveAll(g.tmpdir)
	}
}

func (g *gobco) printOutput() {
	// TODO: print the data from the temporary file in a human-readable format.
}

func (g *gobco) check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

var exit = os.Exit

func gobcoMain(args []string) {
	var g gobco
	g.parseCommandLine(args)
	g.prepareTmpEnv()
	g.instrument()
	g.runGoTest()
	g.cleanUp()
	g.printOutput()
	exit(g.exitCode)
}

func main() {
	gobcoMain(os.Args)
}
