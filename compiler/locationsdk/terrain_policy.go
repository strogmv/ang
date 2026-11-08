package locationsdk

import "strings"

// WalkableTerrainGlyphs is the movement policy used by the generated SDK host.
// Stair nodes also use '.', in their own nav band. '+' is sight-clear solid
// material (for example a low counter), never a traversable cell.
func WalkableTerrainGlyphs() []string { return strings.Split(walkableTerrainGlyphs, "") }

const walkableTerrainGlyphs = "."

func IsWalkableTerrainGlyph(glyph rune) bool {
	return strings.ContainsRune(walkableTerrainGlyphs, glyph)
}
