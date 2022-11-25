package instrumenter

// https://go.dev/ref/spec#Go_statements

// TODO: Add systematic tests.

// goStmt covers the instrumentation of [ast.GoStmt], which has the expression
// field Call.
func goStmt() {
	go func(args ...interface{}) {}(1, 1 > 0, !false)
}
