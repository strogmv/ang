package main

import "strings"

// titleWords upper-cases the first letter of every space-separated word
// ("patch apply" -> "Patch Apply"); it replaces the deprecated strings.Title
// for the ASCII command names it is used on.
func titleWords(s string) string {
	words := strings.Split(s, " ")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
