package instrumenter

// https://go.dev/ref/spec#Constant_declarations
// https://go.dev/ref/spec#Variable_declarations

// TODO: Add systematic tests.

// valueSpec covers the instrumentation of [ast.ValueSpec], which contains the
// expression fields Type (only relevant at compile time) and Values.
func valueSpec() {
	var (
		_ = 1 > 0
		_ = 0 > 1
	)

	// No not instrument constant expressions.
	// Wrapping them with a call to gobcoCover would turn them
	// non-constant.
	const (
		_ = 1 > 0
		_ = 0 > 1
	)
}

// TODO: instrument the initialization of global variables.
