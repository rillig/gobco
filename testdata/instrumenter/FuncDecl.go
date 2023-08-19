package instrumenter

// https://go.dev/ref/spec#Function_declarations

// TODO: Add systematic tests.

// funcDecl covers the instrumentation of [ast.FuncDecl], which has no
// expression fields.
//
// Function declarations are not instrumented themselves.
func funcDecl() {

	// When this switch statement is instrumented, gobco saves the tag
	// expression in a temporary variable with a generated name that
	// is unlikely to conflict with any actually used variable.
	switch 1 > 0 {
	}
}

func funcDecl2() {
	// The names of the temporary variables are unique per top-level
	// function declaration.
	switch 2 > 0 {
	default:
		// Nested functions are not FuncDecl but instead FuncLiteral,
		// so the counter for variable names is not reset here.
		_ = func() {
			switch 3 > 0 {
			}
		}
	}
}
