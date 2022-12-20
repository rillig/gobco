#! /bin/sh
set -eu

go test -coverprofile=coverage.txt -covermode=count .
go test ./testdata/instrumenter

go install

gobco .
gobco ./testdata/instrumenter
gobco -test -v ./testdata/testmain | tee out
grep "original" out
