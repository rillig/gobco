package instrumenter

import "fmt"

// https://go.dev/ref/spec#If_statements

// ifStmt covers the instrumentation of [ast.IfStmt], which has the expression
// field Cond.
func ifStmt(i int, s string, cond bool) string {

	if i > 0 && s == "positive" {
		return "yes, positive"
	}

	if len(s) > 5 {
		if len(s) > 10 {
			return "long string"
		} else {
			return "medium string"
		}
	}

	// The condition from an if statement is always a boolean expression.
	// And even if the condition is a simple variable, it is wrapped.
	// This is different from arguments to function calls, where simple
	// variables are not wrapped.
	if cond {
		return "cond is true"
	}

	// An if statement, like a switch statement, can have an initializer
	// statement. Other than in a switch statement, the condition in an if
	// statement is used exactly once, so there is no need to introduce a new
	// variable. Therefore, no complicated rewriting is needed.

	if i++; cond {
		return fmt.Sprint("incremented ", i > 5)
	}

	if i := i + 1; cond {
		return fmt.Sprint("added 1, now ", i > 6)
	}

	// Conditions in the initializer are instrumented as well.
	if cond := i > 7; cond {
		return fmt.Sprint("condition in initializer ", i > 8)
	}

	if i < 21 {
		i += 31
	} else if i < 22 {
		i += 32
	} else {
		i += 33
	}

	return "other"
}
