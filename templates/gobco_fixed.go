// This is the fixed part of the gobco code that is injected into the
// package being checked.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type gobcoOptions struct {
	immediately bool
	listAll     bool
}

type gobcoStats struct {
	conds []gobcoCond
}

type gobcoCond struct {
	Start      string
	Code       string
	TrueCount  int
	FalseCount int
}

func (st *gobcoStats) filename() string {
	filename := os.Getenv("GOBCO_STATS")
	if filename == "" {
		panic("gobco: GOBCO_STATS environment variable must be set")
	}
	return filename
}

func (st *gobcoStats) check(err error) {
	if err != nil {
		panic(err)
	}
}

func (st *gobcoStats) load(filename string) {
	file, err := os.Open(filename)
	if err != nil && os.IsNotExist(err) {
		return
	}

	defer func() { st.check(file.Close()) }()

	var data []gobcoCond
	decoder := json.NewDecoder(bufio.NewReader(file))
	decoder.DisallowUnknownFields()
	st.check(decoder.Decode(&data))

	if len(st.conds) != len(data) {
		msg := fmt.Sprintf(
			"gobco: stats file '%s' must have exactly %d coverage counters",
			filename, len(st.conds))
		panic(msg)
	}
	st.conds = data
}

func (st *gobcoStats) merge(other *gobcoStats) {
	type key struct {
		start string
		code  string
	}

	m := make(map[key]*gobcoCond)
	for i, cond := range st.conds {
		m[key{cond.Start, cond.Code}] = &st.conds[i]
	}

	for i := range other.conds {
		datum := &other.conds[i]
		cond := m[key{datum.Start, datum.Code}]
		datum.TrueCount += cond.TrueCount
		datum.FalseCount += cond.FalseCount
	}
}

func (st *gobcoStats) persist() {
	// TODO: First write to a temporary file.
	file, err := os.Create(st.filename())
	st.check(err)

	defer func() { st.check(file.Close()) }()

	buf := bufio.NewWriter(file)

	encoder := json.NewEncoder(buf)
	encoder.SetIndent("", "\t")
	encoder.SetEscapeHTML(false)
	st.check(encoder.Encode(st.conds))
	st.check(buf.Flush())
}

func (st *gobcoStats) cover(idx int, cond bool) bool {
	counts := &st.conds[idx]
	if cond {
		counts.TrueCount++
	} else {
		counts.FalseCount++
	}

	if gobcoOpts.immediately {
		st.persist()
	}

	return cond
}

func (st *gobcoStats) finish(exitCode int) int {
	st.persist()
	return exitCode
}

// GobcoCover is a top-level function to keep the instrumented code as simple
// as possible.
func GobcoCover(idx int, cond bool) bool {
	return gobcoCounts.cover(idx, cond)
}
