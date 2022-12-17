package testmain

import (
	"fmt"
	"os"
	"testing"
)

// https://github.com/rillig/gobco/issues/4

// There may be a TestMain function in the package to be tested.
// In that function, each call to os.Exit is wrapped so that the
// coverage statistics are written just before exiting.

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
