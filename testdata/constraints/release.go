//go:build go1.16
// +build go1.16

package constraints

// Files can also be constrained on the release of the go toolchain.
func IsZero(x int) bool {
	return x == 0
}
