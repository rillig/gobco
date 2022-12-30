package pkgname_test

import (
	"github.com/rillig/gobco/testdata/pkgname"
	"testing"
)

// In a black box test, the package name ends with '_test'
// and therefore can only access the exported members of the main code.

func TestBlackBox(t *testing.T) {
	if pkgname.Exported(true) != 'E' {
		t.Fail()
	}
}
