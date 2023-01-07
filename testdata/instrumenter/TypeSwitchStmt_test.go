package instrumenter

import "testing"

func Test_typeSwitchStmt(t *testing.T) {
	tests := []struct {
		name     string
		arg1     interface{}
		arg2     interface{}
		expected string
	}{
		{"int", 0, nil, "parenthesized int"},
		{"nil", nil, nil, "parenthesized nil"},
		{"uint", uint(0), nil, "uint uint"},
		{"uint8", uint8(0), nil, "any uint8"},
		{"uint16", uint16(0), nil, "any uint16"},
		{"other", 0.0, nil, "nil" /* Due to the second argument. */},
		{"int", 0.0, 0, "integer int"},
		{"uint", 0.0, uint(0), "integer uint"},
		{"string", 0.0, "", "string string"},
		{"struct", 0.0, struct{}{}, "struct{} struct {}"},
		{"byte", 0.0, uint8(0), "byte"},
		{"other", 0.0, 0.0, "other float64"},
		{"literal nil", 0.0, nil, "nil"},
		{"typed nil", 0.0, []int(nil), "other []int"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := typeSwitchStmt(tt.arg1, tt.arg2)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
