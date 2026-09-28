//go:build !gobco
// +build !gobco

package constraints

// Sign is declared in two files, like a function that is implemented once
// per platform. A plain 'go test' builds this file, while
// 'go test -tags gobco' builds tagged.go instead.
func Sign(x int) string {
	if x < 0 {
		return "negative"
	}
	return "non-negative"
}
