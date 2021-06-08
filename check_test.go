package main

import (
	"bytes"
	"gopkg.in/check.v1"
	"os"
	"strings"
	"testing"
)

type Suite struct {
	out bytes.Buffer
	err bytes.Buffer
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

func Test(t *testing.T) {
	check.Suite(new(Suite))
	check.TestingT(t)
}

func (s *Suite) SetUpTest(c *check.C) {
	exit = func(code int) {
		panic(exited(code))
	}
}

func (s *Suite) TearDownTest(c *check.C) {

	if stdout := s.Stdout(); stdout != "" {
		c.Errorf("%s: unchecked stdout %q", c.TestName(), stdout)
	}

	if stderr := s.Stderr(); stderr != "" {
		c.Errorf("%s: unchecked stderr %q", c.TestName(), stderr)
	}

	exit = os.Exit
}

func (s *Suite) CheckContains(c *check.C, output, str string) {
	if !strings.Contains(output, str) {
		c.Errorf("expected %q in the output, got %q", str, output)
	}
}

func (s *Suite) CheckNotContains(c *check.C, output, str string) {
	if strings.Contains(output, str) {
		c.Errorf("expected %q to not appear in the output %q", str, output)
	}
}

func (s *Suite) RunMain(c *check.C, exitCode int, argv ...string) (stdout, stderr string) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	c.Check(
		func() { gobcoMain(&outBuf, &errBuf, argv...) },
		check.Panics,
		exited(exitCode))

	return outBuf.String(), errBuf.String()
}

type exited int
