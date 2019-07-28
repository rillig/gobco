package main

import (
	"bytes"
	"gopkg.in/check.v1"
	"os"
	"strings"
)

func (s *Suite) Test_gobco_parseCommandLine(c *check.C) {
	var g gobco

	g.parseCommandLine([]string{"gobco"})

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.keep, check.Equals, false)
	c.Check(g.args, check.DeepEquals, []argument{
		{".", "github.com/rillig/gobco", "", true}})
}

func (s *Suite) Test_gobco_parseCommandLine__keep(c *check.C) {
	var g gobco

	g.parseCommandLine([]string{"gobco", "-keep"})

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.keep, check.Equals, true)
	c.Check(g.args, check.DeepEquals, []argument{
		{".", "github.com/rillig/gobco", "", true}})
}

func (s *Suite) Test_gobco_parseCommandLine__go_test_options(c *check.C) {
	var g gobco

	g.parseCommandLine([]string{"gobco", "-test", "-vet=off", "-test", "help", "pkg"})

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.goTestOpts, check.DeepEquals, []string{"-vet=off", "help"})
	c.Check(g.args, check.DeepEquals, []argument{
		{"pkg", "github.com/rillig/gobco/pkg", "", false}})
}

func (s *Suite) Test_gobco_parseCommandLine__two_packages(c *check.C) {
	var g gobco

	c.Check(
		func() { g.parseCommandLine([]string{"gobco", "pkg1", "pkg2"}) },
		check.Panics,
		"gobco: checking multiple packages doesn't work yet")
}

func (s *Suite) Test_gobco_parseCommandLine__usage(c *check.C) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	g := newGobco(&stdout, &stderr)

	c.Check(
		func() { g.parseCommandLine([]string{"gobco", "-invalid"}) },
		check.Panics,
		exited(2))

	c.Check(stdout.String(), check.Equals, "")
	c.Check(stderr.String(), check.Equals, ""+
		"flag provided but not defined: -invalid\n"+
		"usage: gobco [options] package...\n"+
		"  -first-time\n"+
		"    \tprint each condition to stderr when it is reached the first time\n"+
		"  -help\n"+
		"    \tprint the available command line options\n"+
		"  -immediately\n"+
		"    \tpersist the coverage immediately at each check point\n"+
		"  -keep\n"+
		"    \tdon't remove the temporary working directory\n"+
		"  -list-all\n"+
		"    \tat finish, print also those conditions that are fully covered\n"+
		"  -stats string\n"+
		"    \tload and persist the JSON coverage data to this file\n"+
		"  -test option\n"+
		"    \tpass a command line option to \"go test\", such as -vet=off\n"+
		"  -verbose\n"+
		"    \tshow progress messages\n"+
		"  -version\n"+
		"    \tprint the gobco version\n")
}

func (s *Suite) Test_gobco_parseCommandLine__help(c *check.C) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	g := newGobco(&stdout, &stderr)

	c.Check(
		func() { g.parseCommandLine([]string{"gobco", "--help"}) },
		check.Panics,
		exited(0))

	c.Check(stdout.String(), check.Equals, ""+
		"usage: gobco [options] package...\n"+
		"  -first-time\n"+
		"    \tprint each condition to stderr when it is reached the first time\n"+
		"  -help\n"+
		"    \tprint the available command line options\n"+
		"  -immediately\n"+
		"    \tpersist the coverage immediately at each check point\n"+
		"  -keep\n"+
		"    \tdon't remove the temporary working directory\n"+
		"  -list-all\n"+
		"    \tat finish, print also those conditions that are fully covered\n"+
		"  -stats string\n"+
		"    \tload and persist the JSON coverage data to this file\n"+
		"  -test option\n"+
		"    \tpass a command line option to \"go test\", such as -vet=off\n"+
		"  -verbose\n"+
		"    \tshow progress messages\n"+
		"  -version\n"+
		"    \tprint the gobco version\n")
	c.Check(stderr.String(), check.Equals, "")
}

func (s *Suite) Test_gobco_parseCommandLine__version(c *check.C) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	g := newGobco(&stdout, &stderr)

	c.Check(
		func() { g.parseCommandLine([]string{"gobco", "--version"}) },
		check.Panics,
		exited(0))

	c.Check(stdout.String(), check.Equals, "0.9.1\n")
	c.Check(stderr.String(), check.Equals, "")
}

func (s *Suite) Test_gobco_prepareTmp(c *check.C) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	g := newGobco(&stdout, &stderr)
	g.parseCommandLine([]string{"gobco", "."})

	g.prepareTmp()

	c.Check(g.args[0].absTmpFilename, check.Not(check.Equals), "")
}

func (s *Suite) Test_gobco_instrument(c *check.C) {
	var g gobco
	g.parseCommandLine([]string{"gobco", "sample"})
	g.prepareTmp()

	g.instrument()

	tmpdir := g.args[0].absTmpFilename
	c.Check(listRegularFiles(tmpdir), check.DeepEquals, []string{
		"foo.go",
		"foo_test.go",
		"gobco_fixed.go",
		"gobco_fixed_test.go",
		"gobco_variable.go",
		"gobco_variable_test.go",
		"random.go"})

	g.cleanUp()
}

func (s *Suite) Test_gobco_printCond(c *check.C) {
	var out bytes.Buffer
	g := newGobco(&out, &out)

	g.printCond(condition{"location", "zero-zero", 0, 0})
	g.printCond(condition{"location", "zero-once", 0, 1})
	g.printCond(condition{"location", "zero-many", 0, 5})
	g.printCond(condition{"location", "once-zero", 1, 0})
	g.printCond(condition{"location", "once-once", 1, 1})
	g.printCond(condition{"location", "once-many", 1, 5})
	g.printCond(condition{"location", "many-zero", 5, 0})
	g.printCond(condition{"location", "many-once", 5, 1})
	g.printCond(condition{"location", "many-many", 5, 5})

	g.listAll = true
	g.printCond(condition{"location", "many-many-listAll", 5, 5})

	c.Check(out.String(), check.Equals, ""+
		"location: condition \"zero-zero\" was never evaluated\n"+
		"location: condition \"zero-once\" was once false but never true\n"+
		"location: condition \"zero-many\" was 5 times false but never true\n"+
		"location: condition \"once-zero\" was once true but never false\n"+
		"location: condition \"many-zero\" was 5 times true but never false\n"+
		"location: condition \"many-many-listAll\" was 5 times true and 5 times false\n")
}

func (s *Suite) Test_gobco_cleanup(c *check.C) {
	var g gobco
	g.parseCommandLine([]string{"gobco", "sample"})
	g.prepareTmp()

	g.instrument()

	tmpdir := g.args[0].absTmpFilename
	c.Check(listRegularFiles(tmpdir), check.DeepEquals, []string{
		"foo.go",
		"foo_test.go",
		"gobco_fixed.go",
		"gobco_fixed_test.go",
		"gobco_variable.go",
		"gobco_variable_test.go",
		"random.go"})

	g.cleanUp()

	_, err := os.Stat(tmpdir)
	c.Check(os.IsNotExist(err), check.Equals, true)
}

func (s *Suite) Test_gobco_runGoTest(c *check.C) {
	var buf bytes.Buffer
	g := newGobco(&buf, &buf)
	g.parseCommandLine([]string{"gobco", "sample"})
	g.prepareTmp()
	g.instrument()
	g.runGoTest()
	g.printOutput()

	output := buf.String()

	if strings.Contains(output, "[build failed]") {
		c.Fatalf("build failed: %s", output)
	}

	// "go test" returns 1 because one of the sample tests fails.
	c.Check(g.exitCode, check.Equals, 1)

	c.Check(output, check.Matches, `(?s).*Branch coverage: 6/10.*`)

	g.cleanUp()
}

func (s *Suite) Test_gobco__single_file(c *check.C) {
	var buf bytes.Buffer
	g := newGobco(&buf, &buf)
	g.parseCommandLine([]string{"gobco", "sample/foo.go"})
	g.prepareTmp()
	g.instrument()
	g.runGoTest()
	g.printOutput()

	output := buf.String()

	if strings.Contains(output, "[build failed]") {
		c.Fatalf("build failed: %s", output)
	}

	// "go test" returns 1 because one of the sample tests fails.
	c.Check(g.exitCode, check.Equals, 1)

	s.CheckContains(c, output, "Branch coverage: 5/6")

	g.cleanUp()
}

func (s *Suite) Test_gobco__TestMain(c *check.C) {
	var buf bytes.Buffer
	g := newGobco(&buf, &buf)
	g.parseCommandLine([]string{"gobco", "testdata/testmain"})
	g.prepareTmp()
	g.instrument()
	g.runGoTest()
	g.printOutput()

	output := buf.String()

	if strings.Contains(output, "[build failed]") {
		c.Fatalf("build failed: %s", output)
	}

	s.CheckContains(c, output, "begin original TestMain")
	s.CheckContains(c, output, "end original TestMain")

	c.Check(g.exitCode, check.Equals, 0)

	g.cleanUp()
}
