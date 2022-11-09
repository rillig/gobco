package instrumenter

// https://go.dev/ref/spec#Assignment_statements

func assignStmt() {
	m := map[bool]int{}
	mm := map[bool]bool{}
	var i int
	var b1, b2 bool

	// Assignments without conditions are kept as-is.
	i = 3 - i
	assertEquals(i, 3)

	// Most assignments assign to a single variable.
	mm[i > 0] = i > 3

	// An assignment can assign multiple variables simultaneously.
	b1, m[i > 0], b2 = b2, m[i > 1], b1
	mm[i > 10], mm[i > 11], mm[i > 12] =
		i > 13, i > 14, i > 15

	// Assignments from a function call can assign to multiple variables.
	mm[i > 0], mm[i > 1], mm[i > 2] =
		func() (bool, bool, bool) { return false, false, false }()

	// Operands may be parenthesized.
	// TODO: Instrument parenthesized expressions.
	(m[i > 21]), (m[i > 22]) = (m[i > 23]), (m[i > 24])

	// Since the left-hand side in an assignment must be a variable,
	// and since only those expressions are instrumented that are
	// syntactically bool, the instrumented code never converts an
	// lvalue to an rvalue.

	// The instrumentation wraps each condition with a function call,
	// so the order of evaluation stays the same.

	// The operators '|=' and '&=' are not defined on bool,
	// they are only defined on integer types.
	i |= 7
	i &= -7
}
