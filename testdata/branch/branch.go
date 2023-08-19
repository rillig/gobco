package branch

// Branches demonstrates the difference between the default condition coverage
// mode and branch coverage mode.
func Branches(x int) int {
	if x > 0 && x > 100 {
		return x + 3
	}
	switch x {
	case 100:
		return 100
	case 15, 30, 40:
		return 'T'
	}
	return 0
}
