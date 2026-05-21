package target_test

import (
	"testing"

	"github.com/rillig/gobco/testdata/issue33/helper"
)

func TestAdd(t *testing.T) {
	if helper.AddViaTarget(1, 2) != 3 {
		t.Fail()
	}
}
