package instrumenter

import "testing"

func TestSwitchInit(t *testing.T) {
	switchStmt(7, false, ".a.")
	switchStmt(9, true, ".b.")
}
