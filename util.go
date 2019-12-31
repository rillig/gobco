package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// userHomeDir returns the home directory of the current user.
// Copied from go1.13.
// TODO: Remove this function again once go1.13 is considered "old enough".
func userHomeDir() (string, error) {
	env, enverr := "HOME", "$HOME"
	switch runtime.GOOS {
	case "windows":
		env, enverr = "USERPROFILE", "%userprofile%"
	case "plan9":
		env, enverr = "home", "$home"
	case "nacl", "android":
		return "/", nil
	case "darwin":
		if runtime.GOARCH == "arm" || runtime.GOARCH == "arm64" {
			return "/", nil
		}
	}
	if v := os.Getenv(env); v != "" {
		return v, nil
	}
	return "", errors.New(enverr + " is not defined")
}

func copyDir(src string, dst string) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	err = os.MkdirAll(dst, 0777)
	if err != nil {
		return
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
