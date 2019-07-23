// This is the fixed part of the gobco code that is injected into the
// package being checked.

package main

import "testing"

// gobcoTestingM provides a hook to run callbacks before or after the
// actual testing.M.Run is called.
//
// This seemed the easiest way to hook into an arbitrary go program
// for persisting the coverage data just before exiting.
type gobcoTestingM struct {
	m *testing.M
}

func (m gobcoTestingM) Run() int {
	filename := gobcoCounts.filename()
	gobcoCounts.load(filename)
	defer gobcoCounts.persist(filename)

	return m.Run()
}
