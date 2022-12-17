package main

// This file is used if the code to be instrumented does not define its own
// TestMain function.

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(gobcoRun(m))
}

func gobcoRun(m *testing.M) int {
	filename := gobcoCounts.filename()
	gobcoCounts.load(filename)
	defer gobcoCounts.persist(filename)

	return m.Run()
}
