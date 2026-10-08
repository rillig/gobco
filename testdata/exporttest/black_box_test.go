package exporttest_test

// The black box test imports two packages that depend on the package
// under test before it imports the package under test itself.
import "github.com/rillig/gobco/testdata/exporttest/filter"
import "github.com/rillig/gobco/testdata/exporttest/numbers"
import "github.com/rillig/gobco/testdata/exporttest"
import "testing"

// IsEven is only declared in export_test.go.
// The list from the package numbers is passed to the package filter,
// which imports the package numbers as well.

func TestKeep(t *testing.T) {
	evens := filter.Keep(numbers.Of(1, 2, 3, 4), exporttest.IsEven)
	if len(evens) != 2 {
		t.Fail()
	}
}
