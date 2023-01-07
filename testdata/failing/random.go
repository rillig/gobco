package main

// This file is instrumented when the whole package is checked.
// But when fail.go is mentioned explicitly on the command line,
// only that file is instrumented.

func isRandom(x int) bool {
	return x == 4
}
