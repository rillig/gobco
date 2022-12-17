package main

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
)

type Suite struct {
	t   *testing.T
	out bytes.Buffer
	err bytes.Buffer
}

func NewSuite(t *testing.T) *Suite {
	s := Suite{t: t}
	exit = func(code int) {
		panic(exited(code))
	}
	return &s
}

func (s *Suite) Stdout() string {
	defer s.out.Reset()
	return s.out.String()
}

func (s *Suite) Stderr() string {
	defer s.err.Reset()
	return s.err.String()
}

func (s *Suite) newGobco() *gobco {
	return newGobco(&s.out, &s.err)
}

func (s *Suite) TearDownTest() {

	if stdout := s.Stdout(); stdout != "" {
		s.t.Errorf("unchecked stdout %q", stdout)
	}

	if stderr := s.Stderr(); stderr != "" {
		s.t.Errorf("unchecked stderr %q", stderr)
	}

	exit = os.Exit
}

func (s *Suite) CheckContains(output, str string) {
	if !strings.Contains(output, str) {
		s.t.Errorf("expected %q in the output, got %q", str, output)
	}
}

func (s *Suite) CheckNotContains(output, str string) {
	if strings.Contains(output, str) {
		s.t.Errorf("expected %q to not appear in the output %q", str, output)
	}
}

func (s *Suite) CheckEquals(actual, expected interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		s.t.Errorf("expected %+#v, got %+#v", expected, actual)
	}
}

func (s *Suite) CheckPanics(action func(), expected interface{}) {
	ok := true
	defer func() {
		if !ok {
			s.t.Errorf("expected panic %+v, got no panic", expected)
			return
		}
		actual := recover()
		if actual != nil && reflect.DeepEqual(actual, expected) {
			return
		}
		s.t.Errorf("expected panic %+v, got panic %+v", expected, actual)
	}()
	action()
	ok = false
}

func (s *Suite) RunMain(expectedExitCode int, argv ...string) (stdout, stderr string) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	actualExitCode := gobcoMain(&outBuf, &errBuf, argv...)

	s.CheckEquals(actualExitCode, expectedExitCode)

	return outBuf.String(), errBuf.String()
}

// GobcoLines extracts and normalizes the relevant lines from the output of
// running gobco, see RunMain.
func (s *Suite) GobcoLines(stdout string) []string {
	relevant := stdout[strings.Index(stdout, "Branch coverage:"):]
	trimmed := strings.TrimRight(relevant, "\n")
	normalized := strings.Replace(trimmed, "\\", "/", -1)
	return strings.Split(normalized, "\n")
}

type exited int

func Test_gobco_parseCommandLine(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()

	g.parseCommandLine([]string{"gobco"})
	tmpModuleDir := g.args[0].copyDst

	s.CheckEquals(g.exitCode, 0)
	s.CheckEquals(g.listAll, false)
	s.CheckEquals(g.keep, false)
	s.CheckEquals(g.args, []argInfo{{
		arg:       ".",
		argDir:    ".",
		module:    true,
		copySrc:   ".",
		copyDst:   tmpModuleDir,
		instrFile: "",
		instrDir:  tmpModuleDir,
	}})
}

func Test_gobco_parseCommandLine__keep(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()

	g.parseCommandLine([]string{"gobco", "-keep"})
	tmpModuleDir := g.args[0].copyDst

	s.CheckEquals(g.exitCode, 0)
	s.CheckEquals(g.listAll, false)
	s.CheckEquals(g.keep, true)
	s.CheckEquals(g.args, []argInfo{{
		arg:       ".",
		argDir:    ".",
		module:    true,
		copySrc:   ".",
		copyDst:   tmpModuleDir,
		instrFile: "",
		instrDir:  tmpModuleDir,
	}})
}

func Test_gobco_parseCommandLine__go_test_options(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()

	g.parseCommandLine([]string{"gobco", "-test", "-vet=off", "-test", "help", "pkg"})
	tmpModuleDir := g.args[0].copyDst

	s.CheckEquals(g.exitCode, 0)
	s.CheckEquals(g.listAll, false)
	s.CheckEquals(g.goTestArgs, []string{"-vet=off", "help"})
	s.CheckEquals(g.args, []argInfo{{
		arg:       "pkg",
		argDir:    ".",
		module:    true,
		copySrc:   ".", // Since 'pkg' is not an (existing) directory.
		copyDst:   tmpModuleDir,
		instrFile: "pkg",
		instrDir:  tmpModuleDir,
	}})
}

func Test_gobco_parseCommandLine__two_packages(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	var g gobco

	s.CheckPanics(
		func() { g.parseCommandLine([]string{"gobco", "pkg1", "pkg2"}) },
		"gobco: checking multiple packages doesn't work yet")
}

func Test_gobco_parseCommandLine__usage(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()

	s.CheckPanics(
		func() { g.parseCommandLine([]string{"gobco", "-invalid"}) },
		exited(2))

	s.CheckEquals(s.Stdout(), "")
	s.CheckEquals(s.Stderr(), ""+
		"flag provided but not defined: -invalid\n"+
		"usage: gobco [options] package...\n"+
		"  -cover-test\n"+
		"    \tcover the test code as well\n"+
		"  -help\n"+
		"    \tprint the available command line options\n"+
		"  -immediately\n"+
		"    \tpersist the coverage immediately at each check point\n"+
		"  -keep\n"+
		"    \tdon't remove the temporary working directory\n"+
		"  -list-all\n"+
		"    \tat finish, print also those conditions that are fully covered\n"+
		"  -stats file\n"+
		"    \tload and persist the JSON coverage data to this file\n"+
		"  -test option\n"+
		"    \tpass the option to \"go test\", such as -vet=off\n"+
		"  -verbose\n"+
		"    \tshow progress messages\n"+
		"  -version\n"+
		"    \tprint the gobco version\n")
}

func Test_gobco_parseCommandLine__help(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	g := newGobco(&stdout, &stderr)

	s.CheckPanics(
		func() { g.parseCommandLine([]string{"gobco", "--help"}) },
		exited(0))

	s.CheckEquals(stdout.String(), ""+
		"usage: gobco [options] package...\n"+
		"  -cover-test\n"+
		"    \tcover the test code as well\n"+
		"  -help\n"+
		"    \tprint the available command line options\n"+
		"  -immediately\n"+
		"    \tpersist the coverage immediately at each check point\n"+
		"  -keep\n"+
		"    \tdon't remove the temporary working directory\n"+
		"  -list-all\n"+
		"    \tat finish, print also those conditions that are fully covered\n"+
		"  -stats file\n"+
		"    \tload and persist the JSON coverage data to this file\n"+
		"  -test option\n"+
		"    \tpass the option to \"go test\", such as -vet=off\n"+
		"  -verbose\n"+
		"    \tshow progress messages\n"+
		"  -version\n"+
		"    \tprint the gobco version\n")
	s.CheckEquals(stderr.String(), "")
}

func Test_gobco_parseCommandLine__version(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()

	s.CheckPanics(
		func() { g.parseCommandLine([]string{"gobco", "--version"}) },
		exited(0))

	s.CheckEquals(s.Stdout(), version+"\n")
	s.CheckEquals(s.Stderr(), "")
}

func Test_gobco_prepareTmp(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()

	if g.tmpdir == "" {
		s.t.Errorf("expected tmpdir to be set")
	}
}

func Test_gobco_instrument(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()
	g.parseCommandLine([]string{"gobco", "testdata/sample"})
	g.prepareTmp()

	g.instrument()

	instrDst := g.file(g.args[0].instrDir)
	s.CheckEquals(listRegularFiles(instrDst), []string{
		"foo.go",
		"foo_test.go",
		"gobco_fixed.go",
		"gobco_no_testmain_test.go",
		"gobco_variable.go",
		"random.go"})

	g.cleanUp()
}

func Test_gobco_printCond(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer

	g := newGobco(&out, &err)

	g.printCond(condition{"location", "zero-zero", 0, 0})
	g.printCond(condition{"location", "zero-once", 0, 1})
	g.printCond(condition{"location", "zero-many", 0, 5})
	g.printCond(condition{"location", "once-zero", 1, 0})
	g.printCond(condition{"location", "once-once", 1, 1})
	g.printCond(condition{"location", "once-many", 1, 5})
	g.printCond(condition{"location", "many-zero", 5, 0})
	g.printCond(condition{"location", "many-once", 5, 1})
	g.printCond(condition{"location", "many-many", 5, 5})

	expectedOut := "" +
		"location: condition \"zero-zero\" was never evaluated\n" +
		"location: condition \"zero-once\" was once false but never true\n" +
		"location: condition \"zero-many\" was 5 times false but never true\n" +
		"location: condition \"once-zero\" was once true but never false\n" +
		"location: condition \"many-zero\" was 5 times true but never false\n"
	if stdout := out.String(); stdout != expectedOut {
		t.Errorf("unexpected stdout %q", stdout)
	}
	if stderr := err.String(); stderr != "" {
		t.Errorf("unexpected stderr %q", stderr)
	}
}

func Test_gobco_printCond__listAll(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer

	g := newGobco(&out, &err)

	g.listAll = true
	g.printCond(condition{"location", "zero-zero", 0, 0})
	g.printCond(condition{"location", "zero-once", 0, 1})
	g.printCond(condition{"location", "zero-many", 0, 5})
	g.printCond(condition{"location", "once-zero", 1, 0})
	g.printCond(condition{"location", "once-once", 1, 1})
	g.printCond(condition{"location", "once-many", 1, 5})
	g.printCond(condition{"location", "many-zero", 5, 0})
	g.printCond(condition{"location", "many-once", 5, 1})
	g.printCond(condition{"location", "many-many", 5, 5})

	expectedOut := "" +
		"location: condition \"zero-zero\" was never evaluated\n" +
		"location: condition \"zero-once\" was once false but never true\n" +
		"location: condition \"zero-many\" was 5 times false but never true\n" +
		"location: condition \"once-zero\" was once true but never false\n" +
		"location: condition \"once-once\" was once true and once false\n" +
		"location: condition \"once-many\" was once true and 5 times false\n" +
		"location: condition \"many-zero\" was 5 times true but never false\n" +
		"location: condition \"many-once\" was 5 times true and once false\n" +
		"location: condition \"many-many\" was 5 times true and 5 times false\n"
	if stdout := out.String(); stdout != expectedOut {
		t.Errorf("unexpected stdout %q", stdout)
	}
	if stderr := err.String(); stderr != "" {
		t.Errorf("unexpected stderr %q", stderr)
	}
}

func Test_gobco_cleanup(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()
	g.parseCommandLine([]string{"gobco", "-verbose", "testdata/sample"})
	g.prepareTmp()

	g.instrument()

	instrDst := g.file(g.args[0].instrDir)
	s.CheckEquals(listRegularFiles(instrDst), []string{
		"foo.go",
		"foo_test.go",
		"gobco_fixed.go",
		"gobco_no_testmain_test.go",
		"gobco_variable.go",
		"random.go"})

	g.cleanUp()

	_, err := os.Stat(instrDst)
	s.CheckEquals(os.IsNotExist(err), true)

	_ = s.Stdout()
	_ = s.Stderr()
}

func Test_gobcoMain__test_fails(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	actualExitCode := gobcoMain(&s.out, &s.err, "gobco", "-verbose", "-keep", "testdata/sample")
	s.CheckEquals(actualExitCode, 1)

	stdout := s.Stdout()
	stderr := s.Stderr()

	if strings.Contains(stderr, "[build failed]") {
		s.t.Fatalf("build failed: %s", stderr)
	}

	s.CheckContains(stdout, `Branch coverage: 5/8`)
}

func Test_gobcoMain__single_file(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	// "go test" returns 1 because one of the sample tests fails.
	stdout, stderr := s.RunMain(1, "gobco", "-list-all", "testdata/sample/foo.go")

	s.CheckNotContains(stdout, "[build failed]")
	s.CheckNotContains(stderr, "[build failed]")
	// There is no condition for testdata/sample/random.go since that file
	// is not mentioned in the command line.
	s.CheckEquals(s.GobcoLines(stdout), []string{
		"Branch coverage: 5/6",
		"testdata/sample/foo.go:4:14: condition \"i < 10\" was 10 times true and once false",
		"testdata/sample/foo.go:7:6: condition \"a < 1000\" was 5 times true and once false",
		"testdata/sample/foo.go:10:5: condition \"Bar(a) == 10\" was once false but never true",
	})
}

func Test_gobcoMain__multiple_files(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	// "go test" returns 1 because one of the sample tests fails.
	stdout, stderr := s.RunMain(1, "gobco", "-list-all", "testdata/sample")

	s.CheckNotContains(stdout, "[build failed]")
	s.CheckNotContains(stderr, "[build failed]")
	// Ensure that the files in the output are sorted.
	s.CheckEquals(s.GobcoLines(stdout), []string{
		"Branch coverage: 5/8",
		"testdata/sample/foo.go:4:14: condition \"i < 10\" was 10 times true and once false",
		"testdata/sample/foo.go:7:6: condition \"a < 1000\" was 5 times true and once false",
		"testdata/sample/foo.go:10:5: condition \"Bar(a) == 10\" was once false but never true",
		"testdata/sample/random.go:8:9: condition \"x == 4\" was never evaluated",
	})
}

func Test_gobcoMain__TestMain(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	stdout, stderr := s.RunMain(0, "gobco", "-verbose", "testdata/testmain")

	s.CheckNotContains(stdout, "[build failed]")
	s.CheckContains(stdout, "begin original TestMain")
	s.CheckContains(stdout, "end original TestMain")
	_ = stderr
}

func Test_gobcoMain__oddeven(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	stdout, stderr := s.RunMain(0, "gobco", "testdata/oddeven")

	s.CheckContains(stdout, "Branch coverage: 0/2")
	s.CheckContains(stdout, "odd.go:4:9: condition \"x%2 != 0\" was never evaluated")
	// The condition in even_test.go is not instrumented since
	// the main code is the test subject.
	s.CheckEquals(stderr, "")
}
