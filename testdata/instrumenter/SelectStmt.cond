package instrumenter

// https://go.dev/ref/spec#Select_statements

// TODO: Add systematic tests.

// selectStmt covers the instrumentation of [ast.SelectStmt], which has no
// expression fields.
//
// Select statements are already handled by the normal go coverage.
// Therefore gobco doesn't instrument them.
func selectStmt(c chan int) {
	select {
	case c <- 1:
	}
}
