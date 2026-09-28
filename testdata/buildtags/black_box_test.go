//go:build gobco
// +build gobco

package buildtags_test

import (
	"github.com/rillig/gobco/testdata/buildtags"
	"testing"
)

// When the black box test is type-checked, the imported package under test
// must be resolved with the same build tags, as otherwise it would not have
// any files.

func TestAbs(t *testing.T) {
	if buildtags.Abs(-3) != 3 {
		t.Fail()
	}
}
