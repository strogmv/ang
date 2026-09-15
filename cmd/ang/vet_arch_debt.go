package main

import "strings"

// listOrNone joins names for a one-line report, or says "none".
func listOrNone(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}
