// Package gfx places chess piece images with terminal graphics protocols.
//
// The default is off: the board draws text pieces. A protocol is used only
// when --graphics is kitty, iterm, or sixel. There is no automatic detection.
// TERM, KITTY_WINDOW_ID, KITTY_PID, a tty query, and a saved config value
// cannot turn images on. --graphics off forces text.
package gfx

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// Mode is a resolved graphics protocol. The zero value draws no images.
type Mode int

const (
	Off Mode = iota
	Kitty
	ITerm
	Sixel
)

func (m Mode) String() string {
	switch m {
	case Kitty:
		return "kitty"
	case ITerm:
		return "iterm"
	case Sixel:
		return "sixel"
	default:
		return "off"
	}
}

// Parse reads an explicit --graphics value. An empty value and off are text.
// auto is not a protocol: nothing in this package inspects the terminal.
func Parse(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off", "none", "no", "false":
		return Off, nil
	case "kitty":
		return Kitty, nil
	case "iterm", "iterm2":
		return ITerm, nil
	case "sixel":
		return Sixel, nil
	default:
		return Off, fmt.Errorf("unknown --graphics %q (want kitty, iterm, sixel, or off)", s)
	}
}

// Resolve returns the protocol named by an explicit --graphics flag.
// An empty flag means the flag was omitted and the result is off.
// cfg and env are ignored so a saved "kitty" or "auto" value, TERM, and
// KITTY_* cannot select a protocol. off forces text over any of those.
func Resolve(flag, cfg string, env []string) (Mode, error) {
	_, _ = cfg, env
	return Parse(flag)
}

// Spot is one piece image, in terminal cells. Row and Col are 1-based.
type Spot struct {
	Row, Col   int
	Cols, Rows int
	ID         int
	PNG        []byte
}

// Frame builds the sequences for one redraw. sent remembers which Kitty image
// ids have already been uploaded, and which virtual placements exist, so a
// later frame does not transmit or delete anything. The returned map is the
// updated set.
//
// Kitty images are not cursor-placed. A redraw cycle is:
//
//  1. The first time an id appears, transmit the PNG once (a=t) and create a
//     virtual placement (a=p,U=1) at the square's cell size. Nothing is deleted.
//  2. The board text stays the block pieces. This package does not replace
//     those cells with U+10EEEE placeholders.
//  3. A later frame with the same ids and size emits no graphics sequences.
//     An empty spot list also emits nothing. a=d is reserved for Cleanup,
//     after the program has left the alternate screen.
func Frame(m Mode, spots []Spot, sent map[int]struct{}) (string, map[int]struct{}) {
	if m == Off || len(spots) == 0 || m != Kitty {
		if m == ITerm && len(spots) > 0 {
			return itermFrame(spots), sent
		}
		if m == Sixel && len(spots) > 0 {
			return sixelFrame(spots), sent
		}
		return "", sent
	}
	return kittyFrame(spots, sent)
}

// Cleanup frees images after the program leaves the alternate screen.
func Cleanup(m Mode) string {
	if m == Kitty {
		return "\x1b_Ga=d,d=A,q=2\x1b\\"
	}
	return ""
}

// Placeholder is one Kitty unicode-placeholder cell. row and col are 0-based
// positions inside the virtual placement. The image id is not encoded here;
// it is the cell's foreground colour (38;5;<id>), and the placement id is the
// underline colour. Both diacritics are always written so a cell does not
// inherit its neighbour's column when two pieces share an image id.
func Placeholder(row, col int) string {
	if row < 0 || col < 0 || row >= len(rowColDiacritic) || col >= len(rowColDiacritic) {
		return ""
	}
	return string([]rune{placeholderRune, rowColDiacritic[row], rowColDiacritic[col]})
}

// PlacementID is the non-zero virtual-placement id for a cell rectangle.
// It fits in an 8-bit underline colour. Rows and columns above 15 share the
// last id; a chess square never asks for that.
func PlacementID(cols, rows int) int {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	if cols > 15 {
		cols = 15
	}
	if rows > 15 {
		rows = 15
	}
	return rows<<4 | cols
}

// rowColDiacritic is the start of Kitty's rowcolumn-diacritics.txt.
// Index 0 is U+0305 and index 1 is U+030D, matching the 2×2 example in the
// graphics protocol.
var rowColDiacritic = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F,
	0x0346, 0x034A, 0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357,
}

const placeholderRune = '\U0010EEEE'

func kittyFrame(spots []Spot, sent map[int]struct{}) (string, map[int]struct{}) {
	if sent == nil {
		sent = map[int]struct{}{}
	}
	var b strings.Builder
	transmitted := map[int]struct{}{}
	placed := map[int]struct{}{}
	for _, s := range spots {
		if s.ID <= 0 || s.Cols < 1 || s.Rows < 1 {
			continue
		}
		if _, ok := sent[s.ID]; !ok {
			if _, ok := transmitted[s.ID]; !ok {
				transmitted[s.ID] = struct{}{}
				b.WriteString(kittyTransmit(s.ID, s.PNG))
				sent[s.ID] = struct{}{}
			}
		}
		key := s.ID<<16 | s.Cols<<8 | s.Rows
		if _, ok := sent[key]; ok {
			continue
		}
		if _, ok := placed[key]; ok {
			continue
		}
		placed[key] = struct{}{}
		// Virtual placement only. U=1 does not paint anything; the placeholder
		// characters in the grid do. No cursor move, no z-index, no delete.
		fmt.Fprintf(&b, "\x1b_Ga=p,U=1,i=%d,p=%d,c=%d,r=%d,q=2\x1b\\",
			s.ID, PlacementID(s.Cols, s.Rows), s.Cols, s.Rows)
		sent[key] = struct{}{}
	}
	return b.String(), sent
}

func kittyTransmit(id int, png []byte) string {
	payload := base64.StdEncoding.EncodeToString(png)
	const max = 4096
	var b strings.Builder
	if payload == "" {
		fmt.Fprintf(&b, "\x1b_Ga=t,f=100,t=d,i=%d,q=2,m=0;\x1b\\", id)
		return b.String()
	}
	for off := 0; off < len(payload); {
		end := off + max
		more := 1
		if end >= len(payload) {
			end = len(payload)
			more = 0
		}
		if off == 0 {
			fmt.Fprintf(&b, "\x1b_Ga=t,f=100,t=d,i=%d,q=2,m=%d;", id, more)
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;", more)
		}
		b.WriteString(payload[off:end])
		b.WriteString("\x1b\\")
		off = end
	}
	return b.String()
}

func itermFrame(spots []Spot) string {
	var b strings.Builder
	b.WriteString("\x1b7")
	for _, s := range spots {
		fmt.Fprintf(&b, "\x1b[%d;%dH", s.Row, s.Col)
		fmt.Fprintf(&b, "\x1b]1337;File=inline=1;width=%d;height=%d;preserveAspectRatio=1:%s\a",
			s.Cols, s.Rows, base64.StdEncoding.EncodeToString(s.PNG))
	}
	b.WriteString("\x1b8")
	return b.String()
}

func sixelFrame(spots []Spot) string {
	var b strings.Builder
	b.WriteString("\x1b7")
	for _, s := range spots {
		fmt.Fprintf(&b, "\x1b[%d;%dH", s.Row, s.Col)
		b.WriteString(sixelImage(s.PNG, s.Cols, s.Rows))
	}
	b.WriteString("\x1b8")
	return b.String()
}

// Cell pixels assumed when a terminal does not report its cell size. Sixel
// has no cell-unit scale, so the bitmap is this many pixels per column and row.
const (
	CellPxW = 8
	CellPxH = 16
)

func sixelImage(pngBytes []byte, cols, rows int) string {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	src, err := png.Decode(strings.NewReader(string(pngBytes)))
	if err != nil {
		return ""
	}
	w, h := cols*CellPxW, rows*CellPxH
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return encodeSixel(dst)
}

func encodeSixel(img *image.NRGBA) string {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	idx := make([]int, w*h)
	var pal []color.NRGBA
	seen := map[uint32]int{}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(b.Min.X+x, b.Min.Y+y)
			if c.A < 128 {
				idx[y*w+x] = -1
				continue
			}
			key := uint32(c.R>>3)<<16 | uint32(c.G>>3)<<8 | uint32(c.B>>3)
			id, ok := seen[key]
			if !ok {
				id = len(pal)
				seen[key] = id
				pal = append(pal, color.NRGBA{c.R &^ 7, c.G &^ 7, c.B &^ 7, 255})
			}
			idx[y*w+x] = id
		}
	}
	var s strings.Builder
	fmt.Fprintf(&s, "\x1bPq\"1;1;%d;%d", w, h)
	for i, c := range pal {
		fmt.Fprintf(&s, "#%d;2;%d;%d;%d", i, int(c.R)*100/255, int(c.G)*100/255, int(c.B)*100/255)
	}
	for y := 0; y < h; y += 6 {
		for ci := range pal {
			var band strings.Builder
			used := false
			for x := 0; x < w; x++ {
				bits := 0
				for dy := 0; dy < 6; dy++ {
					yy := y + dy
					if yy < h && idx[yy*w+x] == ci {
						bits |= 1 << uint(dy)
						used = true
					}
				}
				band.WriteByte(byte(63 + bits))
			}
			if !used {
				continue
			}
			fmt.Fprintf(&s, "#%d%s$", ci, sixelRLE(band.String()))
		}
		s.WriteByte('-')
	}
	s.WriteString("\x1b\\")
	return s.String()
}

func sixelRLE(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		j := i + 1
		for j < len(s) && s[j] == s[i] && j-i < 255 {
			j++
		}
		n := j - i
		if n >= 4 {
			fmt.Fprintf(&b, "!%d%c", n, s[i])
		} else {
			b.WriteString(s[i:j])
		}
		i = j
	}
	return b.String()
}
