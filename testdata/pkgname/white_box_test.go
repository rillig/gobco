package pkgname

import "testing"

// In a black box test, the package name is the same as in the main code
// and therefore can access the unexported members of the main code,
// as well as the exported ones.

func TestWhiteBox(t *testing.T) {
	if unexported(true) != 'U' {
		t.Fail()
	}
}
