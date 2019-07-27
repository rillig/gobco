package main

// This is the variable part of the gobco code that is injected into the
// test of the package being checked.
//
// It is kept as minimal and maintainable as possible.
//
// It serves as a template to be used in instrumenter.writeGobcoTestGo.

import (
	"os"
	"testing"
)

// TestMain is a demonstration of how gobco may rewrite the code in order
// to persist the coverage data just before exiting.
//
// The original parameter (most probably called m) is renamed to gobcoM,
// and the original parameter is then introduced as a wrapper around the
// original m.
//
// Since testing.M only provides a single method Run that is expected to be
// called a single time, this should be enough for most real-world programs.
func TestMain(gobcoM *testing.M) {
	m := gobcoTestingM{gobcoM}
	os.Exit(m.Run())
}
