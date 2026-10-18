package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/mattn/go-runewidth"
	"github.com/notnil/chess"
	"termchess/internal/coord"
	"termchess/internal/game"
	"termchess/internal/interact"
	"termchess/internal/theme"
)

// layout reports whether the 8×4 board fits, and whether the history panel
// can sit beside it. A window that cannot hold that board is too small.
// There is no smaller square size.
func layout(width, height int) (cellW, cellH int, side bool, tooSmall bool) {
	cw, ch := sizeW, sizeH
	fits := func(beside bool) bool {
		bw := 3 + 8*cw + 2
		bh := 2 + 8*ch
		needW := bw + 2
		needH := bh + 7
		if beside {
			needW += 32
		} else {
			needH += 8
		}
		return width >= needW && height >= needH
	}
	if fits(true) {
		return cw, ch, true, false
	}
	if fits(false) {
		return cw, ch, false, false
	}
	return cw, ch, false, true
}

// displayWidth counts cells. Chess symbols are one cell, and a variation
// selector adds none, even when a width library treats the symbol as an emoji.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		switch r {
		case '\uFE0E', '\uFE0F', '\u200D':
			continue
		}
		if isChessRune(r) {
			w++
			continue
		}
		rw := runewidth.RuneWidth(r)
		if rw < 1 {
			rw = 1
		}
		w += rw
	}
	return w
}

// solidGlyph is a filled piece for both colours. Outline glyphs (♔) are faint
// on a dark terminal; the side colour carries white versus black.
func solidGlyph(p chess.Piece) string {
	switch p.Type() {
	case chess.King:
		return "♚"
	case chess.Queen:
		return "♛"
	case chess.Rook:
		return "♜"
	case chess.Bishop:
		return "♝"
	case chess.Knight:
		return "♞"
	case chess.Pawn:
		return "♟"
	default:
		return ""
	}
}

func pad(s string, w int) string {
	sw := displayWidth(s)
	for sw > w && s != "" {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
		sw = displayWidth(s)
	}
	left := (w - sw) / 2
	if left < 0 {
		left = 0
	}
	right := w - sw - left
	if right < 0 {
		right = 0
	}
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func darkSquare(sq chess.Square) bool {
	return (int(sq.File())+int(sq.Rank()))%2 == 0
}

type cellCtx struct {
	session  *game.Session
	machine  *interact.Machine
	styles   theme.Styles
	checkHot bool
	illegal  bool
	// mono forces the colorless cursor (brackets) even when the theme is wood.
	mono bool
	// pieces selects classic blocks or the hand-drawn silhouettes. The zero
	// value is classic.
	pieces PieceStyle
	// detail selects quadrant, sextant, or octant text art.
	detail BlockDetail
	// profile is how board cells downsample their explicit colours.
	// The zero value emits 24-bit SGR.
	profile colorprofile.Profile
}

func (c cellCtx) style(sq chess.Square) lipgloss.Style {
	st := c.styles
	base := st.Light
	if darkSquare(sq) {
		base = st.Dark
	}
	var out lipgloss.Style
	switch {
	case c.illegal && sq == c.machine.Cursor:
		out = st.Illegal
	case sq == c.machine.Cursor:
		out = st.Cursor
	case c.machine.HasSel && sq == c.machine.Selected:
		out = st.Selected
	default:
		if c.session.InCheck() {
			if king, ok := game.KingSquare(c.session.Game().Position().Board(), c.session.Turn()); ok && king == sq {
				if c.checkHot {
					out = st.CheckHot
				} else {
					out = st.Check
				}
				break
			}
		}
		if d, ok := c.machine.IsTarget(sq); ok {
			if d.Capture {
				out = st.Capture
			} else {
				out = st.Target
			}
			break
		}
		if pm := c.machine.Premove; pm != nil && (sq == pm.From || sq == pm.To) {
			out = st.Premove
			break
		}
		if from, to, ok := c.session.LastMove(); ok {
			if sq == from {
				out = st.LastFrom
				break
			}
			if sq == to {
				out = st.LastTo
				break
			}
		}
		out = base
	}
	return out
}

// wantsOutline used to add a contrasting crust. Colour silhouettes ignore it
// and stay one solid ink on every square. The flag remains so a highlight
// still takes the same path; buildArt does not paint from it.
func (c cellCtx) wantsOutline(p chess.Piece, sq chess.Square) bool {
	if c.styles.Mono || c.mono || p == chess.NoPiece {
		return false
	}
	if p.Color() == chess.White {
		return !darkSquare(sq) || c.highlighted(sq)
	}
	return darkSquare(sq) && !c.highlighted(sq)
}

func (c cellCtx) highlighted(sq chess.Square) bool {
	if (c.illegal && sq == c.machine.Cursor) || sq == c.machine.Cursor {
		return true
	}
	if c.machine.HasSel && sq == c.machine.Selected {
		return true
	}
	if c.session.InCheck() {
		if king, ok := game.KingSquare(c.session.Game().Position().Board(), c.session.Turn()); ok && king == sq {
			return true
		}
	}
	if _, ok := c.machine.IsTarget(sq); ok {
		return true
	}
	if pm := c.machine.Premove; pm != nil && (sq == pm.From || sq == pm.To) {
		return true
	}
	if from, to, ok := c.session.LastMove(); ok && (sq == from || sq == to) {
		return true
	}
	return false
}

// renderCell draws one square. A resting piece is a bold glyph centred on a
// three-cell pill, with the square colour left in the padding. Each column
// is rendered on its own so a chess glyph cannot be measured as two cells.
func (c cellCtx) renderCell(sq chess.Square, content string, cellW int) string {
	base := c.style(sq)
	if content == "" || c.styles.Mono || c.mono || c.styles.WhitePiece == nil {
		return paintColumns(base, content, cellW, lipgloss.Style{}, false)
	}
	p := c.session.Piece(sq)
	if p == chess.NoPiece {
		return paintColumns(base.Bold(true), content, cellW, lipgloss.Style{}, false)
	}
	if content == solidGlyph(p) && cellW >= 3 {
		return renderPill(base, p, cellW)
	}
	return paintColumns(base, content, cellW, pieceStyle(p), true)
}

// renderPill centres a 3-wide side-coloured ground, with the glyph in its
// middle cell. Leftover columns keep the square or highlight colour.
func renderPill(base lipgloss.Style, p chess.Piece, cellW int) string {
	pw := 3
	if pw > cellW {
		pw = cellW
	}
	side := (cellW - pw) / 2
	glyphAt := side + pw/2
	ground := pieceGround(p)
	glyph := pieceStyle(p)
	var b strings.Builder
	for i := 0; i < cellW; i++ {
		switch {
		case i == glyphAt:
			b.WriteString(glyph.Render(textPiece(solidGlyph(p))))
		case i >= side && i < side+pw:
			b.WriteString(ground.Render(" "))
		default:
			b.WriteString(base.Render(" "))
		}
	}
	return b.String()
}

// paintColumns renders one rune per cell. Chess symbols are forced to text
// presentation and are never handed to lipgloss as part of a wider string.
func paintColumns(base lipgloss.Style, content string, cellW int, piece lipgloss.Style, usePiece bool) string {
	var b strings.Builder
	for _, r := range pad(content, cellW) {
		switch {
		case usePiece && isChessRune(r):
			b.WriteString(piece.Render(textPiece(string(r))))
		case usePiece && r != ' ':
			b.WriteString(piece.Render(string(r)))
		case isChessRune(r):
			b.WriteString(base.Render(textPiece(string(r))))
		default:
			b.WriteString(base.Render(string(r)))
		}
	}
	return b.String()
}

func pieceChip(p chess.Piece, st theme.Styles) lipgloss.Style {
	_ = st
	return pieceStyle(p)
}

func (c cellCtx) glyph(sq chess.Square) string {
	p := c.session.Piece(sq)
	g := ""
	if p != chess.NoPiece {
		if c.styles.Mono || c.mono {
			// Outline for white, solid for black, so a colorless terminal
			// can still tell the sides apart.
			g = p.String()
		} else {
			g = solidGlyph(p)
		}
	}
	if d, ok := c.machine.IsTarget(sq); ok {
		if d.Capture {
			// A mark, not only a background, so a capture reads without colour.
			if g == "" {
				g = "×"
			} else {
				g += "×"
			}
		} else if g == "" {
			g = "·"
		}
	}
	if (c.styles.Mono || c.mono) && sq == c.machine.Cursor {
		if g == "" {
			g = " "
		}
		return "[" + g + "]"
	}
	return g
}

// renderBoard draws the 8×8 grid with rank and file labels.
// Every line has the same visible width.
func renderBoard(c cellCtx, cellW, cellH int, flipped bool) string {
	files := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	if flipped {
		for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
			files[i], files[j] = files[j], files[i]
		}
	}
	var labels []string
	for _, f := range files {
		labels = append(labels, pad(f, cellW))
	}
	fileLine := "   " + strings.Join(labels, "") + "  "
	var lines []string
	lines = append(lines, c.frameText(fileLine))
	// The rank digit sits on the same row as the centred glyph.
	// Every label cell, including the padding rows, uses the dark frame.
	labelRow := axisCenter(cellH)
	for row := 0; row < 8; row++ {
		rank := rankLabel(row, flipped)
		for sub := 0; sub < cellH; sub++ {
			var b strings.Builder
			if sub == labelRow {
				b.WriteString(c.frameText(fmt.Sprintf("%2s ", rank)))
			} else {
				b.WriteString(c.frameText("   "))
			}
			for col := 0; col < 8; col++ {
				sq := coord.Square(row, col, flipped)
				b.WriteString(c.renderSquareLine(sq, sub, cellW, cellH))
			}
			if sub == labelRow {
				b.WriteString(c.frameText(" " + rank))
			} else {
				b.WriteString(c.frameText("  "))
			}
			lines = append(lines, b.String())
		}
	}
	lines = append(lines, c.frameText(fileLine))
	return strings.Join(lines, "\n")
}

// frameText paints the file and rank margin. Each cell has an explicit
// foreground and the dark border background, so the frame does not depend
// on the terminal's own background.
func (c cellCtx) frameText(s string) string {
	if c.styles.Mono || c.mono {
		return s
	}
	fg, okF := rgbFrom(c.styles.Frame.GetForeground())
	bg, okB := rgbFrom(c.styles.Frame.GetBackground())
	if !okF || !okB {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		b.WriteString(paintTermCell(r, fg, bg, c.profile))
	}
	return b.String()
}

func rankLabel(row int, flipped bool) string {
	if !flipped {
		return fmt.Sprintf("%d", 8-row)
	}
	return fmt.Sprintf("%d", row+1)
}

func clockLine(name, time string, active bool, st theme.Styles) string {
	label := fmt.Sprintf("%-12s %s", name, time)
	if active {
		return st.ClockOn.Render(label)
	}
	return st.ClockOff.Render(label)
}

func promotionLine(m *interact.Machine, st theme.Styles) string {
	names := []string{"Q", "R", "B", "N"}
	if len(m.Promo) == 0 {
		return ""
	}
	var parts []string
	for i, pt := range m.Promo {
		label := names[0]
		switch pt {
		case chess.Queen:
			label = "Q"
		case chess.Rook:
			label = "R"
		case chess.Bishop:
			label = "B"
		case chess.Knight:
			label = "N"
		}
		if i == m.PromoIndex {
			parts = append(parts, st.Cursor.Render(" "+label+" "))
		} else {
			parts = append(parts, st.Muted.Render(" "+label+" "))
		}
	}
	return st.Title.Render("Promote") + "  " + strings.Join(parts, "") + "   " + st.Muted.Render("← → choose   enter confirm   esc cancel")
}
