#! /bin/sh
set -eu

go test -coverprofile=coverage.txt -covermode=count ./...
go test ./testdata/instrumenter

go install

gobco .
gobco ./testdata/instrumenter
gobco -branch ./testdata/instrumenter
