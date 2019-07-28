package main

import (
	"gopkg.in/check.v1"
	"os"
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

type exited int
