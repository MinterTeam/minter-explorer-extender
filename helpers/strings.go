package helpers

func RemovePrefix(str string) string {
	strRune := []rune(str)
	return string(strRune[2:])
}
