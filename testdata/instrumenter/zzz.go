package instrumenter

import (
	"fmt"
	"reflect"
)

func assertEquals(actual, expected interface{}) {
	if !reflect.DeepEqual(actual, expected) {
		panic(fmt.Sprintf("assertion failed: %v != %v", actual, expected))
	}
}
