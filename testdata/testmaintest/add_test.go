package add_test

import (
	add "github.com/rillig/gobco/testdata/testmaintest"
	"os"
	"testing"
)

// The TestMain function is defined in a black box test, thus the suffix
// '_test' in the package name.

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestAdd(t *testing.T) {
	have, want := add.Add(1, 2), 3
	if have != want {
		t.Errorf("want %d, have %d", want, have)
	}
}
