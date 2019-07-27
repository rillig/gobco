package main

// This file is wrapped with coverage code when the whole package is
// checked. But when foo.go is mentioned explicitly on the command line,
// only that file is wrapped with coverage.

func isRandom(x int) bool {
	return x == 4
}
