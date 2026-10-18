package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/notnil/chess"
)

// Every square is 8×4, a step down from the old 9×4 large size. Classic
// pieces are block sprites. Silhouettes use the same cell. Neither a Kitty
// terminal nor an image protocol replaces those sprites.
const (
	sizeW, sizeH = 8, 4

	whiteInk  = "#FFFFFF" // 255,255,255
	blackInk  = "#000000" // 0,0,0
	whitePill = "#14110E" // 20,17,14 dark ground behind white ink
	blackPill = "#FFFFFF" // 255,255,255 light ground behind black ink
)

// PieceStyle selects the text pieces. The zero value is the classic blocks.
type PieceStyle int

const (
	PieceClassic PieceStyle = iota
	PieceSilhouette
)

func (s PieceStyle) String() string {
	if s == PieceSilhouette {
		return "silhouette"
	}
	return "classic"
}

// ParsePieces reads a flag or config value. Empty is classic.
func ParsePieces(s string) (PieceStyle, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "classic", "block", "blocks":
		return PieceClassic, nil
	case "silhouette", "silhouettes":
		return PieceSilhouette, nil
	default:
		return PieceClassic, fmt.Errorf("unknown --pieces %q (want classic or silhouette)", s)
	}
}

// ResolvePieces uses an explicit flag, otherwise the config value.
// An empty flag means --pieces was omitted.
func ResolvePieces(flag, cfg string) (PieceStyle, error) {
	src := strings.TrimSpace(flag)
	if src == "" {
		src = cfg
	}
	return ParsePieces(src)
}

// axisCenter is the column or row that holds a centred glyph.
// The widths we use are odd, so this is the middle column.
// An even height (the 4-row square) uses the lower middle row so the
// piece is not pinned against the top of the cell.
func axisCenter(n int) int {
	if n <= 1 {
		return 0
	}
	return n / 2
}

// squareName is the label shown in the header.
func squareName(cellW, cellH int) string {
	return fmt.Sprintf("%d×%d", cellW, cellH)
}

func spriteSize(cellW, cellH int) bool {
	return cellW == sizeW && cellH == sizeH
}

// classicMask is the reference picture at quadrant resolution: 16 samples
// across and 8 down, two samples per cell of the 8×4 square. '#' is the
// piece and '.' is the square. One colour, no outline. The king carries a
// cross, the queen a three-point crown, the rook four merlons, the bishop a
// ball on a mitre, the knight a head on the right, and the pawn a round head
// over a wide base. The first and last sample columns stay empty.
var classicMask = map[chess.PieceType][]string{
	chess.King: {
		".......##.......",
		"....#########...",
		"....#########...",
		".......##.......",
		".....######.....",
		"....#########...",
		"...###########..",
		"..############..",
	},
	chess.Queen: {
		"................",
		"......####......",
		"..####.##.####..",
		"..####.##.####..",
		"......####......",
		"....########....",
		"...###########..",
		"..############..",
	},
	chess.Rook: {
		"................",
		"..##.##..##.##..",
		"..##.##..##.##..",
		"..############..",
		"..############..",
		"..############..",
		"..############..",
		"..############..",
	},
	chess.Bishop: {
		"................",
		"......####......",
		".......##.......",
		".....######.....",
		"....#########...",
		".....######.....",
		"......####......",
		"..############..",
	},
	chess.Knight: {
		"................",
		"...........###..",
		"........######..",
		"...###########..",
		"....##########..",
		".....#########..",
		"....##########..",
		"..############..",
	},
	chess.Pawn: {
		"................",
		"................",
		".......##.......",
		"......####......",
		".......##.......",
		".....######.....",
		"....#########...",
		"...###########..",
	},
}

// spriteRows collapses the reference mask to one rune per cell, for tests
// that only need to know which cells the piece occupies.
func spriteRows(pt chess.PieceType, w, h int) []string {
	rows := classicMask[pt]
	if w != sizeW || h != sizeH || len(rows) != h*2 {
		return nil
	}
	out := make([]string, h)
	for sub := 0; sub < h; sub++ {
		var b strings.Builder
		for col := 0; col < w; col++ {
			if classicCellMask(rows, sub, col) == 0 {
				b.WriteRune(' ')
			} else {
				b.WriteRune('█')
			}
		}
		out[sub] = b.String()
	}
	return out
}

// classicCellMask is the 2×2 quadrant mask for one cell. Bit 0 is the upper
// left, then upper right, lower left, lower right.
func classicCellMask(rows []string, sub, col int) int {
	if sub < 0 || col < 0 || len(rows) < (sub+1)*2 {
		return 0
	}
	mask := 0
	for sy := 0; sy < 2; sy++ {
		line := rows[sub*2+sy]
		for sx := 0; sx < 2; sx++ {
			x := col*2 + sx
			if x < len(line) && line[x] == '#' {
				mask |= 1 << uint(sy*2+sx)
			}
		}
	}
	return mask
}

func solidRune(pt chess.PieceType) rune {
	switch pt {
	case chess.King:
		return '♚'
	case chess.Queen:
		return '♛'
	case chess.Rook:
		return '♜'
	case chess.Bishop:
		return '♝'
	case chess.Knight:
		return '♞'
	case chess.Pawn:
		return '♟'
	default:
		return 0
	}
}

func spriteLine(pt chess.PieceType, w, h, sub int) []rune {
	rows := spriteRows(pt, w, h)
	if sub < 0 || sub >= len(rows) {
		return spaces(w)
	}
	r := []rune(rows[sub])
	if len(r) != w {
		return spaces(w)
	}
	return r
}

func spaces(w int) []rune {
	r := make([]rune, w)
	for i := range r {
		r[i] = ' '
	}
	return r
}

// renderSquareLine draws one row of a square. The block pieces are always
// drawn. A graphics protocol and a Kitty terminal never replace them: an
// image, when one was explicitly requested, is only an extra sequence
// outside the cell grid.
func (c cellCtx) renderSquareLine(sq chess.Square, sub, cellW, cellH int) string {
	if !spriteSize(cellW, cellH) {
		content := ""
		if sub == (cellH-1)/2 {
			content = c.glyph(sq)
		}
		return c.renderCell(sq, content, cellW)
	}
	if c.pieces == PieceSilhouette {
		return c.renderArtLine(sq, sub, cellW, cellH)
	}
	return c.renderClassicLine(sq, sub, cellW, cellH)
}

// renderClassicLine draws the reference picture. The whole 8×4 tile uses the
// square or highlight colour as its background. Filled samples are quadrant
// blocks in one foreground: pure white or pure black. Nothing is outlined.
// A colorless white piece is ▒ and a colorless black piece is █, so the two
// sides still differ when the terminal has no colour.
func (c cellCtx) renderClassicLine(sq chess.Square, sub, cellW, cellH int) string {
	p := c.session.Piece(sq)
	tile, _, mono := c.tileInk(sq)
	var rows []string
	if p != chess.NoPiece {
		rows = classicMask[p.Type()]
	}
	fg := whiteBody
	if p != chess.NoPiece && p.Color() == chess.Black {
		fg = blackBody
	}
	kind := c.markKind(sq, p)
	glyphRow := axisCenter(cellH)
	cursorMono := mono && sq == c.machine.Cursor
	var b strings.Builder
	for i := 0; i < cellW; i++ {
		on, _ := markerAt(cellW, cellH, sub, i, kind)
		switch {
		case cursorMono && sub == glyphRow && (i == 0 || i == cellW-1):
			br := '['
			if i == cellW-1 {
				br = ']'
			}
			b.WriteString(c.emit(br, tile, tile, true))
		case on:
			b.WriteString(c.emitMark(sq, markRune(kind)))
		default:
			mask := 0
			if cellW == sizeW && cellH == sizeH {
				mask = classicCellMask(rows, sub, i)
			}
			if mask == 0 || p == chess.NoPiece {
				b.WriteString(c.emit(' ', tile, tile, mono))
				continue
			}
			if mono {
				ch := '█'
				if p.Color() == chess.White {
					ch = '▒'
				}
				b.WriteString(c.emit(ch, tile, tile, true))
				continue
			}
			b.WriteString(c.emit(quadrant[mask], fg, tile, false))
		}
	}
	return b.String()
}

func (c cellCtx) markKind(sq chess.Square, p chess.Piece) markKind {
	d, ok := c.machine.IsTarget(sq)
	if !ok {
		return markNone
	}
	if d.Capture {
		return markCapture
	}
	if p == chess.NoPiece {
		return markTarget
	}
	return markNone
}

func markRune(kind markKind) rune {
	if kind == markCapture {
		return '×'
	}
	return '·'
}

func (c cellCtx) renderArtLine(sq chess.Square, sub, cellW, cellH int) string {
	p := c.session.Piece(sq)
	var row []artCell
	if p != chess.NoPiece {
		_, _, mono := c.tileInk(sq)
		grid := artGrid(p.Color(), p.Type(), cellW, cellH, mono, c.wantsOutline(p, sq), c.detail)
		if sub >= 0 && sub < len(grid) {
			row = grid[sub]
		}
	}
	return c.paintRow(sq, sub, cellW, cellH, row)
}

func (c cellCtx) paintRow(sq chess.Square, sub, cellW, cellH int, row []artCell) string {
	p := c.session.Piece(sq)
	tile, _, mono := c.tileInk(sq)
	kind := c.markKind(sq, p)
	glyphRow := axisCenter(cellH)
	cursorMono := mono && sq == c.machine.Cursor
	var b strings.Builder
	for i := 0; i < cellW; i++ {
		on, _ := markerAt(cellW, cellH, sub, i, kind)
		switch {
		case cursorMono && sub == glyphRow && (i == 0 || i == cellW-1):
			br := '['
			if i == cellW-1 {
				br = ']'
			}
			b.WriteString(c.emit(br, tile, tile, true))
		case on:
			b.WriteString(c.emitMark(sq, markRune(kind)))
		default:
			cell := artCell{ch: ' '}
			if i < len(row) {
				cell = row[i]
			}
			b.WriteString(c.paintArt(cell, tile, mono))
		}
	}
	return b.String()
}

// paintArt draws one silhouette sample. The glyph's foreground is the side
// colour. The background stays the tile, including through a highlight.
func (c cellCtx) paintArt(cell artCell, tile rgb, mono bool) string {
	ch := legacyRune(cell.ch)
	if ch == 0 || ch == ' ' {
		return c.emit(' ', tile, tile, mono)
	}
	fg := tile
	if !mono {
		switch cell.fg {
		case 1:
			fg = whiteBody
		case 2:
			fg = blackBody
		}
	}
	return c.emit(ch, fg, tile, mono)
}

type markKind int

const (
	markNone markKind = iota
	markTarget
	markCapture
)

// markerAt is one centred cell. Empty targets are a dot. Captures are ×.
func markerAt(cellW, cellH, sub, i int, kind markKind) (on, center bool) {
	if kind == markNone {
		return false, false
	}
	row := axisCenter(cellH)
	col := axisCenter(cellW)
	if sub == row && i == col {
		return true, true
	}
	return false, false
}

func pieceStyle(p chess.Piece) lipgloss.Style {
	ink, ground := blackInk, blackPill
	if p.Color() == chess.White {
		ink, ground = whiteInk, whitePill
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ink)).
		Background(lipgloss.Color(ground)).
		Bold(true)
}

func pieceGround(p chess.Piece) lipgloss.Style {
	ground := blackPill
	if p.Color() == chess.White {
		ground = whitePill
	}
	return lipgloss.NewStyle().Background(lipgloss.Color(ground))
}

func isChessRune(r rune) bool {
	return r >= 0x2654 && r <= 0x265F
}

// textPiece returns the chess glyph unchanged. A variation selector (U+FE0E)
// makes Kitty drop the glyph: the font has no VS15 cmap entry, so the cell
// is blank. These symbols are text by default, so the foreground colour applies.
func textPiece(s string) string {
	return s
}
