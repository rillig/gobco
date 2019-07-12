package main

import (
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type options struct {
	firstTime bool
	listAll   bool
}

type gobco struct {
	firstTime bool
	listAll   bool
	keep      bool

	goTestOpts []string
	// The files or directories to cover, relative to the current directory.
	srcItems []string
	// The files or directories to cover, relative to the temporary GOPATH.
	tmpItems []string

	tmpdir string

	exitCode int
}

func (g *gobco) parseCommandLine(osArgs []string) {
	var opts []string // everything before the --
	var args []string // everything after the --

	if len(osArgs) > 1 && strings.HasSuffix(osArgs[1], "-help") {
		fmt.Printf("usage: %s [options for go test] -- [-list-all] [-first-time] [-keep] package...", filepath.Base(osArgs[0]))
		os.Exit(0)
	}

	i := 1
	if len(osArgs) > 1 && osArgs[1] != "" && osArgs[1][0] == '-' {
		for ; i < len(osArgs) && osArgs[i] != "--"; i++ {
			opts = append(opts, osArgs[i])
		}
		if i < len(osArgs) {
			i++
		}
	}
	args = osArgs[i:]

	var items []string
	for _, arg := range args {
		switch arg {
		case "-list-all":
			g.listAll = true
		case "-first-time":
			g.firstTime = true
		case "-keep":
			g.keep = true

		default:
			items = append(items, arg)
		}
	}

	if len(items) == 0 {
		items = []string{"."}
	}

	for _, item := range items {
		g.srcItems = append(g.srcItems, item)
		g.tmpItems = append(g.tmpItems, g.rel(item))
	}

	g.goTestOpts = opts
}

// rel returns the path of the argument, relative to the current GOPATH,
// using forward slashes.
func (g *gobco) rel(arg string) string {
	base := strings.Split(os.Getenv("GOPATH"), string(filepath.ListSeparator))[0]
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		base = filepath.Join(home, "go")
	}

	abs, err := filepath.Abs(arg)
	if err != nil {
		log.Fatal(err)
	}

	rel, err := filepath.Rel(base, abs)
	if err != nil {
		log.Fatal(err)
	}

	if strings.HasPrefix(rel, "..") {
		log.Fatalf("argument %q (%q) must be inside %q", arg, rel, base)
	}

	return filepath.ToSlash(rel)
}

func (g *gobco) prepareTmpEnv() {
	base := os.TempDir()
	tmpdir, err := uuid.NewRandom()
	if err != nil {
		log.Fatal(err)
	}

	g.tmpdir = filepath.Join(base, "gobco-"+tmpdir.String())

	err = os.MkdirAll(g.tmpdir, 0777)
	if err != nil {
		log.Fatal(err)
	}

	for i, srcItem := range g.srcItems {
		tmpItem := g.tmpItems[i]

		info, err := os.Stat(srcItem)
		isDir := err == nil && info.IsDir()

		// TODO: Research how "package/..." is handled by other go commands.
		if isDir {
			infos, err := ioutil.ReadDir(srcItem)
			if err != nil {
				log.Fatal(err)
			}

			err = os.MkdirAll(filepath.Join(g.tmpdir, tmpItem), 0777)
			if err != nil {
				log.Fatal(err)
			}

			for _, info := range infos {
				name := info.Name()
				relevant := strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "*.s")
				if !relevant {
					continue
				}

				err := copyFile(
					filepath.Join(srcItem, info.Name()),
					filepath.Join(g.tmpdir, tmpItem, info.Name()))
				if err != nil {
					log.Fatal(err)
				}
			}
		} else {
			srcFile := srcItem
			dstFile := filepath.Join(g.tmpdir, tmpItem)

			err = os.MkdirAll(filepath.Dir(dstFile), 0777)
			if err != nil {
				log.Fatal(err)
			}

			err = copyFile(srcFile, dstFile)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func (g *gobco) instrument() {
	instrumenter := instrumenter{options: options{g.firstTime, g.listAll}}

	for i, srcItem := range g.srcItems {
		st, err := os.Stat(srcItem)
		isDir := err == nil && st.Mode().IsDir()

		instrumenter.instrument(srcItem, filepath.Join(g.tmpdir, g.tmpItems[i]), isDir)
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

	err := goTest.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			g.exitCode = exitErr.ExitCode()
		}
		log.Println(err)
	}

	// TODO: Make the instrumenter generate a JSON file instead of printing
	//  printing the output directly.
}

func (g *gobco) cleanUp() {
	if !g.keep {
		_ = os.RemoveAll(g.tmpdir)
	}
}

func (g *gobco) printOutput() {
	// TODO: print the data from the temporary file in a human-readable format.
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
