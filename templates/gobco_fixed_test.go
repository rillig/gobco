// This is the fixed part of the gobco code that is injected into the
// package being checked.

package templates

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
	gobcoCounts.load()
	exitCode := m.Run()
	gobcoCounts.persist()
	return exitCode
}
