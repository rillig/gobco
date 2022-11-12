package instrumenter

// https://go.dev/ref/spec#If_statements

// ifStmt covers the instrumentation of [ast.IfStmt], which has the expression
// field Cond.
func ifStmt(i int, s string, cond bool) bool {

	if i > 0 && s == "positive" {
		return true
	}

	if len(s) > 5 {
		return len(s) > 10
	}

	// The condition from an if statement is always a boolean expression.
	// And even if the condition is a simple variable, it is wrapped.
	// This is different from arguments to function calls, where simple
	// variables are not wrapped.
	if cond {
		return true
	}

	// An if statement, like a switch statement, can have an initializer
	// statement. Other than in a switch statement, the condition in an if
	// statement is used exactly once, so there is no need to introduce a new
	// variable. Therefore, no complicated rewriting is needed.

	if i++; cond {
		return i > 5
	}

	if i := i + 1; cond {
		return i > 6
	}

	// Conditions in the initializer are instrumented as well.
	// TODO: Instrument the initialization before the condition.
	if cond := i > 7; cond {
		return i > 8
	}

	return false
}
