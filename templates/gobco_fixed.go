// This is the fixed part of the gobco code that is injected into the
// package being checked.
//
// Alternatively this code could be provided as a separate go package.
// This would require that this package were installed at run time,
// which is a needless restriction.

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

	defer func() {
		closeErr := file.Close()
		st.check(closeErr)
	}()

	var data []gobcoCond
	decoder := json.NewDecoder(bufio.NewReader(file))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&data)
	st.check(err)

	if len(st.conds) != len(data) {
		msg := fmt.Sprintf(
			"gobco: stats file %q must have exactly %d coverage counters",
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

func (st *gobcoStats) persist(filename string) {
	file, err := os.Create(filename)
	st.check(err)

	defer func() { st.check(file.Close()) }()

	buf := bufio.NewWriter(file)
	defer func() { st.check(buf.Flush()) }()

	encoder := json.NewEncoder(buf)
	encoder.SetIndent("", "\t")
	encoder.SetEscapeHTML(false)
	st.check(encoder.Encode(st.conds))
}

func (st *gobcoStats) cover(idx int, cond bool) bool {
	counts := &st.conds[idx]
	if cond {
		counts.TrueCount++
	} else {
		counts.FalseCount++
	}

	if gobcoOpts.immediately {
		st.persist(st.filename())
	}

	return cond
}

func (st *gobcoStats) finish(exitCode int) int {
	st.persist(st.filename())
	return exitCode
}

type gobcoCond struct {
	Start      string
	Code       string
	TrueCount  int
	FalseCount int
}

// gobcoCover is a top-level function to keep the instrumented code as simple
// as possible.
func gobcoCover(idx int, cond bool) bool {
	return gobcoCounts.cover(idx, cond)
}
