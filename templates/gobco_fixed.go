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
	firstTime   bool
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
	encoder.Encode(st.conds)
}

func (st *gobcoStats) cover(idx int, cond bool) bool {
	counts := &st.conds[idx]
	if cond {
		if gobcoOpts.firstTime && counts.TrueCount == 0 {
			fmt.Fprintf(os.Stderr, "%s: condition %q is true for the first time.\n", counts.Start, counts.Code)
		}
		counts.TrueCount++
	} else {
		if gobcoOpts.firstTime && counts.FalseCount == 0 {
			fmt.Fprintf(os.Stderr, "%s: condition %q is false for the first time.\n", counts.Start, counts.Code)
		}
		counts.FalseCount++
	}

	if gobcoOpts.immediately {
		st.persist(st.filename())
	}

	return cond
}

type gobcoCond struct {
	Start      string
	Code       string
	TrueCount  int
	FalseCount int
}

func (st *gobcoStats) printCoverage() {
	cnt := 0
	for _, c := range st.conds {
		if c.TrueCount > 0 {
			cnt++
		}
		if c.FalseCount > 0 {
			cnt++
		}
	}
	fmt.Printf("Branch coverage: %d/%d\n", cnt, len(st.conds)*2)

	for _, cond := range st.conds {
		st.printCond(cond)
	}
}

func (st *gobcoStats) printCond(cond gobcoCond) {
	switch {
	case cond.TrueCount == 0 && cond.FalseCount > 1:
		fmt.Printf("%s: condition %q was %d times false but never true\n", cond.Start, cond.Code, cond.FalseCount)
	case cond.TrueCount == 0 && cond.FalseCount == 1:
		fmt.Printf("%s: condition %q was once false but never true\n", cond.Start, cond.Code)

	case cond.FalseCount == 0 && cond.TrueCount > 1:
		fmt.Printf("%s: condition %q was %d times true but never false\n", cond.Start, cond.Code, cond.TrueCount)
	case cond.FalseCount == 0 && cond.TrueCount == 1:
		fmt.Printf("%s: condition %q was once true but never false\n", cond.Start, cond.Code)

	case cond.TrueCount == 0 && cond.FalseCount == 0:
		fmt.Printf("%s: condition %q was never evaluated\n", cond.Start, cond.Code)

	default:
		if gobcoOpts.listAll {
			fmt.Printf("%s: condition %q was %d times true and %d times false\n",
				cond.Start, cond.Code, cond.TrueCount, cond.FalseCount)
		}
	}
}

// gobcoCover is a separate function to keep the code generation small and simple.
// It's probably easy to adjust the code generation in instrumenter.wrap.
func gobcoCover(idx int, cond bool) bool {
	return gobcoCounts.cover(idx, cond)
}
