package main

import (
	"os"
	"testing"
)

func Test_gobco_parseCommandLine_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()

	g.parseCommandLine([]string{"gobco", "-want-c1"})
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
		wantC1:    true,
	}})
}

func Test_gobco_parseCommandLine__keep_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()

	g.parseCommandLine([]string{"gobco", "-keep", "-want-c1"})

	s.CheckEquals(g.exitCode, 0)
	s.CheckEquals(g.keep, true)
}

func Test_gobco_parseCommandLine__two_packages_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	var g gobco

	s.CheckPanics(
		func() { g.parseCommandLine([]string{"gobco", "-want-c1", "pkg1", "pkg2"}) },
		"checking multiple packages doesn't work yet")
}

func Test_gobco_instrument_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()
	g.parseCommandLine([]string{"gobco", "-want-c1", "testdata/failing"})
	g.prepareTmp()

	g.instrument()

	instrDst := g.file(g.args[0].instrDir)
	s.CheckEquals(listRegularFiles(instrDst), []string{
		"fail.go",
		"fail_test.go",
		"gobco_fixed.go",
		"gobco_no_testmain_test.go",
		"gobco_variable.go",
		"random.go"})

	g.cleanUp()
}

// Instrumenting a directory that doesn't contain a Go package does nothing.
func Test_gobco_instrument__empty_dir_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()
	g.parseCommandLine([]string{"gobco", "-want-c1", "testdata/deeply"})
	g.prepareTmp()

	g.instrument()

	instrDst := g.file(g.args[0].instrDir)
	s.CheckEquals(listRegularFiles(instrDst), []string{
		"nested/main.go",
	})

	g.cleanUp()
}

func Test_gobco_cleanup_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	g := s.newGobco()
	g.parseCommandLine([]string{"gobco", "-want-c1", "-verbose", "testdata/failing"})
	g.prepareTmp()

	g.instrument()

	instrDst := g.file(g.args[0].instrDir)
	s.CheckEquals(listRegularFiles(instrDst), []string{
		"fail.go",
		"fail_test.go",
		"gobco_fixed.go",
		"gobco_no_testmain_test.go",
		"gobco_variable.go",
		"random.go"})

	g.cleanUp()

	_, err := os.Stat(instrDst)
	s.CheckEquals(os.IsNotExist(err), true)

	_ = s.Stderr()
}

func Test_gobcoMain__test_fails_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	actualExitCode := gobcoMain(&s.out, &s.err, "gobco", "-want-c1", "-verbose", "testdata/failing")
	s.CheckEquals(actualExitCode, 1)

	stdout := s.Stdout()
	stderr := s.Stderr()

	s.CheckNotContains(stderr, "[build failed]")
	s.CheckContains(stdout, `Branch coverage: 5/6`)
}

func Test_gobcoMain__single_file_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	// "go test" returns 1 because one of the tests fails.
	stdout, stderr := s.RunMain(1, "gobco", "-want-c1", "-list-all", "testdata/failing/fail.go")

	s.CheckNotContains(stdout, "[build failed]")
	s.CheckNotContains(stderr, "[build failed]")
	s.CheckEquals(s.GobcoLines(stdout), []string{
		"Branch coverage: 5/6",
		"testdata/failing/fail.go:4:14: condition \"i < 10\" was 10 times true and once false",
		"testdata/failing/fail.go:7:6: condition \"a < 1000\" was 5 times true and once false",
		"testdata/failing/fail.go:10:5: condition \"Bar(a) == 10\" was once false but never true",
		// testdata/failing/random.go is not listed here
		// since that file is not mentioned in the command line.
	})
}

func Test_gobcoMain__multiple_files_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	// "go test" returns 1 because one of the tests fails.
	stdout, stderr := s.RunMain(1, "gobco", "-want-c1", "-list-all", "testdata/failing")

	s.CheckNotContains(stdout, "[build failed]")
	s.CheckNotContains(stderr, "[build failed]")
	// Ensure that the files in the output are sorted.
	s.CheckEquals(s.GobcoLines(stdout), []string{
		"Branch coverage: 5/6",
		"testdata/failing/fail.go:4:14: condition \"i < 10\" was 10 times true and once false",
		"testdata/failing/fail.go:7:6: condition \"a < 1000\" was 5 times true and once false",
		"testdata/failing/fail.go:10:5: condition \"Bar(a) == 10\" was once false but never true",
		// testdata/failing/random.go is not listed here
		// since no branches available to list.
	})
}

func Test_gobcoMain__TestMain_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	stdout, stderr := s.RunMain(0, "gobco", "-want-c1", "-verbose", "testdata/testmain")

	s.CheckNotContains(stdout, "[build failed]")
	s.CheckEquals(s.GobcoLines(stdout), []string{
		"Branch coverage: 0/0",
	})
	s.CheckContains(stdout, "begin original TestMain")
	s.CheckContains(stdout, "end original TestMain")
	_ = stderr
}

func Test_gobcoMain__oddeven_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	stdout, stderr := s.RunMain(0, "gobco", "-want-c1", "testdata/oddeven")

	s.CheckContains(stdout, "Branch coverage: 0/0")
	// No branches listed since nothing there to instrument

	// The condition in even_test.go is not instrumented since
	// gobco was not run with the '-cover-test' option.
	s.CheckEquals(stderr, "")
}

func Test_gobcoMain__blackBox_c1(t *testing.T) {
	s := NewSuite(t)
	defer s.TearDownTest()

	stdout, stderr := s.RunMain(0, "gobco", "-want-c1", "-cover-test", "testdata/pkgname")

	s.CheckEquals(s.GobcoLines(stdout), []string{
		"Branch coverage: 4/8",
		"testdata/pkgname/main.go:4:5: " +
			"condition \"cond\" was once true but never false",
		"testdata/pkgname/main.go:11:5: " +
			"condition \"cond\" was once true but never false",
		"testdata/pkgname/white_box_test.go:10:5: " +
			"condition \"unexported(true) != 'U'\" " +
			"was once false but never true",
		"testdata/pkgname/black_box_test.go:12:5: " +
			"condition \"pkgname.Exported(true) != 'E'\" " +
			"was once false but never true",
	})
	s.CheckEquals(stderr, "")
}
