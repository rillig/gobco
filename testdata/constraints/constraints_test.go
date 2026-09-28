package constraints

import "testing"

func TestSign(t *testing.T) {
	if Sign(1) == "" {
		t.Error("Sign must not return an empty string")
	}
}
