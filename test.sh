#! /bin/sh
set -eu

# Don't test the subpackages with './...' since the templates in ./templates
# are not meaningfully testable on their own.

go test -coverprofile=coverage.txt -covermode=count .
go test ./testdata/instrumenter

go install

gobco .
gobco ./testdata/instrumenter
gobco -test -v ./testdata/testmain | tee out
grep "original" out

go test ./testdata/pkgname
gobco ./testdata/pkgname
gobco -cover-test ./testdata/pkgname
