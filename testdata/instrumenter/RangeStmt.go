package instrumenter

// https://go.dev/ref/spec#For_statements

// TODO: Add systematic tests.

// rangeStmt covers the instrumentation of [ast.RangeStmt], which has the
// expression fields Key, Value and X.
func rangeStmt(i int) bool {
	mi := map[bool]int{}
	ms := map[bool]string{}
	mr := map[bool]rune{}

	// In a RangeStmt there is no obvious condition, therefore nothing
	// is wrapped. Maybe it would be possible to distinguish empty and
	// nonempty, but that would require a temporary variable, to avoid
	// computing the range expression twice.
	//
	// Code that wants to have this check in a specific place can just
	// manually add a statement before the range statement:
	//  _ = len(b) > 0
	for _, r := range ms[i > 10] { // TODO: instrument
		if r == mr[i > 11] {
			return true
		}
	}

	// In a range loop using '=', the expressions on the left don't need
	// to be plain identifiers.
	for mi[i > 10], mr[i > 11] = range ms[i > 12] { // TODO: instrument
	}

	return false
}
