package main

import (
  "os"
  "testing"
)

func TestMain(m *testing.M) {
  retCode := m.Run()
  PrintCoverage()
  os.Exit(retCode)
}

func TestFoo(t *testing.T) {
  if !Foo(9) {
    t.Error("wrong")
  }
}
