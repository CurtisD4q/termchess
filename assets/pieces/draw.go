// Package pieces draws an original Staunton-like chess set.
//
// The silhouettes are drawn from scratch in this file (not traced from an
// existing font or diagram). White and black are the same outlines with the
// fill and the rim swapped. See LICENSE.
package pieces

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"

	"github.com/notnil/chess"
	"golang.org/x/image/vector"
)

const design = 64

// Ink colours. The rim is the other side's fill so a piece reads on either square.
var (
	WhiteFill = color.NRGBA{247, 244, 239, 255}
	WhiteEdge = color.NRGBA{26, 18, 12, 255}
	BlackFill = color.NRGBA{22, 18, 16, 255}
	BlackEdge = color.NRGBA{246, 240, 228, 255}
)

// Types is the six piece kinds in display order.
func Types() []chess.PieceType {
	return []chess.PieceType{chess.King, chess.Queen, chess.Rook, chess.Bishop, chess.Knight, chess.Pawn}
}

// ID is the stable graphics id for a side and a piece, from 1 to 12.
func ID(c chess.Color, pt chess.PieceType) int {
	if pt < chess.King || pt > chess.Pawn {
		return 0
	}
	id := int(pt)
	if c == chess.Black {
		id += 6
	}
	return id
}

// Name is the file stem, such as white-knight.
func Name(c chess.Color, pt chess.PieceType) string {
	side := "white"
	if c == chess.Black {
		side = "black"
	}
	kind := [...]string{"", "king", "queen", "rook", "bishop", "knight", "pawn"}
	return side + "-" + kind[pt]
}

type xy struct{ x, y float32 }

type layer struct {
	fill color.NRGBA
	pts  []xy
}

// Layers is the paint order for one piece: rim, then fill, then cutouts.
func Layers(c chess.Color, pt chess.PieceType) []layer {
	fill, edge := WhiteFill, WhiteEdge
	if c == chess.Black {
		fill, edge = BlackFill, BlackEdge
	}
	var parts [][]xy
	var cuts [][]xy
	switch pt {
	case chess.King:
		parts = kingParts()
	case chess.Queen:
		parts = queenParts()
	case chess.Rook:
		parts = rookParts()
	case chess.Bishop:
		parts = bishopParts()
		cuts = [][]xy{bishopSlit()}
	case chess.Knight:
		parts = [][]xy{knightBody()}
		cuts = [][]xy{circ(30, 20, 2.4, 12)}
	default:
		parts = pawnParts()
	}
	var out []layer
	for _, p := range parts {
		out = append(out, layer{edge, fat(p, 1.1)})
		out = append(out, layer{fill, p})
	}
	for _, p := range cuts {
		if len(p) >= 3 {
			out = append(out, layer{edge, p})
		}
	}
	return out
}

// Render rasterises one piece into a square RGBA image with a transparent ground.
func Render(c chess.Color, pt chess.PieceType, size int) *image.NRGBA {
	if size < 1 {
		size = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	scale := float32(size) / design
	for _, l := range Layers(c, pt) {
		fillPoly(dst, l.pts, scale, l.fill)
	}
	return dst
}

// SVG is the same drawing as a standalone document.
func SVG(c chess.Color, pt chess.PieceType) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(&b, "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 %d %d\" width=\"%d\" height=\"%d\">\n", design, design, design, design)
	b.WriteString("  <!-- Original Termchess piece. MIT. Drawn in assets/pieces/draw.go. -->\n")
	for _, l := range Layers(c, pt) {
		fmt.Fprintf(&b, "  <path fill=\"%s\" d=\"%s\"/>\n", hex(l.fill), pathD(l.pts))
	}
	b.WriteString("</svg>\n")
	return b.String()
}

// EmbeddedPNG is the 128px file produced from the same drawing.
func EmbeddedPNG(c chess.Color, pt chess.PieceType) ([]byte, error) {
	return Files.ReadFile("png/" + Name(c, pt) + "-128.png")
}

// FillSpan is the fraction of the image width and height covered by opaque pixels.
func FillSpan(img *image.NRGBA) (width, height float64) {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return 0, 0
	}
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y).A < 80 {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}
	if minX >= maxX || minY >= maxY {
		return 0, 0
	}
	return float64(maxX-minX) / float64(b.Dx()), float64(maxY-minY) / float64(b.Dy())
}

// PNG encodes Render at size.
func PNG(c chess.Color, pt chess.PieceType, size int) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, Render(c, pt, size)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Preview is a contact sheet: black row, then white row, on a wood checkerboard.
func Preview() *image.NRGBA {
	const cell = 128
	const gap = 16
	w := 6*cell + 7*gap
	h := 2*cell + 3*gap
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.NRGBA{42, 32, 24, 255}}, image.Point{}, draw.Src)
	light := color.NRGBA{176, 137, 104, 255}
	dark := color.NRGBA{107, 79, 58, 255}
	order := Types()
	for row := 0; row < 2; row++ {
		side := chess.Black
		if row == 1 {
			side = chess.White
		}
		for col, pt := range order {
			x := gap + col*(cell+gap)
			y := gap + row*(cell+gap)
			sq := light
			if (row+col)%2 == 1 {
				sq = dark
			}
			draw.Draw(dst, image.Rect(x, y, x+cell, y+cell), &image.Uniform{sq}, image.Point{}, draw.Src)
			im := Render(side, pt, cell)
			draw.Draw(dst, image.Rect(x, y, x+cell, y+cell), im, image.Point{}, draw.Over)
		}
	}
	return dst
}

func fillPoly(dst *image.NRGBA, pts []xy, scale float32, c color.NRGBA) {
	if len(pts) < 3 {
		return
	}
	var r vector.Rasterizer
	r.Reset(dst.Bounds().Dx(), dst.Bounds().Dy())
	r.MoveTo(pts[0].x*scale, pts[0].y*scale)
	for _, p := range pts[1:] {
		r.LineTo(p.x*scale, p.y*scale)
	}
	r.ClosePath()
	r.Draw(dst, dst.Bounds(), &image.Uniform{c}, image.Point{})
}

func pathD(pts []xy) string {
	if len(pts) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "M%.2f %.2f", pts[0].x, pts[0].y)
	for _, p := range pts[1:] {
		fmt.Fprintf(&b, "L%.2f %.2f", p.x, p.y)
	}
	b.WriteByte('Z')
	return b.String()
}

func hex(c color.NRGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

func fat(pts []xy, amount float32) []xy {
	if len(pts) == 0 {
		return nil
	}
	var cx, cy float32
	for _, p := range pts {
		cx += p.x
		cy += p.y
	}
	n := float32(len(pts))
	cx /= n
	cy /= n
	out := make([]xy, len(pts))
	for i, p := range pts {
		out[i] = xy{cx + (p.x-cx)*amount, cy + (p.y-cy)*amount}
	}
	return out
}

func circ(cx, cy, r float32, n int) []xy {
	if n < 8 {
		n = 8
	}
	pts := make([]xy, n)
	for i := 0; i < n; i++ {
		a := float64(i) / float64(n) * 2 * math.Pi
		pts[i] = xy{cx + r*float32(math.Cos(a)), cy + r*float32(math.Sin(a))}
	}
	return pts
}

func rect(x0, y0, x1, y1 float32) []xy {
	return []xy{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
}

func lerp(a, b xy, t float32) xy {
	return xy{a.x + (b.x-a.x)*t, a.y + (b.y-a.y)*t}
}

func plinth(top, halfW float32) []xy {
	// A low rounded plinth. y grows downward.
	y0, y1 := top, top+8
	x0, x1 := 32-halfW, 32+halfW
	return roundish(x0, y0, x1, y1, 3)
}

func roundish(x0, y0, x1, y1, rad float32) []xy {
	var p []xy
	add := func(cx, cy, a0, a1 float64) {
		const n = 5
		for i := 0; i <= n; i++ {
			t := a0 + (a1-a0)*float64(i)/float64(n)
			p = append(p, xy{float32(cx) + float32(math.Cos(t))*rad, float32(cy) + float32(math.Sin(t))*rad})
		}
	}
	// Corners: top-left, top-right, bottom-right, bottom-left. y-down, so
	// screen angles run clockwise from the left side of the top edge.
	add(float64(x0+rad), float64(y0+rad), math.Pi, 3*math.Pi/2)
	add(float64(x1-rad), float64(y0+rad), 3*math.Pi/2, 2*math.Pi)
	add(float64(x1-rad), float64(y1-rad), 0, math.Pi/2)
	add(float64(x0+rad), float64(y1-rad), math.Pi/2, math.Pi)
	return p
}

func pawnParts() [][]xy {
	return [][]xy{
		plinth(50, 24),
		body(30, 50, 10, 16),
		circ(32, 28, 7, 16),
		circ(32, 16, 9, 20),
	}
}

func rookParts() [][]xy {
	return [][]xy{
		plinth(50, 24),
		body(22, 50, 16, 20),
		rect(8, 12, 20, 24),
		rect(26, 7, 38, 24),
		rect(44, 12, 56, 24),
	}
}

func bishopParts() [][]xy {
	mitre := []xy{
		{32, 14}, {16, 36}, {20, 42}, {44, 42}, {48, 36},
	}
	return [][]xy{
		plinth(50, 24),
		body(44, 50, 14, 18),
		circ(32, 40, 11, 16),
		mitre,
		circ(32, 12, 5, 14),
	}
}

func bishopSlit() []xy { return rect(29.5, 20, 34.5, 34) }

func queenParts() [][]xy {
	return [][]xy{
		plinth(50, 24),
		body(34, 50, 15, 20),
		roundish(12, 26, 52, 36, 4),
		circ(11, 20, 4.6, 12),
		circ(21, 14, 4.4, 12),
		circ(32, 11, 4.6, 12),
		circ(43, 14, 4.4, 12),
		circ(53, 20, 4.6, 12),
	}
}

func kingParts() [][]xy {
	return [][]xy{
		plinth(50, 24),
		body(34, 50, 16, 21),
		roundish(14, 24, 50, 36, 4),
		rect(28, 6, 36, 22),
		rect(16, 10, 48, 17),
	}
}

func knightBody() []xy {
	// Profile facing left. The base and the head reach the same outer box
	// as the other pieces, about four fifths of the design square.
	return []xy{
		{38, 6},
		{48, 8},
		{54, 16},
		{52, 26},
		{56, 34},
		{54, 46},
		{56, 58},
		{8, 58},
		{10, 48},
		{16, 44},
		{18, 36},
		{8, 30},
		{6, 22},
		{14, 14},
		{26, 10},
		{34, 8},
	}
}

func body(y0, y1, topHalf, botHalf float32) []xy {
	topL := xy{32 - topHalf, y0}
	topR := xy{32 + topHalf, y0}
	botR := xy{32 + botHalf, y1}
	botL := xy{32 - botHalf, y1}
	// A few steps along the sides so the body is not a pure trapezoid.
	var p []xy
	p = append(p, topL, topR)
	for i := 1; i <= 4; i++ {
		t := float32(i) / 4
		p = append(p, lerp(topR, botR, t))
	}
	for i := 1; i <= 4; i++ {
		t := float32(i) / 4
		p = append(p, lerp(botR, botL, t))
	}
	for i := 1; i <= 3; i++ {
		t := float32(i) / 4
		p = append(p, lerp(botL, topL, t))
	}
	return p
}
