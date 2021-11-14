[![Build Status](https://app.travis-ci.com/rillig/gobco.svg?branch=master)](https://app.travis-ci.com/github/rillig/gobco)
[![codecov](https://codecov.io/gh/rillig/gobco/branch/master/graph/badge.svg)](https://codecov.io/gh/rillig/gobco)

# GOBCO - Golang Branch Coverage

Gobco measures branch coverage of Go code.

Gobco should be used in addition to `go test -cover`,
rather than replacing it.
For example, gobco does not detect functions or methods that are completely
unused, it only notices them if they contain any conditions or branches.
Gobco also doesn't cover `select` statements.

## Installation

With go1.17 or later:

```text
$ go install github.com/rillig/gobco@latest
```

With go1.16 or older:

```text
$ go get github.com/rillig/gobco
```

## Usage

To run gobco on a single package, run it in the package directory:

~~~text
$ gobco
~~~

The output typically looks like the following example, taken from package
[netbsd.org/pkglint](https://github.com/rillig/pkglint):

```text
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

Even if some tests still fail, gobco can compute the code coverage: 

```text
$ gobco sample/foo.go
--- FAIL: TestFoo (0.00s)
    foo_test.go:7: wrong
FAIL
FAIL    github.com/rillig/gobco/sample  0.315s
FAIL
exit status 1

Branch coverage: 5/6
sample\foo.go:10:5: condition "Bar(a) == 10" was once false but never true
```

## Adding custom test conditions

If you want to ensure that a certain condition in your code is covered by the
tests, you can insert the desired condition into the code and just assign it
to the underscore:

~~~go
func square(x int) int {
    _ = x > 50
    _ = x == 0
    _ = x < 0

    return x * x
}
~~~

The compiler will see that these conditions are side-effect-free and will thus
optimize them away, so there is no runtime overhead.

Since the above conditions are syntactically recognizable as boolean 
expressions, gobco inserts its coverage code around them.

Note that for boolean expressions that don't clearly look like boolean
expressions, you have to write `cond == true` instead of a simple `cond` since
as of March 2021, gobco only analyzes the code at the syntactical level,
without resolving any types.
