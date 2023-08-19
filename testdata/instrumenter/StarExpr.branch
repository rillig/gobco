package instrumenter

// https://go.dev/ref/spec#Pointer_types
// https://go.dev/ref/spec#Address_operators

// TODO: Add systematic tests.

// starExpr covers the instrumentation of [ast.StarExpr], which has the
// expression field X.
//
// Star expressions are not instrumented themselves.
func starExpr() {
	m := map[bool]*int{}

	_ = *m[11 == 0]
}
