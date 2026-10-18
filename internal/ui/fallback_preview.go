package ui

import (
	"image"
	"image/color"
	"image/draw"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/notnil/chess"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"termchess/internal/game"
	"termchess/internal/interact"
	"termchess/internal/theme"
)

// FallbackPreview paints the quadrant silhouettes for the 8×4 square.
// It is not the SVG raster.
func FallbackPreview() *image.NRGBA {
	const pxW, pxH = 12, 24
	const gap = 18
	light := color.NRGBA{176, 137, 104, 255}
	dark := color.NRGBA{107, 79, 58, 255}
	order := []chess.PieceType{chess.King, chess.Queen, chess.Rook, chess.Bishop, chess.Knight, chess.Pawn}
	sizes := [][2]int{{sizeW, sizeH}}
	w := 6*(sizeW*pxW) + 7*gap
	h := gap
	for _, sz := range sizes {
		h += 2*(sz[1]*pxH) + 2*gap
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.NRGBA{42, 32, 24, 255}}, image.Point{}, draw.Src)
	y := gap
	for _, sz := range sizes {
		for row := 0; row < 2; row++ {
			side := chess.Black
			if row == 1 {
				side = chess.White
			}
			rowW := 6*(sz[0]*pxW) + 5*gap
			x0 := (w - rowW) / 2
			for col, pt := range order {
				x := x0 + col*(sz[0]*pxW+gap)
				sq := chess.NewSquare(chess.File(col), chess.Rank(1-row))
				bg := light
				if darkSquare(sq) {
					bg = dark
				}
				paintFallbackSquare(dst, x, y, side, pt, bg.R > 140, bg, sz[0], sz[1], pxW, pxH)
			}
			y += sz[1]*pxH + gap
		}
		y += gap
	}
	return dst
}

func paintFallbackSquare(dst *image.NRGBA, x, y int, side chess.Color, pt chess.PieceType, lightGround bool, bg color.NRGBA, cellW, cellH, pxW, pxH int) {
	subW, subH := BlockQuadrant.subs()
	outline := (side == chess.White && lightGround) || (side == chess.Black && !lightGround)
	grid := artGrid(side, pt, cellW, cellH, false, outline, BlockQuadrant)
	white := color.NRGBA{245, 245, 245, 255}
	black := color.NRGBA{17, 17, 17, 255}
	fill, contrast := white, black
	if side == chess.Black {
		fill, contrast = black, white
	}
	for cy := 0; cy < cellH; cy++ {
		for cx := 0; cx < cellW; cx++ {
			cell := grid[cy][cx]
			ink := fill
			if (side == chess.White && cell.fg == 2) || (side == chess.Black && cell.fg == 1) {
				ink = contrast
			}
			hole := bg
			if cell.bg == 1 {
				hole = white
			} else if cell.bg == 2 {
				hole = black
			}
			for sy := 0; sy < subH; sy++ {
				for sx := 0; sx < subW; sx++ {
					bit := sy*subW + sx
					on := cell.ch != ' ' && cell.ch != 0 && quadrantBit(cell.ch, bit)
					c := hole
					if on {
						c = ink
					}
					x0 := x + cx*pxW + sx*(pxW/subW)
					y0 := y + cy*pxH + sy*(pxH/subH)
					draw.Draw(dst, image.Rect(x0, y0, x0+pxW/subW, y0+pxH/subH), &image.Uniform{c}, image.Point{}, draw.Src)
				}
			}
		}
	}
}

func quadrantBit(ch rune, bit int) bool {
	for mask, r := range quadrant {
		if r == ch {
			return mask&(1<<uint(bit)) != 0
		}
	}
	return false
}

// BoardPreview rasterises the large starting board through renderBoard, the
// same function the terminal uses. The cursor sits on e2, as a new game does.
func BoardPreview() *image.NRGBA {
	s, err := game.New(game.Options{TC: game.PresetByID("none")})
	if err != nil {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	st := theme.For(theme.Wood, true)
	ctx := cellCtx{
		session: s,
		machine: interact.New(chess.E2, chess.White),
		styles:  st,
		detail:  BlockQuadrant,
	}
	return rasterANSI(renderBoard(ctx, sizeW, sizeH, false), 10, 20)
}

func rasterANSI(board string, pxW, pxH int) *image.NRGBA {
	lines := strings.Split(board, "\n")
	wide := 0
	for _, ln := range lines {
		if n := cellCount(ln); n > wide {
			wide = n
		}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, wide*pxW, len(lines)*pxH))
	frame := color.NRGBA{42, 32, 24, 255}
	draw.Draw(dst, dst.Bounds(), &image.Uniform{frame}, image.Point{}, draw.Src)
	for y, ln := range lines {
		paintANSILine(dst, ln, y*pxH, pxW, pxH, frame)
	}
	return dst
}

func cellCount(s string) int {
	n := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			j := strings.IndexByte(s[i:], 'm')
			if j < 0 {
				break
			}
			i += j + 1
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		n++
		i += size
	}
	return n
}

type pen struct {
	fg, bg       color.NRGBA
	hasFg, hasBg bool
}

func paintANSILine(dst *image.NRGBA, s string, y, pxW, pxH int, frame color.NRGBA) {
	var p pen
	x := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			j := strings.IndexByte(s[i:], 'm')
			if j < 0 {
				break
			}
			applySGR(s[i+1:i+j], &p)
			i += j + 1
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		fg, bg := frame, frame
		if p.hasFg {
			fg = p.fg
		}
		if p.hasBg {
			bg = p.bg
		}
		paintCell(dst, x*pxW, y, pxW, pxH, r, fg, bg)
		x++
	}
}

func applySGR(seq string, p *pen) {
	if strings.HasPrefix(seq, "[") {
		seq = seq[1:]
	}
	if seq == "" || seq == "0" {
		*p = pen{}
		return
	}
	// Lip Gloss uses semicolons. Some terminals use colons inside 38:2.
	seq = strings.ReplaceAll(seq, ":", ";")
	parts := strings.Split(seq, ";")
	for i := 0; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}
		switch n {
		case 0:
			*p = pen{}
		case 39:
			p.hasFg = false
		case 49:
			p.hasBg = false
		case 38, 48:
			if i+1 < len(parts) && parts[i+1] == "2" && i+4 < len(parts) {
				r := atoiU(parts[i+2])
				g := atoiU(parts[i+3])
				b := atoiU(parts[i+4])
				c := color.NRGBA{r, g, b, 255}
				if n == 38 {
					p.fg, p.hasFg = c, true
				} else {
					p.bg, p.hasBg = c, true
				}
				i += 4
			} else if i+1 < len(parts) && parts[i+1] == "5" && i+2 < len(parts) {
				i += 2
			}
		}
	}
}

func atoiU(s string) uint8 {
	n, _ := strconv.Atoi(s)
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return uint8(n)
}

func paintCell(dst *image.NRGBA, x, y, pxW, pxH int, r rune, fg, bg color.NRGBA) {
	if isQuadrantRune(r) {
		subW, subH := 2, 2
		for sy := 0; sy < subH; sy++ {
			for sx := 0; sx < subW; sx++ {
				c := bg
				if quadrantBit(r, sy*subW+sx) {
					c = fg
				}
				x0 := x + sx*(pxW/subW)
				y0 := y + sy*(pxH/subH)
				draw.Draw(dst, image.Rect(x0, y0, x0+pxW/subW, y0+pxH/subH), &image.Uniform{c}, image.Point{}, draw.Src)
			}
		}
		return
	}
	draw.Draw(dst, image.Rect(x, y, x+pxW, y+pxH), &image.Uniform{bg}, image.Point{}, draw.Src)
	if r == ' ' || r == 0 {
		return
	}
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  dst,
		Src:  &image.Uniform{fg},
		Face: face,
		Dot:  fixed.P(x+(pxW-7)/2, y+(pxH-13)/2+face.Ascent),
	}
	d.DrawString(string(r))
}

func isQuadrantRune(r rune) bool {
	switch r {
	case '▘', '▝', '▀', '▖', '▌', '▞', '▛', '▗', '▚', '▐', '▜', '▄', '▙', '▟', '█':
		return true
	default:
		return false
	}
}
