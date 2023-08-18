package instrumenter

// https://go.dev/ref/spec#IncDec_statements

// incDecStmt covers the instrumentation of [ast.IncDecStmt], which has the
// expression field X.
func incDecStmt() {

	// The expression must be addressable ...
	i := 0
	i++
	i--

	var arr [1]int
	arr[0]++
	arr[0]--

	// ... or a map index expression.
	m := map[bool]int{}
	m[!true]++
	m[!false]--
	m[i == 11]++
	m[i == 12]--
}
