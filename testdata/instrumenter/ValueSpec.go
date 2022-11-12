package instrumenter

// https://go.dev/ref/spec#Constant_declarations
// https://go.dev/ref/spec#Variable_declarations

// TODO: Add systematic tests.

// valueSpec covers the instrumentation of [ast.ValueSpec], which contains the
// expression fields Type (only relevant at compile time) and Values.
func valueSpec() {
}

// TODO: instrument the initialization of global variables.
