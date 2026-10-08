package filter

import (
	"github.com/rillig/gobco/testdata/exporttest"
	"github.com/rillig/gobco/testdata/exporttest/numbers"
)

func Keep(list numbers.List, keep func(exporttest.Number) bool) numbers.List {
	var kept numbers.List
	for _, n := range list {
		if keep(n) {
			kept = append(kept, n)
		}
	}
	return kept
}
