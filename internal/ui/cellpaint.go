package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/notnil/chess"
)

// Piece ink is only a foreground. The square, or a highlight, is the
// background of every cell in the 8×4 tile. White is pure white and black
// is pure black, with no second colour and no outline.
var (
	whiteBody = rgb{255, 255, 255} // #FFFFFF
	blackBody = rgb{0, 0, 0}       // #000000
)

type rgb struct{ R, G, B uint8 }

func (c rgb) String() string { return fmt.Sprintf("%d,%d,%d", c.R, c.G, c.B) }

func pieceInk(p chess.Piece) rgb {
	if p.Color() == chess.White {
		return whiteBody
	}
	return blackBody
}

func rgbFrom(c color.Color) (rgb, bool) {
	if c == nil {
		return rgb{}, false
	}
	r, g, b, a := c.RGBA()
	if a == 0 {
		return rgb{}, false
	}
	return rgb{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}, true
}

// tileInk is the square or highlight background, and the ink already chosen
// for marks on that square. A theme with no background is drawn as text.
func (c cellCtx) tileInk(sq chess.Square) (tile, ink rgb, mono bool) {
	st := c.style(sq)
	tile, ok := rgbFrom(st.GetBackground())
	ink, _ = rgbFrom(st.GetForeground())
	mono = c.styles.Mono || c.mono || !ok
	return tile, ink, mono
}

// emit writes one board cell. Colour cells always carry an explicit
// foreground and background. Nothing is left to reverse video, a default
// colour, or a width-padding library.
func (c cellCtx) emit(ch rune, fg, bg rgb, mono bool) string {
	if ch == 0 {
		ch = ' '
	}
	if mono {
		return string(ch)
	}
	return paintTermCell(ch, fg, bg, c.profile)
}

func (c cellCtx) emitMark(sq chess.Square, ch rune) string {
	tile, ink, mono := c.tileInk(sq)
	if mono {
		return string(ch)
	}
	return paintTermCell(ch, ink, tile, c.profile)
}

// paintTermCell is one character with both colours set, then a reset so the
// next cell cannot inherit a missing parameter. The zero profile, like
// TrueColor, is 24-bit. ANSI256 and ANSI are downsampled.
func paintTermCell(ch rune, fg, bg rgb, profile colorprofile.Profile) string {
	var b strings.Builder
	b.WriteString("\x1b[")
	b.WriteString(colorParam(38, fg, profile))
	b.WriteByte(';')
	b.WriteString(colorParam(48, bg, profile))
	b.WriteByte('m')
	b.WriteRune(ch)
	b.WriteString("\x1b[m")
	return b.String()
}

func colorParam(ground int, c rgb, profile colorprofile.Profile) string {
	src := color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255}
	switch profile {
	case colorprofile.ANSI256:
		switch conv := colorprofile.ANSI256.Convert(src).(type) {
		case ansi.IndexedColor:
			return fmt.Sprintf("%d;5;%d", ground, uint8(conv))
		case ansi.BasicColor:
			return fmt.Sprintf("%d;5;%d", ground, uint8(conv))
		}
	case colorprofile.ANSI:
		switch conv := colorprofile.ANSI.Convert(src).(type) {
		case ansi.BasicColor:
			return basicParam(ground, uint8(conv))
		case ansi.IndexedColor:
			return basicParam(ground, uint8(conv))
		}
	}
	return fmt.Sprintf("%d;2;%d;%d;%d", ground, c.R, c.G, c.B)
}

func basicParam(ground int, n uint8) string {
	n &= 15
	if ground == 38 {
		if n < 8 {
			return strconv.Itoa(30 + int(n))
		}
		return strconv.Itoa(90 + int(n-8))
	}
	if n < 8 {
		return strconv.Itoa(40 + int(n))
	}
	return strconv.Itoa(100 + int(n-8))
}
