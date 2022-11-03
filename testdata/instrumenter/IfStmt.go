package instrumenter

// https://go.dev/ref/spec#If_statements

// TODO: Add systematic tests.

// The condition from an if statement is always a boolean expression.
// And even if the condition is a simple variable, it is wrapped.
// This is different from arguments to function calls, where simple
// variables are not wrapped.
func ifStmt(a int, b string, c bool) bool {
	if a > 0 && b == "positive" {
		return true
	}
	if len(b) > 5 {
		return len(b) > 10
	}
	if c {
		return true
	}
	return false
}
