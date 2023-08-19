package instrumenter

// https://go.dev/ref/spec#Expression_statements

// exprStmt covers the instrumentation of [ast.ExprStmt], which has the
// expression field X.
//
// Expression statements are not instrumented themselves.
func exprStmt(i int, ch map[bool]<-chan int) {

	f := func(bool) {}

	// Statement expressions may be parenthesized.
	f(i > 0)
	(f(i > 1))
	(f)(i > 2)
	((f)(i > 3))

	<-ch[i > 10]
	(<-ch[i > 11])
	<-(ch[i > 12])
	(<-(ch[i > 13]))
}
