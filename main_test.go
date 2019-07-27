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
	c.Check(g.srcItems, check.DeepEquals, []string{"."})
	c.Check(g.tmpItems, check.DeepEquals, []tmpItem{{"github.com/rillig/gobco", true}})
}

func (s *Suite) Test_gobco_parseCommandLine__keep(c *check.C) {
	var g gobco

	g.parseCommandLine([]string{"gobco", "-keep"})

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.keep, check.Equals, true)
	c.Check(g.srcItems, check.DeepEquals, []string{"."})
	c.Check(g.tmpItems, check.DeepEquals, []tmpItem{
		{"github.com/rillig/gobco", true}})
}

func (s *Suite) Test_gobco_parseCommandLine__go_test_options(c *check.C) {
	var g gobco

	g.parseCommandLine([]string{"gobco", "-test", "-vet=off", "-test", "help", "pkg"})

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.goTestOpts, check.DeepEquals, []string{"-vet=off", "help"})
	c.Check(g.srcItems, check.DeepEquals, []string{"pkg"})
	c.Check(g.tmpItems, check.DeepEquals, []tmpItem{
		{"github.com/rillig/gobco/pkg", false}})
}

func (s *Suite) Test_gobco_parseCommandLine__two_packages(c *check.C) {
	var g gobco

	c.Check(
		func() { g.parseCommandLine([]string{"gobco", "pkg1", "pkg2"}) },
		check.Panics,
		"gobco: checking multiple packages doesn't work yet")

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.keep, check.Equals, false)
	c.Check(g.srcItems, check.DeepEquals, []string{"pkg1", "pkg2"})
	c.Check(g.tmpItems, check.DeepEquals, []tmpItem{
		{"github.com/rillig/gobco/pkg1", false},
		{"github.com/rillig/gobco/pkg2", false}})
}

func (s *Suite) Test_gobco_parseCommandLine__usage(c *check.C) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	g := newGobco(&stdout, &stderr)

	exit = func(code int) {
		c.Check(code, check.Equals, 2)
		panic("exited as expected")
	}

	c.Check(
		func() { g.parseCommandLine([]string{"gobco", "-invalid"}) },
		check.Panics,
		"exited as expected")
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

func (s *Suite) Test_gobco_instrument(c *check.C) {
	var g gobco
	g.parseCommandLine([]string{"gobco", "sample"})
	g.prepareTmpEnv()
	tmpdir := g.tmpSrc(g.tmpItems[0].rel)

	g.instrument()

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

func (s *Suite) Test_gobco_cleanup(c *check.C) {
	var g gobco
	g.parseCommandLine([]string{"gobco", "sample"})
	g.prepareTmpEnv()
	tmpdir := g.tmpSrc(g.tmpItems[0].rel)

	g.instrument()

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
	g.prepareTmpEnv()
	g.instrument()
	g.runGoTest()
	g.printOutput()

	output := buf.String()

	if strings.Contains(output, "[build failed]") {
		c.Fatalf("build failed: %s", output)
	}

	// "go test" returns 1 because one of the sample tests fails.
	c.Check(g.exitCode, check.Equals, 1)

	c.Check(output, check.Matches, `(?s).*Branch coverage: 5/8.*`)

	g.cleanUp()
}

func (s *Suite) Test_gobco__single_file(c *check.C) {
	var buf bytes.Buffer
	g := newGobco(&buf, &buf)
	g.parseCommandLine([]string{"gobco", "sample/foo.go"})
	g.prepareTmpEnv()
	g.instrument()
	g.runGoTest()
	g.printOutput()

	output := buf.String()

	if strings.Contains(output, "[build failed]") {
		c.Fatalf("build failed: %s", output)
	}

	// "go test" returns 1 because one of the sample tests fails.
	c.Check(g.exitCode, check.Equals, 1)

	c.Check(output, check.Matches, `(?s).*Branch coverage: 5/6.*`)

	g.cleanUp()
}
