package main

import (
	"gopkg.in/check.v1"
)

func (s *Suite) Test_gobco_parseCommandLine(c *check.C) {
	args := func(args ...string) []string { return args }

	var g gobco
	g.parseCommandLine(args("gobco"))

	c.Check(g.exitCode, check.Equals, 0)
	c.Check(g.firstTime, check.Equals, false)
	c.Check(g.listAll, check.Equals, false)
	c.Check(g.keep, check.Equals, false)
	c.Check(g.srcItems, check.DeepEquals, []string{"."})
	c.Check(g.tmpItems, check.DeepEquals, []string{"src/github.com/rillig/gobco"})
}
