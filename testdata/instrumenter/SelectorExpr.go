package instrumenter

// https://go.dev/ref/spec#Selectors

// TODO: Add systematic tests.

// selectorExpr covers the instrumentation of [ast.SelectorExpr], which has
// the expression field X.
//
// Selector expressions are not instrumented themselves, as they are already
// covered by the standard go coverage tool.
func selectorExpr() {
	m := map[bool]struct{ a string }{true: {""}}

	_ = m[1 > 0].a
}
