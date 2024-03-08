package instrumenter

// https://go.dev/ref/spec#Index_expressions
// https://go.dev/ref/spec#Arithmetic_operators
// https://go.dev/ref/spec#Comparison_operators
// https://go.dev/ref/spec#Logical_operators

// TODO: Add systematic tests.

// binaryExpr covers the instrumentation of [ast.BinaryExpr], which has the
// expression fields X and Y.
//
// In condition coverage mode, binary expressions whose type is syntactically
// guaranteed to be 'bool' are instrumented.
//
// In branch coverage mode, binary expressions are not instrumented themselves.
func binaryExpr(i int, a bool, b bool, c bool) {
	// Comparison expressions have return type boolean and are
	// therefore instrumented.
	_ = i > 0
	pos := i > 0

	// Expressions consisting of a single identifier do not look like boolean
	// expressions, therefore they are not instrumented.
	_ = pos

	// Binary boolean operators are clearly identifiable and are
	// therefore instrumented in condition coverage mode.
	//
	// Copying boolean variables is not instrumented though since there
	// is no code branch involved.
	//
	// Also, gobco only looks at the parse tree without any type resolution.
	// Therefore it cannot decide whether a variable is boolean or not.
	both := a && b
	either := a || b
	_, _ = both, either

	// When a long chain of '&&' or '||' is parsed, it is split into
	// the rightmost operand and the rest, instrumenting both these
	// parts.
	_ = i == 11 ||
		i == 12 ||
		i == 13 ||
		i == 14 ||
		i == 15
	_ = i != 21 &&
		i != 22 &&
		i != 23 &&
		i != 24 &&
		i != 25

	// The operators '&&' and '||' can be mixed as well.
	_ = i == 31 ||
		i >= 32 && i <= 33 ||
		i >= 34 && i <= 35

	m := map[bool]int{}
	_ = m[i == 41] == m[i == 42]

	// In condition coverage mode, do not instrument complex conditions
	// but instead their terminal conditions, in this case 'a', 'b' and
	// 'c', to avoid large and redundant conditions in the output.
	f := func(args ...bool) {}
	f(a && b)
	f(a && b && c)
	f(!a)
	f(!a && !b && !c)

	// In condition coverage mode, instrument deeply nested conditions in
	// if statements; in branch coverage mode, only instrument the main
	// condition.
	mi := map[bool]int{}
	if i == mi[i > 51] {
		_ = i == mi[i > 52]
	}
	for i == mi[i > 61] {
		_ = i == mi[i > 62]
	}

	type MyBool bool
	var nativeTrue, nativeFalse = true, false
	var myTrue, myFalse MyBool = true, false

	if myTrue && myFalse {
	}
	if myFalse || myTrue {
	}

	switch myTrue && myFalse {
	case myTrue:
	case myFalse:
	}

	switch nativeTrue && nativeFalse {
	case nativeTrue:
	case nativeFalse:
	}
}
