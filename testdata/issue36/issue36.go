package issue36

import "fmt"

func demo(f func(int) []string) bool {
	// Printing a function closure instead of calling the function
	// is allowed but discouraged. A normal "go build" succeeds,
	// but a "go test" with its implicit "go vet" will complain:
	//
	//	issue36.go:18:23: fmt.Sprint arg f is a func value, not called
	//
	// Gobco is only intended to be run on code that passes "go test",
	// so there is nothing to be done here.
	//
	// As a workaround, gobco can be instructed to skip "go vet":
	//
	//	gobco -test -vet=off ./testdata/issue36/
	return fmt.Sprint(f) == ""
}
