package exporttest

// In an export test, the package name is the same as in the main code,
// but the file is only compiled into the test.
// It makes some of the unexported members available to the black box test.

func IsEven(n Number) bool {
	return n.isEven()
}
