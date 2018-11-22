# GOBCO - Golang Branch Coverage

Branch coverage measurement tool for golang.

## Install and Usage
```text
$ go get github.com/junhwi/gobco
$ ./gobco sample/foo.go
$ gobco sample/foo.go
--- FAIL: TestFoo (0.00s)
    foo_test.go:7: wrong
FAIL
Branch coverage: 5/6
sample/foo.go:10:5: branch "Bar(a) == 10" was never true
exit status 1
FAIL    github.com/junhwi/gobco/sample  0.112s
exit status 1
```
