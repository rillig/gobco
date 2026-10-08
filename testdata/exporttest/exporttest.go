package exporttest

type Number struct {
	value int
}

func New(value int) Number {
	return Number{value}
}

func (n Number) isEven() bool {
	return n.value%2 == 0
}
