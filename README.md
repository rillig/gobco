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

With go1.16:

```text
$ go get github.com/rillig/gobco
```

Older go releases are not supported.

## Usage

To run gobco on a single package, run it in the package directory:

~~~text
$ gobco
~~~

The output typically looks like the following example, taken from package
[netbsd.org/pkglint](https://github.com/rillig/pkglint):

```text
ok  	netbsd.org/pkglint	23.454s

Condition coverage: 9788/9866
category.go:188:7: condition "value.IsAbs()" was 354 times false but never true
distinfo.go:265:8: condition "alg == \"SHA1\"" was 16 times false but never true
distinfo.go:319:13: condition "remainingHashes[0].algorithm == alg" was 8 times true but never false
...
mkcondsimplifier.go:142:38: condition "p[2] == p[1]-'a'+'A'" was 9 times true but never false
...
vartypecheck.go:1010:11: condition "len(invalid) > 1" was once false but never true
vartypecheck.go:1613:42: condition "cv.MkLines.pkg != nil" was 8 times true but never false
vartypecheck.go:1615:6: condition "distname.IsConstant()" was 8 times true but never false
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
