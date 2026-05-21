// Package target intentionally shares its name
// with github.com/rillig/gobco/testdata/issue33/target,
// to reproduce the import-resolution bug from issue #33.
package target

type Adder interface {
	Add(a, b int) int
}
