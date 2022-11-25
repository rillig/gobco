package instrumenter

// https://go.dev/ref/spec#Send_statements

// TODO: Add systematic tests.

// sendStmt covers the instrumentation of [ast.SendStmt], which has the
// expression fields Chan and Value.
func sendStmt(i int) {
	m := map[bool]chan bool{}

	m[i == 11] <- i == 12
}
