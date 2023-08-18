package instrumenter

// https://go.dev/ref/spec#Type_assertions

// TODO: Add systematic tests.

// typeAssertExpr covers the instrumentation of [ast.TypeAssertExpr], which
// has the branch fields X and Type (the latter is only relevant at
// compile time).
func typeAssertExpr() {
	m := map[bool]interface{}{}

	_ = m[11 != 0].(int)
}
