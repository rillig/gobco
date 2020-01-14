[![Build Status](https://travis-ci.com/rillig/gobco.svg?branch=master)](https://travis-ci.com/rillig/gobco)
[![codecov](https://codecov.io/gh/rillig/gobco/branch/master/graph/badge.svg)](https://codecov.io/gh/rillig/gobco)

# GOBCO - Golang Branch Coverage

Branch coverage measurement tool for golang.

It is intended to be used in addition to `go test -cover`,
rather than replacing it.
For example, gobco doesn't detect when a whole function or method is 
not covered by tests.
Gobco also doesn't cover `select` statements.

## Installation

```text
$ go get github.com/rillig/gobco
```

## Usage

```text
$ gobco sample/foo.go
--- FAIL: TestFoo (0.00s)
    foo_test.go:7: wrong
FAIL
Branch coverage: 5/6
sample/foo.go:10:5: condition "Bar(a) == 10" was never true
exit status 1
FAIL    github.com/junhwi/gobco/sample  0.112s
exit status 1
```

Running gobco on package [netbsd.org/pkglint](https://github.com/rillig/pkglint):

```text
$ gobco
=== RUN   Test
OK: 756 passed
--- PASS: Test (16.56s)
PASS
Branch coverage: 5452/6046
alternatives.go:28:32: condition "G.Pkg.vars.Defined(\"ALTERNATIVES_SRC\")" was 11 times false but never true
autofix.go:98:6: condition "rawLine.Lineno != 0" was 245 times true but never false
autofix.go:390:12: condition "fix.line.firstLine >= 1" was 392 times true but never false
buildlink3.go:22:5: condition "trace.Tracing" was 19 times true but never false
...
substcontext.go:136:22: condition "(value == \"pre-configure\" || value == \"post-configure\")" was once true but never false
substcontext.go:136:23: condition "value == \"pre-configure\"" was once true but never false
substcontext.go:136:51: condition "value == \"post-configure\"" was never evaluated
```

## Adding custom test conditions

To ensure that a certain condition is covered by the tests, you can insert
the desired condition into the code and just assign them to the underscore:

~~~go
func square(x int) int {
    _ = x > 50
    _ = x == 0
    _ = x < 0

    return x * x
}
~~~

Since the above conditions are simple, they are optimized away by the compiler.
In the source code they are clearly identified as boolean expressions, therefore
gobco inserts its coverage code around them.

Note that for boolean expressions that don't clearly look like boolean
expressions, you have to write `cond == true` instead of a simple `cond` since
as of December 2019, gobco only analyzes the code at the syntactical level,
without doing any type resolution.
