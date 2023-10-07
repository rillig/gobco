//go:build ignore
// +build ignore

// This is the variable part of the gobco code that is injected into the
// package being checked.
//
// It is kept as minimal and maintainable as possible.
//
// It serves as a template to be used in instrumenter.writeGobcoGo.

package main

var gobcoOpts = gobcoOptions{
	immediately: true,
	listAll:     true,
}

var gobcoCounts = gobcoStats{
	conds: []gobcoCond{
		{
			Start:      "code.go:5:2",
			Code:       "i > 0",
			TrueCount:  0,
			FalseCount: 0,
		},
	},
}
