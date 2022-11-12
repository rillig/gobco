package instrumenter

// https://go.dev/ref/spec#Calls

// TODO: Add systematic tests.

// callExpr covers the instrumentation of [ast.CallExpr], which has the
// expression fields Fun and Args.
func callExpr(a bool, b string) bool {
	// Those arguments to function calls that can be clearly identified
	// as boolean expressions are wrapped. Direct boolean arguments are
	// not wrapped since, as of July 2019, gobco does not use type
	// resolution.

	if len(b) > 0 {
		return callExpr(len(b)%2 == 0, b[1:])
	}

	// A CallExpr without identifier is also covered. The test for an
	// identifier is only needed to filter out the calls to gobcoCover,
	// which may have been inserted by a previous instrumentation.
	(func(a bool) {})(1 != 2)

	return false
}
