package instrumenter

import "testing"

func Test_ifStmt(t *testing.T) {
	tests := []struct {
		name     string
		arg1     int
		arg2     string
		arg3     bool
		expected string
	}{
		{"all false", 0, "", false, "other"},
		{"nested 1", 0, "123456", false, "medium string"},
		{"nested 2", 0, "12345678901", false, "long string"},
		{"nested 3", 1, "negative", true, "medium string"},
		{"logical and", 1, "positive", false, "yes, positive"},
		{"simple", 1, "", true, "cond is true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ifStmt(tt.arg1, tt.arg2, tt.arg3)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
