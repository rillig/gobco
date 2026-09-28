package constraints

// The go command ignores files whose names start with '_' or '.'.

func Sign(x int) string {
	if x == 0 {
		return "zero"
	}
	return "draft"
}
