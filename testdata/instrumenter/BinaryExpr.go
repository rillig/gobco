package instrumenter

// https://go.dev/ref/spec#Index_expressions
// https://go.dev/ref/spec#Arithmetic_operators
// https://go.dev/ref/spec#Comparison_operators
// https://go.dev/ref/spec#Logical_operators

// TODO: Add systematic tests.

func binaryExpr(i int, a bool, b bool) {
	// Comparison expressions have return type boolean and are
	// therefore instrumented.
	_ = i > 0
	pos := i > 0

	// Expressions consisting of a single identifier do not look like boolean
	// expressions, therefore they are not instrumented.
	_ = pos

	// Binary boolean operators are clearly identifiable and are
	// therefore wrapped.
	//
	// Copying boolean variables is not wrapped though since there
	// is no code branch involved.
	//
	// Also, gobco only looks at the parse tree without any type resolution.
	// Therefore it cannot decide whether a variable is boolean or not.
	both := a && b
	either := a || b
	_, _ = both, either
}
