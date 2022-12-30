#! /bin/sh
set -eu

# Don't test the subpackages with './...' since the templates in ./templates
# are not meaningfully testable on their own.

go test -coverprofile=coverage.txt -covermode=count .
go test ./testdata/instrumenter

go install

gobco .
gobco ./testdata/instrumenter
