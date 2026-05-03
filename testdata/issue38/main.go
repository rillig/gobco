package issue38

func Abs(a int) uint {
	if a >= 0 {
		return uint(a)
	}
	return -uint(a)
}
