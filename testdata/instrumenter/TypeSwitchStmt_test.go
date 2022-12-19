package instrumenter

import "testing"

func Test_typeSwitchStmt(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			"int",
			0,
			"parenthesized int",
		},
		{
			"nil",
			nil,
			"parenthesized nil",
		},
		{
			"uint",
			uint(0),
			"uint uint",
		},
		{
			"uint8",
			uint8(0),
			"any uint8",
		},
		{
			"uint16",
			uint16(0),
			"any uint16",
		},
		{
			"other",
			0.0,
			"nil", // Due to the second argument.
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := typeSwitchStmt(tt.value, nil)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

func Test_typeSwitchStmtScopes(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			"int",
			0,
			"integer int",
		},
		{
			"uint",
			uint(0),
			"integer uint",
		},
		{
			"string",
			"",
			"string string",
		},
		{
			"struct",
			struct{}{},
			"struct{} struct {}",
		},
		{
			"byte",
			uint8(0),
			"byte",
		},
		{
			"other",
			0.0,
			"other float64",
		},
		{
			"literal nil",
			nil,
			"nil",
		},
		{
			"typed nil",
			[]int(nil),
			"other []int",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := typeSwitchStmt(0.0, tt.value)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
