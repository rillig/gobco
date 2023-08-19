package instrumenter

// https://go.dev/ref/spec#Defer_statements

// TODO: Add systematic tests.

// deferStmt covers the instrumentation of [ast.DeferStmt], which has the
// expression field Call.
//
// Defer statements are not instrumented themselves.
func deferStmt() {
	defer func(args ...interface{}) {}(1, 1 > 0, !false)
}
