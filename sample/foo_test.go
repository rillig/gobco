package main

import "testing"

func TestFoo(t *testing.T) {
	if !Foo(9) {
		t.Error("wrong")
	}
}
