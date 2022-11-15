package instrumenter

// https://go.dev/ref/spec#Function_literals

// funcLit covers the instrumentation of [ast.FuncLit], which has no
// expression fields.
func funcLit() {
	inner := func(i int) bool {
		return i > 0
	}
	inner(3)
	inner(-3)
}
