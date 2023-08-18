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

	// Function literals are typically larger than other expressions.
	if func() int { return 3 }() > 2 {
	}

	// Function literals can span multiple lines.
	// The gobco output format has to deal with expressions that include
	// line breaks.
	if func() int {
		return 3
	}() > 2 {
	}

}
