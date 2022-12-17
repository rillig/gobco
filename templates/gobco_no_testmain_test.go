package main

// This file is used if the code to be instrumented does not define its own
// TestMain function.

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	filename := gobcoCounts.filename()
	gobcoCounts.load(filename)
	exitCode := m.Run()
	gobcoCounts.persist(filename)
	os.Exit(exitCode)
}
