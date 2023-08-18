package instrumenter

// https://go.dev/ref/spec#Selectors

// TODO: Add systematic tests.

// selectorExpr covers the instrumentation of [ast.SelectorExpr], which has
// the branch.
func selectorExpr() {
	m := map[bool]struct{ a string }{true: {""}}

	_ = m[1 > 0].a
}
