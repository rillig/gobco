# GOBCO - Golang Branch Coverage

Branch coverage measurement tool for golang.

## Install and Usage
```sh
$ go get github.com/junhwi/gobco
$ ./gobco sample/foo.go
--- FAIL: TestFoo (0.00s)
  foo_test.go:16: wrong
FAIL
Coverage: 5 / 6
exit status 1
FAIL  go-cov/sample 0.008s
```
