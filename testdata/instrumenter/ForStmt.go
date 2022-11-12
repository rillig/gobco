package instrumenter

// https://go.dev/ref/spec#For_statements

// TODO: Add systematic tests.

// forStmt covers the instrumentation of [ast.ForStmt], which has the
// expression field Cond.
func forStmt(a byte, b string) bool {

	// The condition of a ForStmt is always a boolean expression and is
	// therefore instrumented, no matter if it is a simple or a complex
	// expression.
	for i := 0; i < len(b); i++ {
		if b[i] == a {
			return true
		}
	}
	return false
}

// A ForStmt without condition can only have one outcome.
// Therefore no branch coverage is necessary.
func forever() {
	for {
		break
	}
}
