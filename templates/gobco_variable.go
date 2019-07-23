// This is the variable part of the gobco code that is injected into the
// package being checked.
//
// It is kept as minimal and maintainable as possible.

package main

var gobcoOpts = gobcoOptions{
	firstTime:   true,
	immediately: true,
	listAll:     true,
}

var gobcoCounts = gobcoStats{
	conds: []gobcoCond{},
}
