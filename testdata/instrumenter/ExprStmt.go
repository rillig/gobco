package instrumenter

// https://go.dev/ref/spec#Expression_statements

func exprStmt(i int, ch map[bool]<-chan int) {

	f := func(bool) {}

	// Statement expressions may be parenthesized.
	f(i > 0)
	(f(i > 1))
	(f)(i > 2)
	((f)(i > 3))

	// TODO: Instrument the receive statements.
	<-ch[i > 10]
	(<-ch[i > 11])
	<-(ch[i > 12])
	(<-(ch[i > 13]))
}
