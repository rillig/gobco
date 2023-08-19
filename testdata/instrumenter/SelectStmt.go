package instrumenter

// https://go.dev/ref/spec#Select_statements

// TODO: Add systematic tests.

// selectStmt covers the instrumentation of [ast.SelectStmt], which has no
// expression fields.
//
// Select statements are not instrumented themselves, as they are already
// covered.by the standard go coverage tool.
func selectStmt(c chan int) {
	select {
	case c <- 1:
	}
}
