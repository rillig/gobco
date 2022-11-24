package instrumenter

// https://go.dev/ref/spec#Index_expressions
// https://go.dev/ref/spec#Arithmetic_operators
// https://go.dev/ref/spec#Comparison_operators
// https://go.dev/ref/spec#Logical_operators

// TODO: Add systematic tests.

// binaryExpr covers the instrumentation of [ast.BinaryExpr], which has the
// expression fields X and Y.
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

	// When a long chain of '&&' or '||' is parsed, it is split into
	// the rightmost operand and the rest, instrumenting both these
	// parts.
	//
	// TODO: For '&&' and '||', it's enough if each terminal expression
	//  is instrumented.
	_ = i == 11 ||
		i == 12 ||
		i == 13 ||
		i == 14 ||
		i == 15

	// The operators '&&' and '||' can be mixed as well.
	_ = i == 11 ||
		i >= 12 && i <= 13 ||
		i >= 14 && i <= 15
}
