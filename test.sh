#! /bin/sh
set -eu

# Don't test the subpackages with './...' since the templates in ./templates
# are not meaningfully testable on their own.

go test -coverprofile=coverage.txt -covermode=count .
go test ./testdata/instrumenter/coco

go install

gobco .
gobco ./testdata/instrumenter/coco

# To verify the branch coverage instrumentation
gobco -want-c1 ./testdata/instrumenter/bco
