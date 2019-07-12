package main

import (
	"gopkg.in/check.v1"
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

func (s *Suite) Test_gobco_parseCommandLine__only_gobco_option(c *check.C) {
	var g gobco

	g.parseCommandLine([]string{"gobco", "--", "-keep"})

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.keep, check.Equals, true)
	c.Check(g.srcItems, check.DeepEquals, []string{"."})
	c.Check(g.tmpItems, check.DeepEquals, []string{"src/github.com/rillig/gobco"})
}

func (s *Suite) Test_gobco_parseCommandLine__two_packages(c *check.C) {
	var g gobco

	g.parseCommandLine([]string{"gobco", "pkg1", "pkg2"})

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.keep, check.Equals, false)
	c.Check(g.srcItems, check.DeepEquals, []string{"pkg1", "pkg2"})
	c.Check(g.tmpItems, check.DeepEquals, []string{
		"src/github.com/rillig/gobco/pkg1",
		"src/github.com/rillig/gobco/pkg2"})
}
