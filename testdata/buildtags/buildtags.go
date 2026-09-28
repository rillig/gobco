//go:build gobco
// +build gobco

// Like many integration tests, this package is only built with a build tag,
// in this case with 'go test -tags gobco'.

package buildtags

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
