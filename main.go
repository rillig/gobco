package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

const version = "0.9.1"

type gobco struct {
	firstTime   bool
	listAll     bool
	immediately bool
	keep        bool
	verbose     bool
	version     bool

	goTestOpts []string
	// The files or directories to cover, relative to the current directory.
	srcItems []string
	tmpItems []tmpItem

	tmpdir string

	statsFilename string

	stdout   io.Writer
	stderr   io.Writer
	exitCode int
}

// tmpItem is a file or directory to cover, relative to the temporary $GOPATH/src.
type tmpItem struct {
	rel   string // slash-separated
	isDir bool
}

func (ti *tmpItem) dir() string {
	if ti.isDir {
		return ti.rel
	}
	return path.Dir(ti.rel)
}

func newGobco(stdout io.Writer, stderr io.Writer) *gobco {
	return &gobco{stdout: stdout, stderr: stderr}
}

func (g *gobco) parseCommandLine(args []string) {
	flags := flag.NewFlagSet(filepath.Base(args[0]), flag.ContinueOnError)
	flags.BoolVar(&g.firstTime, "first-time", false, "print each condition to stderr when it is reached the first time")
	help := flags.Bool("help", false, "print the available command line options")
	flags.BoolVar(&g.immediately, "immediately", false, "persist the coverage immediately at each check point")
	flags.BoolVar(&g.keep, "keep", false, "don't remove the temporary working directory")
	flags.BoolVar(&g.listAll, "list-all", false, "at finish, print also those conditions that are fully covered")
	flags.StringVar(&g.statsFilename, "stats", "", "load and persist the JSON coverage data to this file")
	flags.Var(newSliceFlag(&g.goTestOpts), "test", "pass a command line `option` to \"go test\", such as -vet=off")
	flags.BoolVar(&g.verbose, "verbose", false, "show progress messages")
	ver := flags.Bool("version", false, "print the gobco version")

	flags.SetOutput(g.stderr)
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "usage: %s [options] package...\n", flags.Name())
		flags.PrintDefaults()
		g.exitCode = 2
	}

	err := flags.Parse(args[1:])
	if g.exitCode != 0 {
		exit(g.exitCode)
	}
	g.check(err)

	if *help {
		flags.SetOutput(g.stdout)
		flags.Usage()
		exit(0)
	}

	if *ver {
		fmt.Fprintln(g.stdout, version)
		exit(0)
	}

	items := flags.Args()
	if len(items) == 0 {
		items = []string{"."}
	}

	for _, item := range items {
		st, err := os.Stat(item)
		dir := err == nil && st.IsDir()

		g.srcItems = append(g.srcItems, item)
		g.tmpItems = append(g.tmpItems, tmpItem{g.rel(item), dir})
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

	slashRel := filepath.ToSlash(rel)
	return strings.TrimPrefix(slashRel, "src/")
}

func (g *gobco) prepareTmpEnv() {
	base := os.TempDir()
	tmpdir, err := uuid.NewRandom()
	g.check(err)

	g.tmpdir = filepath.Join(base, "gobco-"+tmpdir.String())
	if g.statsFilename != "" {
		g.statsFilename, err = filepath.Abs(g.statsFilename)
		g.check(err)
	} else {
		g.statsFilename = filepath.Join(g.tmpdir, "gobco-counts.json")
	}

	g.check(os.MkdirAll(g.tmpdir, 0777))

	g.verbosef("The temporary working directory is %s", g.tmpdir)

	// TODO: Research how "package/..." is handled by other go commands.
	for i, srcItem := range g.srcItems {
		g.prepareTmpDir(srcItem, g.tmpItems[i])
	}
}

func (g *gobco) prepareTmpDir(srcItem string, tmpItem tmpItem) {
	srcDir := srcItem
	if !tmpItem.isDir {
		srcDir = filepath.Dir(srcDir)
	}

	dstDir := tmpItem.dir()
	g.check(os.MkdirAll(g.tmpSrc(dstDir), 0777))

	infos, err := ioutil.ReadDir(srcDir)
	g.check(err)

	for _, info := range infos {
		name := info.Name()
		if !strings.HasSuffix(name, "_test.go") {
			continue
		}

		// The other *.go files are copied there by gobco.instrument().

		srcPath := filepath.Join(srcDir, name)
		dstPath := g.tmpSrc(dstDir, name)
		g.check(copyFile(srcPath, dstPath))

		g.verbosef("Copied %s to %s", srcPath, path.Join(dstDir, name))
	}
}

func (g *gobco) instrument() {
	var instrumenter instrumenter
	instrumenter.firstTime = g.firstTime
	instrumenter.immediately = g.immediately
	instrumenter.listAll = g.listAll

	for i, srcItem := range g.srcItems {
		isDir := g.tmpItems[i].isDir

		instrumenter.instrument(srcItem, g.tmpSrc(g.tmpItems[i].rel), isDir)

		g.verbosef("Instrumented %s to %s", srcItem, g.tmpItems[i].rel)
	}
}

func (g *gobco) runGoTest() {

	args := g.goTestArgs()

	goTest := exec.Command("go", args[1:]...)
	goTest.Stdout = g.stdout
	goTest.Stderr = g.stderr
	goTest.Dir = filepath.Join(g.tmpdir, "src")
	goTest.Env = g.goTestEnv()

	g.verbosef("Running %q in %q",
		strings.Join(args, " "),
		goTest.Dir)

	err := goTest.Run()
	if err != nil {
		g.exitCode = 1
		fmt.Fprintf(g.stderr, "%s\n", err)
	}
}

func (g *gobco) goTestArgs() []string {
	var args []string
	args = append(args, "go")
	args = append(args, "test")

	// The -v is necessary to produce any output at all.
	// Without it, most of the log output is suppressed.
	args = append(args, "-v")

	args = append(args, g.goTestOpts...)

	seen := make(map[string]bool)
	for _, item := range g.tmpItems {
		arg := item.rel
		if !item.isDir {
			arg = path.Dir(arg)
		}

		if !seen[arg] {
			args = append(args, arg)
			seen[arg] = true
		}
	}

	return args
}

func (g *gobco) goTestEnv() []string {
	gopath := fmt.Sprintf("%s%c%s", g.tmpdir, filepath.ListSeparator, os.Getenv("GOPATH"))

	var env []string
	env = append(env, os.Environ()...)
	env = append(env, "GOPATH="+gopath)
	env = append(env, "GOBCO_STATS="+g.statsFilename)

	return env
}
func (g *gobco) cleanUp() {
	if g.keep {
		g.verbosef("the temporary files are in %s", g.tmpdir)
	} else {
		_ = os.RemoveAll(g.tmpdir)
	}
}

func (g *gobco) printOutput() {
	conds := g.load(g.statsFilename)

	cnt := 0
	for _, c := range conds {
		if c.TrueCount > 0 {
			cnt++
		}
		if c.FalseCount > 0 {
			cnt++
		}
	}

	fmt.Fprintln(g.stdout)
	fmt.Fprintf(g.stdout, "Branch coverage: %d/%d\n", cnt, len(conds)*2)

	for _, cond := range conds {
		g.printCond(cond)
	}
}

func (g *gobco) load(filename string) []condition {
	file, err := os.Open(filename)
	g.check(err)

	defer func() {
		closeErr := file.Close()
		g.check(closeErr)
	}()

	var data []condition
	decoder := json.NewDecoder(bufio.NewReader(file))
	decoder.DisallowUnknownFields()
	g.check(decoder.Decode(&data))

	return data
}

func (g *gobco) printCond(cond condition) {
	switch {
	case cond.TrueCount == 0 && cond.FalseCount > 1:
		fmt.Fprintf(g.stdout, "%s: condition %q was %d times false but never true\n",
			cond.Start, cond.Code, cond.FalseCount)
	case cond.TrueCount == 0 && cond.FalseCount == 1:
		fmt.Fprintf(g.stdout, "%s: condition %q was once false but never true\n",
			cond.Start, cond.Code)

	case cond.FalseCount == 0 && cond.TrueCount > 1:
		fmt.Fprintf(g.stdout, "%s: condition %q was %d times true but never false\n",
			cond.Start, cond.Code, cond.TrueCount)
	case cond.FalseCount == 0 && cond.TrueCount == 1:
		fmt.Fprintf(g.stdout, "%s: condition %q was once true but never false\n",
			cond.Start, cond.Code)

	case cond.TrueCount == 0 && cond.FalseCount == 0:
		fmt.Fprintf(g.stdout, "%s: condition %q was never evaluated\n",
			cond.Start, cond.Code)

	case g.listAll:
		fmt.Fprintf(g.stdout, "%s: condition %q was %d times true and %d times false\n",
			cond.Start, cond.Code, cond.TrueCount, cond.FalseCount)
	}
}

func (g *gobco) tmpSrc(rel string, other ...string) string {
	return filepath.Join(append([]string{g.tmpdir, "src", rel}, other...)...)
}

func (g *gobco) verbosef(format string, args ...interface{}) {
	if g.verbose {
		fmt.Fprintf(g.stderr, format+"\n", args...)
	}
}

func (g *gobco) check(err error) {
	if err != nil {
		fmt.Fprintln(g.stderr, err)
		exit(1)
	}
}

type condition struct {
	Start      string
	Code       string
	TrueCount  int
	FalseCount int
}

var exit = os.Exit

func gobcoMain(args []string) {
	g := newGobco(os.Stdout, os.Stderr)
	g.parseCommandLine(args)
	g.prepareTmpEnv()
	g.instrument()
	g.runGoTest()
	g.printOutput()
	g.cleanUp()
	exit(g.exitCode)
}

func main() {
	gobcoMain(os.Args)
}
