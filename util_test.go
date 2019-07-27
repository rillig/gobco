package main

import (
	"os"
	"path/filepath"
	"strings"
)

func listRegularFiles(basedir string) []string {
	var files []string

	err := filepath.Walk(basedir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			rel := strings.TrimPrefix(path, basedir)
			slashed := filepath.ToSlash(rel)
			files = append(files, strings.TrimPrefix(slashed, "/"))
		}
		return err
	})

	if err != nil {
		panic(err)
	}

	return files
}
