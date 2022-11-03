package instrumenter

// https://go.dev/ref/spec#For_statements

// TODO: Add systematic tests.

func rangeStmt(a rune, b string) bool {

	// In a RangeStmt there is no obvious condition, therefore nothing
	// is wrapped. Maybe it would be possible to distinguish empty and
	// nonempty, but that would require a temporary variable, to avoid
	// computing the range expression twice.
	//
	// Code that wants to have this check in a specific place can just
	// manually add a statement before the range statement:
	//  _ = len(b) > 0
	for _, r := range b {
		if r == a {
			return true
		}
	}
	return false
}
