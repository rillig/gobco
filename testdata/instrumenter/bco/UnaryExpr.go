package instrumenter

// https://go.dev/ref/spec#Operators

// TODO: Add systematic tests.

// unaryExpr covers the instrumentation of [ast.UnaryExpr], which has the
// branch statement.
func unaryExpr(a, b, c bool, i int) {
	// not instrumented, as it is not branch
	_ = !!!a
	_ = !b && c

	// Test nested function body of UnaryExpr are instrumented
	if b = !a; b {
		_ = !b
	}

	if !(b && !(func() bool {
		if c == !b {
			return false
		} else {
			return true
		}
	})()) {
		_ = !c
	}

	if -i > 0 {
	}

	//  not instrumented, as it is not branch
	_ = !(!a)
}
