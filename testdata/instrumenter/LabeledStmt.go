package instrumenter

// https://go.dev/ref/spec#Labeled_statements

// TODO: Add systematic tests.

// labeledStmt covers the instrumentation of [ast.LabeledStmt], which has no
// expression fields.
//
// Labeled statements are not instrumented themselves.
func labeledStmt() {
}
