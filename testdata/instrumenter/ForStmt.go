package instrumenter

// https://go.dev/ref/spec#For_statements

// TODO: Add systematic tests.

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
