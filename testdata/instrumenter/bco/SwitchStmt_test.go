package instrumenter

import "testing"

func TestSwitchInit(t *testing.T) {

	// TODO: Actually test that instrumenting the switch statements
	//  preserves the behavior of the code, especially in tricky
	//  cases of implicit scopes.

	switchStmt(7, false, ".a.")
	switchStmt(9, true, ".b.")
}
