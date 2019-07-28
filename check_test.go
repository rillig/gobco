package main

import (
	"bytes"
	"gopkg.in/check.v1"
	"os"
	"strings"
	"testing"
)

type Suite struct{}

func Test(t *testing.T) {
	check.Suite(new(Suite))
	check.TestingT(t)
}

func (s *Suite) SetUpTest(c *check.C) {
	exit = func(code int) { panic(exited(code)) }
}

func (s *Suite) TearDownTest(c *check.C) {
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
