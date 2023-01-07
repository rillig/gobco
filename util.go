package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func copyDir(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	err := os.MkdirAll(dst, 0o777)
	if err != nil {
		return err
	}

	action := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			dstPath := filepath.Join(dst, rel)
			err = os.MkdirAll(filepath.Dir(dstPath), os.ModePerm)
			if err == nil {
				err = copyFile(path, dstPath)
			}
		}
		return err
	}

	return filepath.Walk(src, action)
}

func copyFile(src string, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer func() {
		closeErr := in.Close()
		if err == nil {
			err = closeErr
		}
	}()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		closeErr := out.Close()
		if err == nil {
			err = closeErr
		}
	}()

	_, err = io.Copy(out, in)
	return
}

func randomHex(n int) string {
	rnd := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, rnd[:])
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", rnd)
}

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

func ok(err error) {
	if err != nil {
		panic(err)
	}
}

func assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}
