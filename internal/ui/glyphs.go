package ui

// sextantRune maps a 2×3 mask onto a block character.
// Bit 0 is the upper left, then upper right, middle left, middle right,
// lower left, lower right. Masks 21 and 42 are the older half blocks, so
// the U+1FB00 block skips them.
func sextantRune(mask int) rune {
	switch mask {
	case 0:
		return ' '
	case 63:
		return '█'
	case 21:
		return '▌'
	case 42:
		return '▐'
	}
	off := mask - 1
	if mask > 21 {
		off--
	}
	if mask > 42 {
		off--
	}
	return rune(0x1FB00 + off)
}

// octantReused lists the 2×4 patterns Unicode already encoded elsewhere.
// Bit 0 is the top-left octant; the bits then run left-to-right, top-to-bottom.
var octantReused = map[int]rune{
	0:   ' ',
	1:   '\U0001CEA8',
	2:   '\U0001CEAB',
	3:   '\U0001FB82',
	5:   '▘',
	10:  '▝',
	15:  '▀',
	20:  '\U0001FBE6',
	40:  '\U0001FBE7',
	63:  '\U0001FB85',
	64:  '\U0001CEA3',
	80:  '▖',
	85:  '▌',
	90:  '▞',
	95:  '▛',
	128: '\U0001CEA0',
	160: '▗',
	165: '▚',
	170: '▐',
	175: '▜',
	192: '▂',
	240: '▄',
	245: '▙',
	250: '▟',
	252: '▆',
	255: '█',
}

func octantRune(mask int) rune {
	if r, ok := octantReused[mask]; ok {
		return r
	}
	index := mask
	for bits := range octantReused {
		if bits < mask {
			index--
		}
	}
	return rune(0x1CD00 + index)
}

func blockRune(detail BlockDetail, mask int) rune {
	switch detail {
	case BlockSextant:
		return sextantRune(mask)
	case BlockOctant:
		return octantRune(mask)
	default:
		if mask < 0 || mask > 15 {
			return ' '
		}
		return quadrant[mask]
	}
}

// legacyRune is the glyph actually written to the terminal. Sextants live in
// U+1FB00, which DejaVu Sans Mono — Kitty's usual font — does not contain, so
// the cell is drawn blank. The same mask is collapsed onto a quadrant block
// from U+2580, which that font does have. Octants outside the BMP become a
// full block for the same reason.
func legacyRune(r rune) rune {
	if r >= 0x1FB00 && r <= 0x1FB3B {
		return quadrantFromSextant(r)
	}
	if r > 0xFFFF {
		return '█'
	}
	return r
}

func quadrantFromSextant(r rune) rune {
	mask := sextantMask(r)
	// Bits are upper-left, upper-right, middle-left, middle-right, lower-left, lower-right.
	ul, ur := mask&1 != 0, mask&2 != 0
	ml, mr := mask&4 != 0, mask&8 != 0
	ll, lr := mask&16 != 0, mask&32 != 0
	// The middle sample fills a half that would otherwise be empty, so a
	// narrow stem stays narrow instead of becoming a solid brick.
	if !ul && !ll {
		ul, ll = ml, ml
	} else {
		if !ul {
			ul = ml
		}
		if !ll {
			ll = ml
		}
	}
	if !ur && !lr {
		ur, lr = mr, mr
	} else {
		if !ur {
			ur = mr
		}
		if !lr {
			lr = mr
		}
	}
	q := 0
	if ul {
		q |= 1
	}
	if ur {
		q |= 2
	}
	if ll {
		q |= 4
	}
	if lr {
		q |= 8
	}
	if q == 0 {
		return '█'
	}
	return quadrant[q]
}

// sextantMask inverts sextantRune for the U+1FB00 block. Masks 21 and 42 are
// not in that block; they are the legacy half blocks.
func sextantMask(r rune) int {
	off := int(r - 0x1FB00)
	if off < 0 {
		return 0
	}
	if off < 20 {
		return off + 1
	}
	if off < 40 {
		return off + 2
	}
	return off + 3
}
