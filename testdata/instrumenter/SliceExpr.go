package instrumenter

// https://go.dev/ref/spec#Slice_expressions

// TODO: Add systematic tests.

// sliceExpr covers the instrumentation of [ast.SliceExpr], which has the
// expression fields X, Low, High and Max.
//
// Slice expressions are not instrumented themselves.
func sliceExpr() {
	m := map[bool]int{}
	ms := map[bool][]int{}
	var slice []int

	_ = slice[m[11 == 0]:]
	_ = slice[:m[21 == 0]]
	_ = ms[30 == 0][m[31 == 0]:m[32 == 0]:m[33 == 0]]

	// A slice can only occur in a comparison if it is compared to nil.
	// In that case, it doesn't need to be parenthesized when generating
	// the comparison string.
	switch slice[:] {
	case nil:
	}
}
