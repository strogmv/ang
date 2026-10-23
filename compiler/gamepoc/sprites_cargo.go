package gamepoc

import "image/color"

// cargoColor keeps old CUE recipes compatible while allowing explicit material
// roles. All fallback colors derive from authored palette tokens.
func (c *canvas) cargoColor(role string, fallback color.NRGBA) color.NRGBA {
	if key, exists := c.roles[role]; exists {
		return c.palette[key]
	}
	return fallback
}

// cargoFace decorates the existing prism without changing a single silhouette
// pixel. It is deliberately a readable greybox: monolithic composite panels,
// gasket, corner couplings and a recessed handling/identification plate.
// The little code marks are cosmetic, not an inventory serial or status signal.
func (c *canvas) cargoFace(reg region, u, v, hw, height, variant int, base color.NRGBA) color.NRGBA {
	seam := c.cargoColor("ink", c.cargoColor("slat", shade(c.col("right"), .55)))
	frame := c.cargoColor("trim", shade(c.col("top"), 1.18))
	label := c.cargoColor("label", shade(c.col("top"), 1.40))
	lock := c.cargoColor("lock", mix(frame, label, .3))
	if reg == top {
		// Transform the 2:1 top diamond to local panel coordinates. One continuous
		// lid and its perimeter seal replace the old diagonal board/grain pattern.
		a := (u + 2*v - hw) / 2
		b := (2*v - u + hw) / 2
		if a < 2 || b < 2 || a >= hw-2 || b >= hw-2 {
			return shade(frame, .85)
		}
		if a == 3 || b == 3 || a == hw-4 || b == hw-4 {
			return seam
		}
		if (a >= 5 && a <= 8 || a >= hw-9 && a <= hw-6) && (b >= 5 && b <= 8 || b >= hw-9 && b <= hw-6) {
			if a%3 == 0 && b%3 == 0 {
				return seam
			}
			return lock
		}
		// A single recessed label sits on the lid. Sparse marks follow the same
		// isometric axes; no world-space horizontal stripes suggest wooden slats.
		if a >= hw/2-5 && a <= hw/2+6 && b >= hw/2-4 && b <= hw/2+3 {
			if a == hw/2-5 || a == hw/2+6 || b == hw/2-4 || b == hw/2+3 {
				return frame
			}
			if b == hw/2-2 && a%3 != 0 || b >= hw/2 && a%2 == variant%2 {
				return label
			}
			return seam
		}
		return base
	}
	if height <= 0 {
		return base
	}
	// One top rail, one base rail, and three vertical structural corners. There
	// are no repeated horizontal seams or wooden cross-braces.
	if v < 2 || v >= height-2 {
		return shade(frame, .76)
	}
	if u < 2 || u >= hw-2 {
		return shade(frame, .90)
	}
	if v == 3 || v == height-4 || u == 3 || u == hw-4 {
		return seam
	}
	// Couplings occupy only the panel corners and remain visible at 1x scale.
	nearCorner := u >= 4 && u <= 7 || u >= hw-8 && u <= hw-5
	if nearCorner && (v >= 4 && v <= 6 || v >= height-7 && v <= height-5) {
		if (u == 5 || u == hw-6) && (v == 5 || v == height-6) {
			return seam
		}
		return lock
	}
	// Two small inset handholds, scaled with the established block height.
	if v == height/3 && u >= hw/2-4 && u <= hw/2+4 {
		return seam
	}
	if v == height/3+1 && u >= hw/2-3 && u <= hw/2+3 {
		return shade(frame, .78)
	}
	labelTop := max(7, height/2)
	if u >= hw/2-5 && u <= hw/2+6 && v >= labelTop && v <= labelTop+4 && v < height-4 {
		if v == labelTop || v == labelTop+4 || u == hw/2-5 || u == hw/2+6 {
			return seam
		}
		if v == labelTop+1 && (u+variant)%3 != 0 || v == labelTop+3 && (u+variant)%2 == 0 {
			return label
		}
		return shade(seam, .82)
	}
	// The alternate cassette is used, with a few edge scuffs rather than rust
	// covering the entire face. Damage does not create a different collision.
	if variant%2 == 1 && (u == 4 || u == hw-5 || v == height-5) && hash3(uint32(u), uint32(v), c.seed)%5 == 0 {
		return shade(frame, 1.12)
	}
	return base
}
