package instrumenter

// https://go.dev/ref/spec#Operators

// TODO: Add systematic tests.

// unaryExpr covers the instrumentation of [ast.UnaryExpr], which has the
// expression field X.
func unaryExpr(a, b, c bool, i int) {
	// To avoid double negation, only the innermost expression of a
	// negation is instrumented.
	//
	// Note: The operands of the && are in the "wrong" order because of the
	// order in which the AST nodes are visited. First the two direct
	// operands of the && expression, then each operand further down.
	_ = !!!a
	_ = !b && c

	if -i > 0 {
	}

	// TODO: Do not instrument the outer '!'.
	_ = !(!a)
}
