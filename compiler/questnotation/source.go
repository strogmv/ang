package questnotation

import _ "embed"

// Source is the same formatter emitted into the standalone game server.
//
//go:embed notation.go
var Source string
