// This is the variable part of the gobco code that is injected into the
// package being checked.
//
// It is kept as minimal and maintainable as possible.

package templates

var gobcoOpts = gobcoOptions{true, true, true}

var gobcoCounts = newGobcoStats()

func main() {
	defer gobcoCounts.persist()
}
