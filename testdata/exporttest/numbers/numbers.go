package numbers

import "github.com/rillig/gobco/testdata/exporttest"

type List []exporttest.Number

func Of(values ...int) List {
	var list List
	for _, value := range values {
		list = append(list, exporttest.New(value))
	}
	return list
}
