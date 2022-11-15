package instrumenter

// https://go.dev/ref/spec#Composite_literals

// CompositeLit covers the instrumentation of [ast.CompositeLit], which has
// the expression fields Type (only relevant at compile time) and Elts.
func compositeLit(i int) {

	// Both keys and values are instrumented.
	_ = map[bool]bool{
		i > 0: i > 1, // TODO: instrument
	}

	// Nested values are instrumented.
	_ = [][]bool{
		{i > 2}, // TODO: instrument
		{i > 3}, // TODO: instrument
	}
}
