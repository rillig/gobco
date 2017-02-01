package main

func Foo(a int) bool {
	for i := 0; i < 10; i++ {
		a += i
	}
	for a < 1000 {
		a += a
	}
	if Bar(a) == 10 {
		return true
	} else {
		return false
	}
}

func Bar(a int) int {
	return a + 1
}
