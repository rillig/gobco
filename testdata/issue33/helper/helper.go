// Package helper provides an indirection used by the issue #33
// regression test, so that no file in the target package's directory
// directly imports the target package by its real import path.
package helper

import "github.com/rillig/gobco/testdata/issue33/target"

func AddViaTarget(a, b int) int {
	return target.New().Add(a, b)
}
