package instrumenter

// https://go.dev/ref/spec#Pointer_types
// https://go.dev/ref/spec#Address_operators

// TODO: Add systematic tests.

// starExpr covers the instrumentation of [ast.StarExpr], which has the
// branch statement.
func starExpr() {
	m := map[bool]*int{}

	_ = *m[func(b bool) bool {
		if !b {
			return true
		} else {
			return false
		}
	}(true)]
}
