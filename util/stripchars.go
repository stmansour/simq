package util

import "strings"

// Stripchars removes the characters from chr in str and returns the updated string.
//
// INPUTS:
//
//	str = string to strip characters from
//	chr = characters to remove
//
// RETURNS:
//
//	the stripped string
func Stripchars(str, chr string) string {
	return strings.Map(func(r rune) rune {
		if !strings.ContainsRune(chr, r) {
			return r
		}
		return -1
	}, str)
}
