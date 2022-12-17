package testmain

import (
	"fmt"
	"os"
	"testing"
)

// https://github.com/rillig/gobco/issues/4

// If the code to be instrumented already defines a TestMain function, gobco
// instruments that function so that before calling os.Exit, the gobco results
// are written.
//
// If there is no TestMain function yet, gobco installs its own TestMain.

func TestMain(m *testing.M) {
	fmt.Println("begin original TestMain")
	exitCode := m.Run()
	fmt.Println("end original TestMain")
	os.Exit(exitCode)
}

func TestEmpty(t *testing.T) {
	if !isPositive(3) {
		t.Errorf("3 must be positive")
	}
}
