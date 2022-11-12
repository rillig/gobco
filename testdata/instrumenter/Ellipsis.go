package instrumenter

// https://go.dev/ref/spec#Function_types

// TODO: Add systematic tests.

// ellipsis covers the instrumentation of [ast.Ellipsis], which has the
// expression Elt (only relevant at compile time).
//
// see [callExpr] for the '...' in a call expression.
func ellipsis() {
}
