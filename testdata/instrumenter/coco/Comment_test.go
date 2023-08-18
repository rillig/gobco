package instrumenter

import "testing"

func TestComment(t *testing.T) {
	if commentGo == "" {
		t.Errorf("go:embed must be preserved")
	}
}
