//go:build gobco
// +build gobco

package constraints

func Sign(x int) string {
	if x > 0 {
		return "positive"
	}
	return "non-positive"
}
