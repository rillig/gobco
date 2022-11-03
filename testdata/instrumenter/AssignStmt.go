package instrumenter

// https://go.dev/ref/spec#Assignment_statements

// TODO: Add systematic tests.

func assignStmt() {
	a := "a string"
	b := len(a)
	c := b > 0
	c = !c

	// The operators '|=' and '&=' are not defined on bool,
	// they are only defined on integer types.
	b |= 7
	b &= -7
}

// Before gobco-0.10.2, conditionals on the left-hand side of an assignment
// statement were not instrumented. It's probably an edge case but may
// nevertheless occur in practice.
func assignStmtLeft(i int) {
	m := make(map[bool]string)
	m[i > 0] = "yes"
}
