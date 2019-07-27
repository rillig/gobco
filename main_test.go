package main

import (
	"bytes"
	"gopkg.in/check.v1"
	"os"
	"path/filepath"
)

func (s *Suite) Test_gobco_parseCommandLine(c *check.C) {
	var g gobco

	g.parseCommandLine([]string{"gobco"})

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.keep, check.Equals, false)
	c.Check(g.srcItems, check.DeepEquals, []string{"."})
	c.Check(g.tmpItems, check.DeepEquals, []string{"src/github.com/rillig/gobco"})
}

func (s *Suite) Test_gobco_parseCommandLine__keep(c *check.C) {
	var g gobco

	g.parseCommandLine([]string{"gobco", "-keep"})

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.keep, check.Equals, true)
	c.Check(g.srcItems, check.DeepEquals, []string{"."})
	c.Check(g.tmpItems, check.DeepEquals, []string{"src/github.com/rillig/gobco"})
}

func (s *Suite) Test_gobco_parseCommandLine__go_test_options(c *check.C) {
	var g gobco

	g.parseCommandLine([]string{"gobco", "-test", "-vet=off", "-test", "help", "pkg"})

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.goTestOpts, check.DeepEquals, []string{"-vet=off", "help"})
	c.Check(g.srcItems, check.DeepEquals, []string{"pkg"})
	c.Check(g.tmpItems, check.DeepEquals, []string{"src/github.com/rillig/gobco/pkg"})
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
	c.Check(g.tmpItems, check.DeepEquals, []string{
		"src/github.com/rillig/gobco/pkg1",
		"src/github.com/rillig/gobco/pkg2"})
}

func (s *Suite) Test_gobco_instrument(c *check.C) {
	var g gobco
	g.parseCommandLine([]string{"gobco", "sample"})
	g.prepareTmpEnv()
	tmpdir := filepath.Join(g.tmpdir, g.tmpItems[0])

	g.instrument()

	c.Check(listRegularFiles(tmpdir), check.DeepEquals, []string{
		"foo.go",
		"foo_test.go",
		"gobco.go",
		"gobco_test.go"})

	g.cleanUp()

	_, err := os.Stat(tmpdir)
	c.Check(os.IsNotExist(err), check.Equals, true)
}

func (s *Suite) Test_gobco_runGoTest(c *check.C) {
	var output bytes.Buffer
	g := newGobco(&output, &output)
	g.parseCommandLine([]string{"gobco", "sample"})
	g.prepareTmpEnv()
	g.instrument()
	g.runGoTest()
	g.cleanUp()

	// "go test" returns 1 because one of the sample tests fails.
	c.Check(g.exitCode, check.Equals, 1)
	c.Check(output.String(), check.Matches, `(?s).*Branch coverage: 5/6.*`)
}
