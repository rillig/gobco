# GOBCO - Golang Branch Coverage

Branch coverage measurement tool for golang.

## Installation

```text
$ go get github.com/junhwi/gobco
$ go install github.com/junhwi/gobco
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

Running gobco on package netbsd.org/pkglint:

```text
$ gobco
=== RUN   Test
OK: 756 passed
--- PASS: Test (16.56s)
PASS
Branch coverage: 5452/6046
alternatives.go:28:32: condition "G.Pkg.vars.Defined(\"ALTERNATIVES_SRC\")" was 11 times false but never true
autofix.go:98:6: condition "rawLine.Lineno != 0" was 245 times true but never false
autofix.go:124:6: condition "rawLine.Lineno != 0" was 44 times true but never false
autofix.go:270:7: condition "fix.diagFormat == AutofixFormat" was 198 times false but never true
autofix.go:295:51: condition "len(fix.explanation) == 0" was 3 times false but never true
autofix.go:311:36: condition "mkline.IsCommentedVarassign()" was 8 times true but never false
autofix.go:324:6: condition "m" was 36 times true but never false
autofix.go:332:59: condition "rawLine.textnl != \"\\n\"" was 18 times true but never false
autofix.go:357:6: condition "replaced != rawLine.textnl" was 22 times true but never false
autofix.go:367:5: condition "G.Testing" was 389 times true but never false
autofix.go:374:12: condition "fix.diagFormat == \"\"" was 387 times true but never false
autofix.go:390:12: condition "fix.line.firstLine >= 1" was 392 times true but never false
buildlink3.go:22:5: condition "trace.Tracing" was 19 times true but never false
...
substcontext.go:136:22: condition "(value == \"pre-configure\" || value == \"post-configure\")" was once true but never false
substcontext.go:136:23: condition "value == \"pre-configure\"" was once true but never false
substcontext.go:136:51: condition "value == \"post-configure\"" was never evaluated
```
