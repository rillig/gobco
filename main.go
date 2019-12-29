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

const version = "0.9.4"

type gobco struct {
	firstTime   bool
	listAll     bool
	immediately bool
	keep        bool
	verbose     bool
	version     bool
	coverTest   bool

	goTestOpts []string
	args       []argument

	tmpdir string

	statsFilename string

	stdout   io.Writer
	stderr   io.Writer
	exitCode int
}

func newGobco(stdout io.Writer, stderr io.Writer) *gobco {
	return &gobco{stdout: stdout, stderr: stderr}
}

func (g *gobco) parseCommandLine(argv []string) {
	flags := flag.NewFlagSet(filepath.Base(argv[0]), flag.ContinueOnError)
	flags.BoolVar(&g.firstTime, "first-time", false, "print each condition to stderr when it is reached the first time")
	help := flags.Bool("help", false, "print the available command line options")
	flags.BoolVar(&g.immediately, "immediately", false, "persist the coverage immediately at each check point")
	flags.BoolVar(&g.keep, "keep", false, "don't remove the temporary working directory")
	flags.BoolVar(&g.listAll, "list-all", false, "at finish, print also those conditions that are fully covered")
	flags.StringVar(&g.statsFilename, "stats", "", "load and persist the JSON coverage data to this file")
	flags.Var(newSliceFlag(&g.goTestOpts), "test", "pass a command line `option` to \"go test\", such as -vet=off")
	flags.BoolVar(&g.verbose, "verbose", false, "show progress messages")
	flags.BoolVar(&g.coverTest, "cover-test", false, "cover the test code as well")
	ver := flags.Bool("version", false, "print the gobco version")

	flags.SetOutput(g.stderr)
	flags.Usage = func() {
		_, _ = fmt.Fprintf(flags.Output(), "usage: %s [options] package...\n", flags.Name())
		flags.PrintDefaults()
		g.exitCode = 2
	}

	err := flags.Parse(argv[1:])
	if g.exitCode != 0 {
		exit(g.exitCode)
	}
	g.ok(err)

	if *help {
		flags.SetOutput(g.stdout)
		flags.Usage()
		exit(0)
	}

	if *ver {
		g.outf("%s\n", version)
		exit(0)
	}

	args := flags.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	if len(args) > 1 {
		panic("gobco: checking multiple packages doesn't work yet")
	}

	for _, arg := range args {
		st, err := os.Stat(arg)
		dir := err == nil && st.IsDir()

		rel := g.rel(arg)
		g.args = append(g.args, argument{arg, rel, "", dir})
	}
}

// rel returns the path of the argument, relative to the current $GOPATH/src,
// using forward slashes.
func (g *gobco) rel(arg string) string {
	gopath := strings.Split(os.Getenv("GOPATH"), string(filepath.ListSeparator))[0]
	if gopath == "" {
		home, err := userHomeDir()
		g.ok(err)
		gopath = filepath.Join(home, "go")
	}

	abs, err := filepath.Abs(arg)
	g.ok(err)

	gopathSrc := filepath.Join(gopath, "src")
	rel, err := filepath.Rel(gopathSrc, abs)
	g.ok(err)

	if strings.HasPrefix(rel, "..") {
		g.ok(fmt.Errorf("argument %q must be inside %q", arg, gopathSrc))
	}

	return filepath.ToSlash(rel)
}

// prepareTmp copies the source files to the temporary directory.
//
// Some of these files will later be overwritten by gobco.instrumenter.
func (g *gobco) prepareTmp() {
	base := os.TempDir()
	tmpdir, err := uuid.NewRandom()
	g.ok(err)

	g.tmpdir = filepath.Join(base, "gobco-"+tmpdir.String())
	if g.statsFilename != "" {
		g.statsFilename, err = filepath.Abs(g.statsFilename)
		g.ok(err)
	} else {
		g.statsFilename = filepath.Join(g.tmpdir, "gobco-counts.json")
	}

	g.ok(os.MkdirAll(g.tmpdir, 0777))

	g.verbosef("The temporary working directory is %s", g.tmpdir)

	// TODO: Research how "package/..." is handled by other go commands.
	for i := range g.args {
		arg := &g.args[i]
		arg.absTmpFilename = filepath.Join(g.tmpdir, "src", filepath.FromSlash(arg.tmpName))

		g.prepareTmpDir(*arg)
	}
}

func (g *gobco) prepareTmpDir(arg argument) {
	srcDir := arg.argName
	if !arg.isDir {
		srcDir = filepath.Dir(srcDir)
	}

	dstDir := arg.absDir()
	g.ok(os.MkdirAll(dstDir, 0777))

	infos, err := ioutil.ReadDir(srcDir)
	g.ok(err)

	for _, info := range infos {
		if info.Mode().IsRegular() {
			name := info.Name()
			srcPath := filepath.Join(srcDir, name)
			dstPath := filepath.Join(dstDir, name)
			g.ok(copyFile(srcPath, dstPath))
		}
	}
}

func (g *gobco) instrument() {
	var instrumenter instrumenter
	instrumenter.firstTime = g.firstTime
	instrumenter.immediately = g.immediately
	instrumenter.listAll = g.listAll
	instrumenter.coverTest = g.coverTest

	for _, arg := range g.args {
		instrumenter.instrument(arg.argName, arg.absTmpFilename, arg.isDir)
		g.verbosef("Instrumented %s to %s", arg.argName, arg.tmpName)
	}
}

func (g *gobco) runGoTest() {

	args := g.goTestArgs()

	goTest := exec.Command("go", args[1:]...)
	goTest.Stdout = g.stdout
	goTest.Stderr = g.stderr
	goTest.Dir = filepath.Join(g.tmpdir, "src")
	goTest.Env = g.goTestEnv()

	cmdline := strings.Join(args, " ")
	g.verbosef("Running %q in %q", cmdline, goTest.Dir)

	err := goTest.Run()
	if err != nil {
		g.exitCode = 1
		g.errf("%s\n", err)
	} else {
		g.verbosef("Finished %s", cmdline)
	}
}

func (g *gobco) goTestArgs() []string {
	// The -v is necessary to produce any output at all.
	// Without it, most of the log output is suppressed.
	args := []string{"go", "test"}

	if g.verbose {
		args = append(args, "-v")
	}

	// Work around test result caching which does not apply anyway,
	// since the instrumented files are written to a new directory
	// each time.
	//
	// Without this option, "go test" sometimes needs twice the time.
	args = append(args, "-test.count", "1")

	seenDirs := make(map[string]bool)
	for _, arg := range g.args {
		dir := arg.dir()

		if !seenDirs[dir] {
			args = append(args, dir)
			seenDirs[dir] = true
		}
	}

	args = append(args, g.goTestOpts...)

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
		g.errf("\n")
		g.errf("the temporary files are in %s\n", g.tmpdir)
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

	g.outf("\n")
	g.outf("Branch coverage: %d/%d\n", cnt, len(conds)*2)

	for _, cond := range conds {
		g.printCond(cond)
	}
}

func (g *gobco) load(filename string) []condition {
	file, err := os.Open(filename)
	g.ok(err)

	defer func() {
		closeErr := file.Close()
		g.ok(closeErr)
	}()

	var data []condition
	decoder := json.NewDecoder(bufio.NewReader(file))
	decoder.DisallowUnknownFields()
	g.ok(decoder.Decode(&data))

	return data
}

func (g *gobco) printCond(cond condition) {

	trueCount := cond.TrueCount
	falseCount := cond.FalseCount
	start := cond.Start
	code := cond.Code

	if !g.listAll && trueCount > 0 && falseCount > 0 {
		return
	}

	capped := func(count int) int {
		if count > 1 {
			return 2
		}
		if count < 1 {
			return 0
		}
		return 1
	}

	switch 3*capped(trueCount) + capped(falseCount) {
	case 0:
		g.outf("%s: condition %q was never evaluated\n",
			start, code)
	case 1:
		g.outf("%s: condition %q was once false but never true\n",
			start, code)
	case 2:
		g.outf("%s: condition %q was %d times false but never true\n",
			start, code, falseCount)
	case 3:
		g.outf("%s: condition %q was once true but never false\n",
			start, code)
	case 4:
		g.outf("%s: condition %q was once true and once false\n",
			start, code)
	case 5:
		g.outf("%s: condition %q was once true and %d times false\n",
			start, code, falseCount)
	case 6:
		g.outf("%s: condition %q was %d times true but never false\n",
			start, code, trueCount)
	case 7:
		g.outf("%s: condition %q was %d times true and once false\n",
			start, code, trueCount)
	case 8:
		g.outf("%s: condition %q was %d times true and %d times false\n",
			start, code, trueCount, falseCount)
	}
}

func (g *gobco) outf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(g.stdout, format, args...)
}

func (g *gobco) errf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(g.stderr, format, args...)
}

func (g *gobco) verbosef(format string, args ...interface{}) {
	if g.verbose {
		g.errf(format+"\n", args...)
	}
}

func (g *gobco) ok(err error) {
	if err != nil {
		g.errf("%s\n", err)
		exit(1)
	}
}

type argument struct {
	// as given in the command line
	argName string

	// relative to the temporary $GOPATH/src
	tmpName        string
	absTmpFilename string

	isDir bool
}

func (a *argument) dir() string {
	if a.isDir {
		return a.tmpName
	}
	return path.Dir(a.tmpName)
}

func (a *argument) absDir() string {
	if a.isDir {
		return a.absTmpFilename
	}
	return filepath.Dir(a.absTmpFilename)
}

type condition struct {
	Start      string
	Code       string
	TrueCount  int
	FalseCount int
}

var exit = os.Exit

func gobcoMain(stdout, stderr io.Writer, args ...string) {
	g := newGobco(stdout, stderr)
	g.parseCommandLine(args)
	g.prepareTmp()
	g.instrument()
	g.runGoTest()
	g.printOutput()
	g.cleanUp()
	exit(g.exitCode)
}

func main() {
	gobcoMain(os.Stdout, os.Stderr, os.Args...)
}
