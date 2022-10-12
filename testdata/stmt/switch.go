package stmt

// switchInit tests instrumenter.visitSwitch.
func switchInit(s string) int {
	switch s := "prefix" + s; s + "suffix" {
	case "prefix.a.suffix":
		return 1
	}
	return 0
}
