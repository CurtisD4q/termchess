package ui

import (
	"sync"

	"github.com/notnil/chess"
)

// Quadrant masks. Bit 0 is the upper left, then upper right, lower left, lower right.
var quadrant = [16]rune{
	' ', '▘', '▝', '▀',
	'▖', '▌', '▞', '▛',
	'▗', '▚', '▐', '▜',
	'▄', '▙', '▟', '█',
}

type artCell struct {
	ch     rune
	fg, bg uint8 // 0 = square colour, 1 = white ink, 2 = black ink
}

type artKey struct {
	pt      chess.PieceType
	c       chess.Color
	w, h    int
	mono    bool
	outline bool
	detail  BlockDetail
}

var artMu sync.Mutex
var artMemo = map[artKey][][]artCell{}

func artGrid(c chess.Color, pt chess.PieceType, w, h int, mono, outline bool, detail BlockDetail) [][]artCell {
	if w < 1 || h < 1 || pt < chess.King || pt > chess.Pawn {
		return nil
	}
	key := artKey{pt, c, w, h, mono, outline, detail}
	artMu.Lock()
	defer artMu.Unlock()
	if g, ok := artMemo[key]; ok {
		return g
	}
	g := buildArt(c, pt, w, h, mono, outline, detail)
	artMemo[key] = g
	return g
}

// buildArt paints a hand-tuned silhouette. '#' is the piece and '.' is the
// square. Colour pieces are one solid ink — the same picture on every square.
// outline is ignored there: a light square, a dark square, and a highlight
// all show the same fill, with no crust and no interior cut. High contrast
// still draws white as a rim and black as the fill, because that theme has
// no second colour.
func buildArt(c chess.Color, pt chess.PieceType, cellW, cellH int, mono, outline bool, detail BlockDetail) [][]artCell {
	_ = outline
	rows := maskRows(pt, detail, cellW, cellH)
	subW, subH := detail.subs()
	if len(rows) == 0 || subW < 1 || subH < 1 {
		return nil
	}
	filled := func(x, y int) bool {
		if y < 0 || y >= len(rows) || x < 0 || x >= len(rows[y]) {
			return false
		}
		return rows[y][x] == '#'
	}
	// The rim exists only for a colorless white piece. Crown gaps, merlons,
	// and the chin notch stay empty because the piece continues on both sides.
	showRim := mono && c == chess.White
	toward := func(x, y, dx, dy int) bool {
		for i := 1; i <= 3; i++ {
			if filled(x+dx*i, y+dy*i) {
				return true
			}
		}
		return false
	}
	rimAt := func(x, y int) bool {
		if !showRim || filled(x, y) {
			return false
		}
		if (toward(x, y, -1, 0) && toward(x, y, 1, 0)) || (toward(x, y, 0, -1) && toward(x, y, 0, 1)) {
			return false
		}
		return filled(x-1, y) || filled(x+1, y) || filled(x, y-1) || filled(x, y+1)
	}
	fillFG := uint8(2)
	if c == chess.White {
		fillFG = 1
	}

	grid := make([][]artCell, cellH)
	for cy := 0; cy < cellH; cy++ {
		grid[cy] = make([]artCell, cellW)
		for cx := 0; cx < cellW; cx++ {
			fillMask, rimMask := 0, 0
			for sy := 0; sy < subH; sy++ {
				for sx := 0; sx < subW; sx++ {
					x := cx*subW + sx
					y := cy*subH + sy
					bit := 1 << uint(sy*subW+sx)
					if filled(x, y) {
						fillMask |= bit
					} else if rimAt(x, y) {
						rimMask |= bit
					}
				}
			}
			use := fillMask
			fg := fillFG
			if mono && c == chess.White {
				use = rimMask
				fg = 0
			} else if mono {
				fg = 0
			}
			cell := artCell{ch: ' '}
			if use != 0 {
				cell.ch = blockRune(detail, use)
				if !mono {
					cell.fg = fg
				}
			}
			grid[cy][cx] = cell
		}
	}
	return grid
}
