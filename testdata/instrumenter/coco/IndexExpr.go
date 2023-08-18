package instrumenter

// https://go.dev/ref/spec#Index_expressions

// TODO: Add systematic tests.

// indexExpr covers the instrumentation of [ast.IndexExpr], which has the
// expression fields X and Index.
func indexExpr(i int, cond bool) {
	m := make(map[bool]string)
	mm := make(map[bool]map[bool]string)

	// This index expression is instrumented, as its type must be bool.
	m[i > 0] = "yes"

	// This index expression is not instrumented, as gobco doesn't resolve
	// types.
	m[cond] = "cond"

	// Both index expressions are instrumented.
	mm[i > 1][i > 2] = "both"
}
