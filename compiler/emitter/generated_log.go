package emitter

import "fmt"

// VerboseGenerated makes emitters print a line for every file they write. A
// build writes about two thousand files on a real project, and the list buries
// the diagnostics and the result, so it is off unless ang build --verbose.
var VerboseGenerated bool

func logGenerated(format string, args ...any) {
	if VerboseGenerated {
		fmt.Printf(format, args...)
	}
}
