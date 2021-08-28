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
	conds: []gobcoCond{},
}
