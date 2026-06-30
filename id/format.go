package id

import "strconv"

// Format formats a numeric ID as a decimal string.
func Format(value uint64) string {
	return strconv.FormatUint(value, 10)
}
