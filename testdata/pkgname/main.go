package pkgname

func Exported(cond bool) rune {
	if cond {
		return 'E'
	}
	return 'e'
}

func unexported(cond bool) rune {
	if cond {
		return 'U'
	}
	return 'u'
}
