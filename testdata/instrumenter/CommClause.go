package instrumenter

// https://go.dev/ref/spec#Select_statements

// TODO: Add systematic tests.

// commClause covers the instrumentation of [ast.CommClause], which has no
// expression fields.
//
// Communication clauses are not instrumented themselves.
func commClause() {
}
