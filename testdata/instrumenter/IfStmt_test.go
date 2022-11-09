package instrumenter

import "testing"

func Test_ifStmt(t *testing.T) {
	ifStmt(0, "", false)
	ifStmt(0, "123456", false)
	ifStmt(0, "12345678901", false)
	ifStmt(1, "positive", false)
	ifStmt(1, "positive", true)
	ifStmt(1, "", true)
}
