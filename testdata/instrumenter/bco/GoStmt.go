package instrumenter

// https://go.dev/ref/spec#Go_statements

// TODO: Add systematic tests.

// goStmt covers the instrumentation of [ast.GoStmt], which has the branch in anonymous function
func goStmt() {
	go func(args ...interface{}) {}(1, !(func() bool {
		if 1 == 2 {
			return false
		} else {
			return true
		}
	})(), !false)
}
