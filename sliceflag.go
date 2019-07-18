package main

import "strings"

type sliceFlag struct {
	values *[]string
}

func newSliceFlag(values *[]string) *sliceFlag {
	return &sliceFlag{values}
}

func (s *sliceFlag) String() string {
	if s.values == nil {
		return ""
	}
	return strings.Join(*s.values, ", ")
}

func (s *sliceFlag) Set(str string) error {
	*s.values = append(*s.values, str)
	return nil
}
