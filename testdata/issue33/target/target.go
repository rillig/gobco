package target

type T struct{}

func (t *T) Add(a, b int) int {
	if a > 0 {
		return a + b
	}
	return b
}
