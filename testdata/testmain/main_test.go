package testmain

import (
	"fmt"
	"os"
	"testing"
)

// https://github.com/rillig/gobco/issues/4

// There may be a TestMain function in the package to be tested.
// Even though gobco defines its own function of that name, the
// original function must still be called.

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
