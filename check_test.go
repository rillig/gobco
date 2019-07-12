package main

import (
	"gopkg.in/check.v1"
	"testing"
)

type Suite struct{}

func Test(t *testing.T) {
	check.Suite(new(Suite))
	check.TestingT(t)
}
