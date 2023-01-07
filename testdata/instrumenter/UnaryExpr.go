package instrumenter

// https://go.dev/ref/spec#Operators

// TODO: Add systematic tests.

// unaryExpr covers the instrumentation of [ast.UnaryExpr], which has the
// expression field X.
func unaryExpr(a, b, c bool, i int) {
	// To avoid double negation, only the innermost expression of a
	// negation is instrumented.
	_ = !!!a
	_ = !b && c

	if -i > 0 {
	}

	// In double negations, only the terminal condition is wrapped.
	_ = !(!a)
}
