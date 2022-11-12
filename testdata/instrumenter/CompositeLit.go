package instrumenter

// https://go.dev/ref/spec#Composite_literals

// TODO: Add systematic tests.

// CompositeLit covers the instrumentation of [ast.CompositeLit], which has
// the expression fields Type (only relevant at compile time) and Elts.
func compositeLit() {
}
