package pim

import "strings"

// ParseTicket splits a "SYSTEM:NUMBER" ticket reference into its two parts.
// If the input contains no colon, the whole value is treated as the number
// and the system is empty.
func ParseTicket(s string) (system, number string) {
	if s == "" {
		return "", ""
	}
	idx := strings.Index(s, ":")
	if idx < 0 {
		return "", s
	}
	return s[:idx], s[idx+1:]
}
