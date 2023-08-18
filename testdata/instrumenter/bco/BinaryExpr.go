package instrumenter

// https://go.dev/ref/spec#Index_expressions
// https://go.dev/ref/spec#Arithmetic_operators
// https://go.dev/ref/spec#Comparison_operators
// https://go.dev/ref/spec#Logical_operators

// TODO: Add systematic tests.

// binaryExpr covers the instrumentation of [ast.BinaryExpr], which has the
// expression fields X and Y.
func binaryExpr(i int, a bool, b bool, c bool) {
	// Comparison expressions have return type boolean and are
	// therefore not instrumented since no branches.
	_ = i > 0
	pos := i > 0

	// Expressions consisting of a single identifier do not look like boolean
	// expressions, therefore they are not instrumented.
	_ = pos

	// Binary boolean assingments are not instrumented [no branches]
	//
	// Copying boolean variables is not wrapped though since there
	// is no code branch involved.
	//
	// Also, gobco only looks at the parse tree without any type resolution.
	// Therefore it cannot decide whether a variable is boolean or not.
	both := a && b
	either := a || b
	_, _ = both, either

	// Long chain of Binary boolean assingments are not instrumented [no branches]
	_ = i == 11 ||
		i == 12 ||
		i == 13 ||
		i == 14 ||
		i == 15
	_ = i != 21 &&
		i != 22 &&
		i != 23 &&
		i != 24 &&
		i != 25

	// The operators '&&' and '||' can be mixed as well but not instrumented [no branches]
	_ = i == 31 ||
		i >= 32 && i <= 33 ||
		i >= 34 && i <= 35

	m := map[bool]int{}
	_ = m[i == 41] == m[i == 42]

	// In complex conditions, only instrument the terminal conditions
	// 'a', 'b' and 'c', but not the intermediate conditions,
	// to avoid large and redundant conditions in the output.
	f := func(args ...bool) {}
	f(a && b)
	f(a && b && c)
	f(!a)
	f(!a && !b && !c)

	// Instrument deeply nested conditions in if statements.
	mi := map[bool]int{}
	if i == mi[i > 51] {
		_ = i == mi[i > 52]
	}
	for i == mi[i > 61] {
		_ = i == mi[i > 62]
	}

	// Test nested function body
	if i > 10 && !(i < 100 && a) {
		a = true
		// inner function
		var displayName = func(b bool) bool {
			return !b && !a
		}
		if i > 50 && !displayName(b) {
			_ = !(!a)
			_ = displayName(b)
			if i > 50 && !displayName(b) {
				_ = !a
			}
		}
	}
}
