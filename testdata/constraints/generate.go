//go:build ignore
// +build ignore

// This file does not belong to the package, it is only run by
// 'go generate' or 'go run generate.go'.

package main

import "os"

func main() {
	if len(os.Args) > 1 {
		os.Exit(1)
	}
}
