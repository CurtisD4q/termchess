package ui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/notnil/chess"
	"termchess/internal/game"
	"termchess/internal/interact"
	"termchess/internal/theme"
)

func TestSolidPiecesAndCaptureMark(t *testing.T) {
	s, err := game.New(game.Options{
		FEN: "4k3/8/8/5p2/8/8/2B5/4K3 w - - 0 1",
		TC:  game.PresetByID("none"),
	})
	if err != nil {
		t.Fatal(err)
	}
	m := interact.New(chess.C2, chess.NoColor)
	m.Confirm(s)
	ctx := cellCtx{session: s, machine: m, styles: theme.For(theme.Wood, true)}
	board := renderBoard(ctx, 5, 2, false)
	if strings.Contains(board, "♗") || strings.Contains(board, "♔") {
		t.Fatal("outline glyphs are still on the board")
	}
	if !strings.Contains(board, "♝") || !strings.Contains(board, "×") {
		t.Fatalf("missing solid bishop or capture mark:\n%s", board)
	}
	if !strings.Contains(board, "·") {
		t.Fatalf("missing empty-square target:\n%s", board)
	}
	// Large cells are 5 wide and two rows; every line stays the same width.
	lines := strings.Split(board, "\n")
	width := lipWidth(lines[0])
	for i, ln := range lines {
		if w := lipWidth(ln); w != width {
			t.Fatalf("line %d width %d, want %d", i, w, width)
		}
	}
}

func TestSpriteMasksDiffer(t *testing.T) {
	types := []chess.PieceType{chess.King, chess.Queen, chess.Rook, chess.Bishop, chess.Knight, chess.Pawn}
	for _, sz := range [][2]int{{sizeW, sizeH}} {
		seen := map[string]chess.PieceType{}
		for _, pt := range types {
			var b strings.Builder
			for sub := 0; sub < sz[1]; sub++ {
				line := spriteLine(pt, sz[0], sz[1], sub)
				if len(line) != sz[0] {
					t.Fatalf("%s %dx%d row %d has %d runes", pt, sz[0], sz[1], sub, len(line))
				}
				if runewidth.StringWidth(string(line)) != sz[0] {
					t.Fatalf("%s %dx%d row %d display width %d", pt, sz[0], sz[1], sub, runewidth.StringWidth(string(line)))
				}
				b.WriteString(string(line))
				b.WriteByte('\n')
			}
			if prev, ok := seen[b.String()]; ok {
				t.Fatalf("%s matches %s at %dx%d\n%s", pt, prev, sz[0], sz[1], b.String())
			}
			seen[b.String()] = pt
		}
	}
}

func TestClassicBitmaps(t *testing.T) {
	seen := map[string]chess.PieceType{}
	for pt, rows := range classicMask {
		if len(rows) != sizeH*2 {
			t.Fatalf("%s has %d sample rows", pt, len(rows))
		}
		for _, row := range rows {
			if len(row) != sizeW*2 || row[0] != '.' || row[len(row)-1] != '.' {
				t.Fatalf("%s row %q is not inset", pt, row)
			}
		}
		joined := strings.Join(rows, "\n")
		if prev, ok := seen[joined]; ok {
			t.Fatalf("%s matches %s", pt, prev)
		}
		seen[joined] = pt
		flat := spriteRows(pt, sizeW, sizeH)
		if len(flat) != sizeH || len([]rune(flat[0])) != sizeW {
			t.Fatalf("%s collapsed %q", pt, flat)
		}
	}
	king := strings.Join(classicMask[chess.King], "\n")
	if !strings.HasPrefix(classicMask[chess.King][0], ".......##") {
		t.Fatalf("king cross:\n%s", king)
	}
	if !strings.Contains(classicMask[chess.Rook][1], "##.##") {
		t.Fatal("rook crown is not crenellated")
	}
	if !strings.Contains(classicMask[chess.Queen][2], "####.##.####") {
		t.Fatal("queen crown is not three spikes")
	}
	if !strings.Contains(classicMask[chess.Bishop][1], "####") {
		t.Fatal("bishop has no ball")
	}
	if !strings.Contains(classicMask[chess.Knight][1], "###") {
		t.Fatal("knight has no head")
	}
	pawn := classicMask[chess.Pawn]
	if !strings.Contains(pawn[3], "####") || !strings.Contains(pawn[7], "###########") {
		t.Fatalf("pawn head or base:\n%s", strings.Join(pawn, "\n"))
	}
}

func TestResolvePieces(t *testing.T) {
	if got, err := ResolvePieces("", ""); err != nil || got != PieceClassic {
		t.Fatalf("empty = %v %v", got, err)
	}
	if got, err := ResolvePieces("", "silhouette"); err != nil || got != PieceSilhouette {
		t.Fatalf("config = %v %v", got, err)
	}
	if got, err := ResolvePieces("classic", "silhouette"); err != nil || got != PieceClassic {
		t.Fatalf("flag = %v %v", got, err)
	}
	if _, err := ResolvePieces("blob", ""); err == nil {
		t.Fatal("blob was accepted")
	}
}

func TestRenderPiecesEverySizeAndStyle(t *testing.T) {
	s := mustPos(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	whiteAt := map[chess.PieceType]chess.Square{
		chess.King: chess.E1, chess.Queen: chess.D1, chess.Rook: chess.A1,
		chess.Bishop: chess.C1, chess.Knight: chess.B1, chess.Pawn: chess.E2,
	}
	blackAt := map[chess.PieceType]chess.Square{
		chess.King: chess.E8, chess.Queen: chess.D8, chess.Rook: chess.A8,
		chess.Bishop: chess.C8, chess.Knight: chess.B8, chess.Pawn: chess.E7,
	}
	sizes := []struct {
		name   string
		w, h   int
		detail BlockDetail
	}{
		{"quadrant", sizeW, sizeH, BlockQuadrant},
		{"sextant", sizeW, sizeH, BlockSextant},
	}
	for _, style := range []PieceStyle{PieceClassic, PieceSilhouette} {
		for _, sz := range sizes {
			t.Run(style.String()+"/"+sz.name, func(t *testing.T) {
				ctx := cellCtx{
					session: s,
					machine: interact.New(chess.A1, chess.NoColor),
					styles:  theme.For(theme.Wood, true),
					pieces:  style,
					detail:  sz.detail,
				}
				for _, pt := range []chess.PieceType{chess.King, chess.Queen, chess.Rook, chess.Bishop, chess.Knight, chess.Pawn} {
					for _, side := range []struct {
						sq    chess.Square
						white bool
					}{{whiteAt[pt], true}, {blackAt[pt], false}} {
						var plain, ansi string
						for sub := 0; sub < sz.h; sub++ {
							line := ctx.renderSquareLine(side.sq, sub, sz.w, sz.h)
							plain += plainText(line)
							ansi += line
						}
						if !pieceCellVisible(plain) {
							t.Fatalf("%s %s %dx%d is empty:\n%q", pt, side.sq, sz.w, sz.h, plain)
						}
						for _, r := range plain {
							if r > 0xFFFF {
								t.Fatalf("%s %s draws %U, which Kitty's font leaves blank", pt, side.sq, r)
							}
						}
						if strings.ContainsRune(ansi, '\uFE0E') {
							t.Fatalf("%s %s uses a variation selector", pt, side.sq)
						}
						ink := "38;2;0;0;0"
						if side.white {
							ink = "38;2;255;255;255"
						}
						if !strings.Contains(ansi, ink) {
							t.Fatalf("%s %s has no %s", pt, side.sq, ink)
						}
						if strings.Contains(ansi, "38;2;32;32;32") || strings.Contains(ansi, "38;2;216;216;216") {
							t.Fatalf("%s %s still draws an outline", pt, side.sq)
						}
					}
				}
			})
		}
	}
}

func pieceCellVisible(plain string) bool {
	for _, r := range plain {
		if r != ' ' && r != '·' && r != '×' && r != '[' && r != ']' {
			return true
		}
	}
	return false
}

func TestSilhouetteStyleUsesInk(t *testing.T) {
	s := mustPos(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	ctx := cellCtx{
		session: s,
		machine: interact.New(chess.A1, chess.NoColor),
		styles:  theme.For(theme.Wood, true),
		pieces:  PieceSilhouette,
	}
	white := squareANSI(ctx, chess.E2, sizeW, sizeH)
	if !strings.Contains(white, "38;2;255;255;255") {
		t.Fatal("silhouette white is not the light foreground")
	}
	if strings.Contains(white, "48;2;255;255;255") {
		t.Fatal("silhouette painted the piece colour as a background")
	}
}

func TestLayoutIsOneSquareSize(t *testing.T) {
	w, h, side, small := layout(200, 80)
	if small || !side || w != sizeW || h != sizeH {
		t.Fatalf("wide = %dx%d side %v small %v", w, h, side, small)
	}
	w, h, side, small = layout(80, 60)
	if small || side || w != sizeW || h != sizeH {
		t.Fatalf("stacked = %dx%d side %v small %v", w, h, side, small)
	}
	w, h, _, small = layout(20, 10)
	if !small || w != sizeW || h != sizeH {
		t.Fatalf("tiny = %dx%d small %v", w, h, small)
	}
}

func TestLargeSpritesContrastAndAlign(t *testing.T) {
	s, err := game.New(game.Options{TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	m := interact.New(chess.E1, chess.NoColor)
	ctx := cellCtx{session: s, machine: m, styles: theme.For(theme.Wood, true)}
	board := renderBoard(ctx, sizeW, sizeH, false)
	plain := plainText(board)
	if !containsArt(plain) {
		t.Fatal("large squares are missing the block pieces")
	}
	assertSameWidth(t, board)
	// Rank 1 is the last piece rank. White king is the e-file, bishop the c-file.
	rankLines := strings.Split(plain, "\n")
	start := 1 + 7*4
	king := piecePatch(rankLines[start:start+4], 4, sizeW)
	bishop := piecePatch(rankLines[start:start+4], 2, sizeW)
	if king == bishop {
		t.Fatalf("king and bishop silhouettes match:\n%s", king)
	}
	if !containsArt(king) || !containsArt(bishop) {
		t.Fatalf("king or bishop silhouette is blank:\n%s\n%s", king, bishop)
	}
	if !strings.Contains(board, "48;2;176;138;100") && !strings.Contains(board, "48;2;107;79;58") {
		t.Fatal("large squares lost the checkerboard around the pieces")
	}
	if strings.Count(board, "38;2;255;255;255") == 0 || strings.Count(board, "38;2;0;0;0") == 0 {
		t.Fatal("classic body colours are missing")
	}
	if strings.Contains(board, "38;2;32;32;32") || strings.Contains(board, "38;2;216;216;216") {
		t.Fatal("classic pieces still draw an outline")
	}
	plainTop := strings.Split(plain, "\n")[0]
	plainBot := strings.Split(plain, "\n")
	plainBotLine := plainBot[len(plainBot)-1]
	for _, file := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		if !strings.Contains(plainTop, file) || !strings.Contains(plainBotLine, file) {
			t.Fatalf("file %s missing from the frame\n%s\n%s", file, plainTop, plainBotLine)
		}
	}
	if !strings.Contains(board, "48;2;42;32;24") {
		t.Fatal("file and rank labels are not on the dark border")
	}
	if strings.Contains(plain, "░") {
		t.Fatal("drop shadow is still drawn")
	}
	assertNoChessGlyphs(t, plain)
	assertArtFillsSquare(t, sizeW, sizeH)

	// A bishop that can take and that has an empty step.
	capGame, err := game.New(game.Options{
		FEN: "4k3/8/8/5p2/8/8/2B5/4K3 w - - 0 1",
		TC:  game.PresetByID("none"),
	})
	if err != nil {
		t.Fatal(err)
	}
	capM := interact.New(chess.C2, chess.NoColor)
	capM.Confirm(capGame)
	capRaw := renderBoard(cellCtx{session: capGame, machine: capM, styles: theme.For(theme.Wood, true)}, sizeW, sizeH, false)
	capBoard := plainText(capRaw)
	if !strings.Contains(capBoard, "×") {
		t.Fatalf("missing capture mark:\n%s", capBoard)
	}
	if !strings.Contains(capBoard, "·") {
		t.Fatalf("missing centred target dot:\n%s", capBoard)
	}
	if strings.Contains(capRaw, "48;2;16;32;48") {
		t.Fatal("empty-target mark is still a navy block")
	}
	assertSameWidth(t, capRaw)

	mono := renderBoard(cellCtx{session: s, machine: m, styles: theme.For(theme.Mono, true)}, sizeW, sizeH, false)
	monoPlain := plainText(mono)
	monoLines := strings.Split(monoPlain, "\n")
	whiteKing := piecePatch(monoLines[start:start+4], 4, sizeW)
	blackKing := piecePatch(monoLines[1:5], 4, sizeW)
	if whiteKing == blackKing || !strings.Contains(whiteKing, "▒") || !strings.Contains(blackKing, "█") {
		t.Fatalf("monochrome sprites do not separate the sides:\nwhite\n%s\nblack\n%s", whiteKing, blackKing)
	}
	if !strings.Contains(monoPlain, "[") || !strings.Contains(monoPlain, "]") {
		t.Fatal("monochrome cursor has no brackets")
	}
	assertSameWidth(t, mono)
}

func assertNoChessGlyphs(t *testing.T, s string) {
	t.Helper()
	for _, r := range "♔♕♖♗♘♙♚♛♜♝♞♟" {
		if strings.ContainsRune(s, r) {
			t.Fatalf("large or medium square still uses a chess glyph %c", r)
		}
	}
}

func assertArtFillsSquare(t *testing.T, w, h int) {
	t.Helper()
	for _, pt := range []chess.PieceType{chess.King, chess.Queen, chess.Rook, chess.Bishop, chess.Knight, chess.Pawn} {
		fw, fh := artFill(pt, w, h)
		t.Logf("%s %dx%d cells %.0f%% × %.0f%%", pt, w, h, fw*100, fh*100)
		if fw < 0.7 || fh < 0.7 {
			t.Fatalf("%s %dx%d fills %.0f%% × %.0f%% of the cell", pt, w, h, fw*100, fh*100)
		}
	}
}

func artFill(pt chess.PieceType, w, h int) (float64, float64) {
	grid := artGrid(chess.White, pt, w, h, false, false, BlockQuadrant)
	minX, minY := w, h
	maxX, maxY := 0, 0
	for y, row := range grid {
		for x, cell := range row {
			if cell.ch == 0 || cell.ch == ' ' {
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
	return float64(maxX-minX) / float64(w), float64(maxY-minY) / float64(h)
}

func TestKnightMarginAndCleanInk(t *testing.T) {
	s := mustPos(t, "4k3/3p4/4p3/8/8/8/3P4/4K3 w - - 0 1")
	st := theme.For(theme.Wood, true)
	m := interact.New(chess.A1, chess.NoColor)
	ctx := cellCtx{session: s, machine: m, styles: st}
	for _, sz := range [][2]int{{sizeW, sizeH}} {
		for _, pt := range []chess.PieceType{chess.King, chess.Queen, chess.Rook, chess.Bishop, chess.Knight, chess.Pawn} {
			for sub := 0; sub < sz[1]; sub++ {
				line := spriteLine(pt, sz[0], sz[1], sub)
				if line[0] != ' ' || line[len(line)-1] != ' ' {
					t.Fatalf("%s %dx%d row %d touches the square edge %q", pt, sz[0], sz[1], sub, string(line))
				}
			}
		}
		// d2 is a dark square. A white pawn there is solid white, with no black crust.
		dark := squareANSI(ctx, chess.D2, sz[0], sz[1])
		if strings.Contains(dark, "38;2;0;0;0") {
			t.Fatalf("white pawn on a dark square has black specks (%dx%d)", sz[0], sz[1])
		}
		// d7 is a light square. A black pawn there is solid black, with no white crust.
		light := squareANSI(ctx, chess.D7, sz[0], sz[1])
		if strings.Contains(light, "38;2;255;255;255") {
			t.Fatalf("black pawn on a light square has white specks (%dx%d)", sz[0], sz[1])
		}
	}
	queen := strings.Join(maskRows(chess.Queen, BlockQuadrant, sizeW, sizeH), "\n")
	if !strings.Contains(queen, "#.#.#.#.#") {
		t.Fatalf("queen crown is not five spikes:\n%s", queen)
	}
	start := cellCtx{session: mustPos(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"), machine: m, styles: st}
	// Light squares (a2, c2, f1, d1, b1) and dark squares (b2, g1) are the same ink.
	for _, sq := range []chess.Square{chess.A2, chess.C2, chess.B2, chess.F1, chess.D1, chess.B1, chess.G1} {
		ansi := squareANSI(start, sq, sizeW, sizeH)
		if strings.Contains(ansi, "38;2;0;0;0") || strings.Contains(ansi, "48;2;0;0;0") {
			t.Fatalf("%s has dark ink inside a white piece:\n%s", sq, ansi)
		}
	}
	for _, sq := range []chess.Square{chess.F7, chess.A7, chess.B8, chess.D8} {
		ansi := squareANSI(start, sq, sizeW, sizeH)
		if strings.Contains(ansi, "38;2;255;255;255") || strings.Contains(ansi, "48;2;255;255;255") {
			t.Fatalf("%s has light ink inside a black piece:\n%s", sq, ansi)
		}
	}
}

func TestSolidOnEverySquare(t *testing.T) {
	for _, pt := range []chess.PieceType{chess.King, chess.Queen, chess.Rook, chess.Bishop, chess.Knight, chess.Pawn} {
		for _, detail := range []BlockDetail{BlockQuadrant, BlockSextant} {
			rows := maskRows(pt, detail, sizeW, sizeH)
			if strings.Contains(strings.Join(rows, ""), "x") {
				t.Fatalf("%s %s still has a cut", pt, detail)
			}
		}
		off := artGrid(chess.White, pt, sizeW, sizeH, false, false, BlockQuadrant)
		on := artGrid(chess.White, pt, sizeW, sizeH, false, true, BlockQuadrant)
		for y := range off {
			for x := range off[y] {
				if off[y][x] != on[y][x] {
					t.Fatalf("%s changes when the old crust flag is set", pt)
				}
				if off[y][x].fg == 2 || off[y][x].bg != 0 {
					t.Fatalf("%s cell %d,%d is not solid white ink: %+v", pt, x, y, off[y][x])
				}
			}
		}
	}
}

func TestBoardPreviewHasBothSides(t *testing.T) {
	img := BoardPreview()
	var white, black, light, dark int
	b := img.Bounds()
	if b.Dx() < 100 || b.Dy() < 100 {
		t.Fatalf("board preview is %dx%d", b.Dx(), b.Dy())
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			switch {
			case c.R == 255 && c.G == 255 && c.B == 255:
				white++
			case c.R == 0 && c.G == 0 && c.B == 0:
				black++
			case c.R == 176 && c.G == 138 && c.B == 100:
				light++
			case c.R == 107 && c.G == 79 && c.B == 58:
				dark++
			}
		}
	}
	if white < 200 || black < 200 || light < 200 || dark < 200 {
		t.Fatalf("preview pixels white %d black %d light %d dark %d", white, black, light, dark)
	}
}

func TestKittyBoardIsOneCellWide(t *testing.T) {
	s := mustPos(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	m := interact.New(chess.E2, chess.White)
	ctx := cellCtx{session: s, machine: m, styles: theme.For(theme.Wood, true)}
	board := renderBoard(ctx, sizeW, sizeH, false)
	assertSameWidth(t, board)
	if !strings.Contains(board, "38;2;255;255;255") || !strings.Contains(plainText(board), "█") {
		t.Fatal("start board has no block pieces")
	}
	if strings.Contains(board, "\U0010EEEE") || strings.Contains(board, "a=d") || strings.Contains(board, "z=-1") {
		t.Fatal("board replaced pieces with a graphics placeholder")
	}
}

func TestEmptyCapturedIsADash(t *testing.T) {
	st := theme.For(theme.Wood, true)
	got := renderCaptured(nil, 0, st)
	plain := plainText(got)
	if strings.Contains(got, "—") || strings.Contains(plain, "█") || strings.Contains(plain, "▄") {
		t.Fatalf("empty captured draws a block: %q", plain)
	}
	if plain != "-" {
		t.Fatalf("empty captured = %q", plain)
	}
}

func TestPieceSilhouettes(t *testing.T) {
	sets := []struct {
		detail BlockDetail
		w, h   int
		wide   int
		tall   int
		subW   int
		subH   int
	}{
		{BlockQuadrant, sizeW, sizeH, 16, 8, 2, 2},
		{BlockSextant, sizeW, sizeH, 16, 12, 2, 3},
	}
	for _, set := range sets {
		king := maskRows(chess.King, set.detail, set.w, set.h)
		if len(king) != set.tall {
			t.Fatalf("king %v %dx%d has %d rows", set.detail, set.w, set.h, len(king))
		}
		crossRows := 2 * set.subH
		for y := 0; y < crossRows; y++ {
			if !strings.Contains(king[y], "#") {
				t.Fatalf("king cross row %d is empty at %dx%d:\n%s", y, set.w, set.h, strings.Join(king, "\n"))
			}
		}
		for _, pt := range []chess.PieceType{chess.King, chess.Queen, chess.Rook, chess.Bishop, chess.Knight, chess.Pawn} {
			rows := maskRows(pt, set.detail, set.w, set.h)
			if len(rows) != set.tall {
				t.Fatalf("%s %dx%d has %d rows", pt, set.w, set.h, len(rows))
			}
			top := strings.Contains(rows[0], "#")
			if pt == chess.King && !top {
				t.Fatalf("king is not the tallest piece at %dx%d", set.w, set.h)
			}
			if pt != chess.King && top {
				t.Fatalf("%s reaches the top sample the king uses at %dx%d", pt, set.w, set.h)
			}
			for y, row := range rows {
				if len(row) != set.wide {
					t.Fatalf("%s %dx%d row %d width %d", pt, set.w, set.h, y, len(row))
				}
				for _, r := range row {
					if r != '#' && r != '.' {
						t.Fatalf("%s %dx%d row %d has %q", pt, set.w, set.h, y, r)
					}
				}
			}
			if strings.Contains(strings.Join(rows, ""), "x") {
				t.Fatalf("%s has an interior cut at %dx%d", pt, set.w, set.h)
			}
		}
	}
}

func TestSextantPiecesDiffer(t *testing.T) {
	seen := map[string]chess.PieceType{}
	for _, pt := range []chess.PieceType{chess.King, chess.Queen, chess.Rook, chess.Bishop, chess.Knight, chess.Pawn} {
		var b strings.Builder
		grid := artGrid(chess.White, pt, sizeW, sizeH, false, true, BlockSextant)
		for _, row := range grid {
			for _, cell := range row {
				b.WriteRune(cell.ch)
			}
			b.WriteByte('\n')
		}
		if !containsSextant(b.String()) {
			t.Fatalf("%s has no sextant glyph:\n%s", pt, b.String())
		}
		if prev, ok := seen[b.String()]; ok {
			t.Fatalf("%s matches %s", pt, prev)
		}
		seen[b.String()] = pt
	}
	if r := sextantRune(1); r != '\U0001FB00' {
		t.Fatalf("sextant 1 = %U", r)
	}
	if sextantRune(21) != '▌' || sextantRune(42) != '▐' || sextantRune(63) != '█' {
		t.Fatal("sextant legacy masks drifted")
	}
}

func containsSextant(s string) bool {
	for _, r := range s {
		if r >= 0x1FB00 && r <= 0x1FB3B {
			return true
		}
	}
	return false
}

func containsArt(s string) bool {
	for _, r := range s {
		switch r {
		case '▘', '▝', '▀', '▖', '▌', '▞', '▛', '▗', '▚', '▐', '▜', '▄', '▙', '▟', '█':
			return true
		}
	}
	return false
}

func assertSameWidth(t *testing.T, board string) {
	t.Helper()
	lines := strings.Split(board, "\n")
	width := lipWidth(lines[0])
	for i, ln := range lines {
		if w := lipWidth(ln); w != width {
			t.Fatalf("line %d width %d, want %d", i, w, width)
		}
	}
}

func piecePatch(lines []string, file, cellW int) string {
	var b strings.Builder
	for _, ln := range lines {
		r := []rune(ln)
		at := 3 + file*cellW
		if at+cellW > len(r) {
			return ""
		}
		b.WriteString(string(r[at : at+cellW]))
		b.WriteByte('\n')
	}
	return b.String()
}

func TestSideStaysDistinctUnderHighlights(t *testing.T) {
	st := theme.For(theme.Wood, true)
	pieces := []struct {
		sq    chess.Square
		white bool
	}{
		{chess.D2, true},  // dark square
		{chess.E2, true},  // light square
		{chess.D6, false}, // dark square
		{chess.E6, false}, // light square
	}
	for _, sz := range [][2]int{{sizeW, sizeH}} {
		for _, pc := range pieces {
			for _, hl := range highlightCases(t, pc.sq) {
				s := mustPos(t, hl.fen)
				m := interact.New(chess.A1, chess.NoColor)
				ctx := hl.apply(s, m)
				ctx.styles = st
				ansi := squareANSI(ctx, pc.sq, sz[0], sz[1])
				assertSideColors(t, ansi, pc.white, hl.name+" "+pc.sq.String())
				if hl.bg != "" && !strings.Contains(ansi, hl.bg) {
					t.Fatalf("%s %s %dx%d lost the highlight margin %s", hl.name, pc.sq, sz[0], sz[1], hl.bg)
				}
			}
		}
	}
}

type highlightCase struct {
	name  string
	fen   string
	bg    string
	apply func(s *game.Session, m *interact.Machine) cellCtx
}

func highlightCases(t *testing.T, sq chess.Square) []highlightCase {
	t.Helper()
	base := "4k3/8/3pp3/8/8/8/3PP3/4K3 w - - 0 1"
	return []highlightCase{
		{name: "resting", fen: base, bg: squareBG(sq), apply: func(s *game.Session, m *interact.Machine) cellCtx {
			return cellCtx{session: s, machine: m}
		}},
		{name: "cursor", fen: base, bg: "48;2;242;242;122", apply: func(s *game.Session, m *interact.Machine) cellCtx {
			m.Cursor = sq
			return cellCtx{session: s, machine: m}
		}},
		{name: "selected", fen: base, bg: "48;2;205;210;106", apply: func(s *game.Session, m *interact.Machine) cellCtx {
			m.HasSel = true
			m.Selected = sq
			return cellCtx{session: s, machine: m}
		}},
		{name: "target", fen: base, bg: "48;2;243;227;138", apply: func(s *game.Session, m *interact.Machine) cellCtx {
			m.HasSel = true
			m.Targets = []interact.Dest{{Sq: sq, Capture: false}}
			return cellCtx{session: s, machine: m}
		}},
		{name: "capture", fen: base, bg: "48;2;224;112;112", apply: func(s *game.Session, m *interact.Machine) cellCtx {
			m.HasSel = true
			m.Targets = []interact.Dest{{Sq: sq, Capture: true}}
			return cellCtx{session: s, machine: m}
		}},
		{name: "premove", fen: base, bg: "48;2;215;160;196", apply: func(s *game.Session, m *interact.Machine) cellCtx {
			m.Premove = &interact.Premove{From: sq, To: sq}
			return cellCtx{session: s, machine: m}
		}},
		{name: "illegal", fen: base, bg: "48;2;255;90;90", apply: func(s *game.Session, m *interact.Machine) cellCtx {
			m.Cursor = sq
			return cellCtx{session: s, machine: m, illegal: true}
		}},
	}
}

func TestLastMoveKeepsBlackPawnDark(t *testing.T) {
	st := theme.For(theme.Wood, true)
	// 1. e4 d6 — the black pawn on d6 is the last-move square.
	s := mustPos(t, "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1")
	mv, err := (chess.UCINotation{}).Decode(s.Game().Position(), "d7d6")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Play(mv); err != nil {
		t.Fatal(err)
	}
	ctx := cellCtx{session: s, machine: interact.New(chess.A1, chess.NoColor), styles: st}
	ansi := squareANSI(ctx, chess.D6, sizeW, sizeH)
	assertSideColors(t, ansi, false, "last-move d6")
	if !strings.Contains(ansi, "48;2;214;192;122") {
		t.Fatal("last-move highlight is not visible around the black pawn")
	}
	// White's pawn on e4 is the previous destination's from-square colour pair:
	// a white piece on a light square under the last-move-from highlight.
	from := squareANSI(ctx, chess.E4, sizeW, sizeH)
	assertSideColors(t, from, true, "last-move e4")
}

func TestCheckKeepsKingSide(t *testing.T) {
	st := theme.For(theme.Wood, true)
	// White king on e1 (dark) is in check.
	white := mustPos(t, "4k3/8/8/8/8/8/4q3/4K3 w - - 0 1")
	if !white.InCheck() {
		t.Fatal("expected white in check")
	}
	ctx := cellCtx{session: white, machine: interact.New(chess.A1, chess.NoColor), styles: st}
	assertSideColors(t, squareANSI(ctx, chess.E1, sizeW, sizeH), true, "white check")
	// Black king on d8 (dark) is in check.
	black := mustPos(t, "3k4/8/8/8/8/8/8/3Q3K b - - 0 1")
	if !black.InCheck() {
		t.Fatal("expected black in check")
	}
	ctx = cellCtx{session: black, machine: interact.New(chess.A1, chess.NoColor), styles: st}
	ansi := squareANSI(ctx, chess.D8, sizeW, sizeH)
	assertSideColors(t, ansi, false, "black check")
	if !strings.Contains(ansi, "48;2;224;112;112") {
		t.Fatal("check highlight is not visible around the black king")
	}
}

func squareANSI(c cellCtx, sq chess.Square, w, h int) string {
	var b strings.Builder
	for sub := 0; sub < h; sub++ {
		b.WriteString(c.renderSquareLine(sq, sub, w, h))
	}
	return b.String()
}

func assertSideColors(t *testing.T, ansi string, white bool, label string) {
	t.Helper()
	lightBody := strings.Count(ansi, "38;2;255;255;255")
	darkBody := strings.Count(ansi, "38;2;0;0;0")
	for _, bg := range []string{"48;2;255;255;255", "48;2;0;0;0"} {
		if strings.Contains(ansi, bg) {
			t.Fatalf("%s: piece colour was used as a background", label)
		}
	}
	if strings.Contains(ansi, "38;2;32;32;32") || strings.Contains(ansi, "38;2;216;216;216") {
		t.Fatalf("%s: piece still has an outline", label)
	}
	if white {
		if lightBody == 0 {
			t.Fatalf("%s: white body is not pure white", label)
		}
		if darkBody > 0 {
			t.Fatalf("%s: white piece uses black", label)
		}
		return
	}
	if darkBody == 0 {
		t.Fatalf("%s: black body is not pure black", label)
	}
	if lightBody > 0 {
		t.Fatalf("%s: black piece uses white", label)
	}
}

func TestWhiteBodyIsLight(t *testing.T) {
	s := mustPos(t, "4k3/8/3p4/8/8/8/4P3/4K3 w - - 0 1")
	st := theme.For(theme.Wood, true)
	m := interact.New(chess.A1, chess.NoColor)
	ctx := cellCtx{session: s, machine: m, styles: st}
	for _, sz := range [][2]int{{sizeW, sizeH}} {
		white := squareANSI(ctx, chess.E2, sz[0], sz[1])
		black := squareANSI(ctx, chess.D6, sz[0], sz[1])
		assertSideColors(t, white, true, "sprite white")
		assertSideColors(t, black, false, "sprite black")
		if blocks := strings.Count(plainText(white), "█"); strings.Count(white, "38;2;255;255;255") < blocks {
			t.Fatalf("%dx%d white blocks are not pure white", sz[0], sz[1])
		}
		if blocks := strings.Count(plainText(black), "█"); strings.Count(black, "38;2;0;0;0") < blocks {
			t.Fatalf("%dx%d black blocks are not pure black", sz[0], sz[1])
		}
	}
	compact := ctx.renderCell(chess.E2, ctx.glyph(chess.E2), 3)
	if !strings.Contains(compact, "38;2;255;255;255") || !strings.Contains(compact, "48;2;20;17;14") {
		t.Fatalf("compact white glyph is not white on a dark pill: %q", plainText(compact))
	}
	mono := cellCtx{session: s, machine: m, styles: theme.For(theme.Mono, true)}
	whiteMono := plainText(squareANSI(mono, chess.E2, sizeW, sizeH))
	blackMono := plainText(squareANSI(mono, chess.D6, sizeW, sizeH))
	if whiteMono == blackMono || !strings.Contains(whiteMono, "▒") || !strings.Contains(blackMono, "█") {
		t.Fatalf("mono bodies match:\n%s\n%s", whiteMono, blackMono)
	}
	if strings.Contains(whiteMono, "█") || strings.Contains(blackMono, "▒") {
		t.Fatalf("mono white is ▒ and mono black is the solid blocks:\n%s\n%s", whiteMono, blackMono)
	}
}

func TestOutlineReadsOnCreamAndHighlight(t *testing.T) {
	s := mustPos(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	st := theme.For(theme.Wood, true)
	m := interact.New(chess.A1, chess.NoColor)
	ctx := cellCtx{session: s, machine: m, styles: st}
	for _, sz := range [][2]int{{sizeW, sizeH}} {
		for _, sq := range []chess.Square{chess.D1, chess.B1, chess.H1} {
			ansi := squareANSI(ctx, sq, sz[0], sz[1])
			assertSideColors(t, ansi, true, "cream "+sq.String())
			if !strings.Contains(ansi, "48;2;176;138;100") {
				t.Fatalf("%s %dx%d lost the light square", sq, sz[0], sz[1])
			}
		}
		// Black knight on b8 sits on a dark square.
		black := squareANSI(ctx, chess.B8, sz[0], sz[1])
		assertSideColors(t, black, false, "dark b8")
		if !strings.Contains(black, "48;2;107;79;58") {
			t.Fatalf("b8 %dx%d lost the dark square", sz[0], sz[1])
		}
		m.HasSel = true
		m.Selected = chess.G1
		selected := squareANSI(ctx, chess.G1, sz[0], sz[1])
		assertSideColors(t, selected, true, "selected g1")
		if !strings.Contains(selected, "48;2;205;210;106") {
			t.Fatalf("selected knight %dx%d lost the yellow highlight", sz[0], sz[1])
		}
		m.HasSel = false
		m.Cursor = chess.D1
		cursor := squareANSI(ctx, chess.D1, sz[0], sz[1])
		assertSideColors(t, cursor, true, "cursor d1")
		if !strings.Contains(cursor, "48;2;242;242;122") {
			t.Fatalf("cursor queen %dx%d lost the yellow highlight", sz[0], sz[1])
		}
		m.Cursor = chess.A1
	}
}

func squareBG(sq chess.Square) string {
	if darkSquare(sq) {
		return "48;2;107;79;58"
	}
	return "48;2;176;138;100"
}

func mustPos(t *testing.T, fen string) *game.Session {
	t.Helper()
	s, err := game.New(game.Options{FEN: fen, TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLargeCellCentersPiece(t *testing.T) {
	s, err := game.New(game.Options{TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	m := interact.New(chess.A1, chess.NoColor)
	ctx := cellCtx{session: s, machine: m, styles: theme.For(theme.Wood, true)}
	cell := ctx.renderCell(chess.E2, ctx.glyph(chess.E2), 5)
	plain := plainText(cell)
	if plain != "  ♟  " {
		t.Fatalf("large cell %q", plain)
	}
	if lipWidth(cell) != 5 {
		t.Fatalf("width %d", lipWidth(cell))
	}
	if !strings.Contains(cell, "48;2;") && !strings.Contains(cell, "48;5;") {
		t.Fatal("square has no background colour")
	}
}

func TestGlyphCellLayout(t *testing.T) {
	s := mustPos(t, "4k3/8/3p4/8/8/8/4P3/4K3 w - - 0 1")
	ctx := cellCtx{session: s, machine: interact.New(chess.A1, chess.NoColor), styles: theme.For(theme.Wood, true)}

	one := pieceStyle(chess.WhitePawn).Render(textPiece("♟"))
	if lipWidth(one) != 1 {
		t.Fatalf("styled glyph width %d, want 1: %q", lipWidth(one), one)
	}

	var shapes []string
	for _, sz := range [][2]int{{sizeW, sizeH}} {
		w, h := sz[0], sz[1]
		for _, pc := range []struct {
			sq    chess.Square
			white bool
		}{
			{chess.E2, true},
			{chess.D6, false},
		} {
			var plain strings.Builder
			for sub := 0; sub < h; sub++ {
				line := ctx.renderSquareLine(pc.sq, sub, w, h)
				if got := lipWidth(line); got != w {
					t.Fatalf("%s %dx%d row %d width %d", pc.sq, w, h, sub, got)
				}
				text := plainText(line)
				if len([]rune(text)) != w {
					t.Fatalf("%s %dx%d row %d %q", pc.sq, w, h, sub, text)
				}
				plain.WriteString(text)
				plain.WriteByte('\n')
			}
			body := plain.String()
			if !containsArt(body) {
				t.Fatalf("%s %dx%d has no silhouette:\n%s", pc.sq, w, h, body)
			}
			joined := ""
			for sub := 0; sub < h; sub++ {
				joined += ctx.renderSquareLine(pc.sq, sub, w, h)
			}
			assertSideColors(t, joined, pc.white, pc.sq.String())
			if !strings.Contains(ctx.renderSquareLine(pc.sq, 0, w, h), squareBG(pc.sq)) &&
				!strings.Contains(joined, squareBG(pc.sq)) {
				t.Fatalf("%s %dx%d lost the square margin", pc.sq, w, h)
			}
			shapes = append(shapes, joined)
		}
		if shapes[len(shapes)-1] == shapes[len(shapes)-2] {
			t.Fatalf("%dx%d white and black pawns match", w, h)
		}
	}

	mono := cellCtx{session: s, machine: interact.New(chess.E2, chess.NoColor), styles: theme.For(theme.Mono, true)}
	white := []rune(plainText(mono.renderSquareLine(chess.E2, axisCenter(sizeH), sizeW, sizeH)))
	if white[0] != '[' || white[len(white)-1] != ']' {
		t.Fatalf("mono white brackets %q", string(white))
	}
	black := plainText(mono.renderSquareLine(chess.D6, axisCenter(sizeH), sizeW, sizeH))
	if strings.Contains(black, "[") || strings.Contains(black, "]") {
		t.Fatalf("mono black layout %q", black)
	}
	whiteBody := plainText(squareANSI(mono, chess.E2, sizeW, sizeH))
	blackBody := plainText(squareANSI(mono, chess.D6, sizeW, sizeH))
	if (!containsArt(whiteBody) && !strings.Contains(whiteBody, "▒")) || !containsArt(blackBody) || stripBrackets(whiteBody) == blackBody {
		t.Fatalf("mono pieces:\n%s\n%s", whiteBody, blackBody)
	}
}

func stripBrackets(s string) string {
	return strings.NewReplacer("[", " ", "]", " ").Replace(s)
}

func plainText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\uFE0E' || r == '\uFE0F' {
			return -1
		}
		return r
	}, stripSGR(s))
}

func stripSGR(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == ';' || s[i] == ':') {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func TestMonoCursorBrackets(t *testing.T) {
	s, err := game.New(game.Options{TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	m := interact.New(chess.E2, chess.NoColor)
	wood := cellCtx{session: s, machine: m, styles: theme.For(theme.Wood, true), mono: true}
	if g := wood.glyph(chess.E2); !strings.HasPrefix(g, "[") || !strings.HasSuffix(g, "]") {
		t.Fatalf("colorless cursor glyph %q", g)
	}
	mono := cellCtx{session: s, machine: m, styles: theme.For(theme.Mono, true)}
	if g := mono.glyph(chess.E2); !strings.Contains(g, "[") {
		t.Fatalf("mono cursor glyph %q", g)
	}
}

func TestGameOverShowsResultOnce(t *testing.T) {
	a := testApp(t)
	s, err := game.New(game.Options{TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
		mv, err := (chess.UCINotation{}).Decode(s.Game().Position(), u)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Play(mv); err != nil {
			t.Fatal(err)
		}
	}
	a.useSession(s)
	view := a.View().Content
	if n := strings.Count(view, "Checkmate"); n != 1 {
		t.Fatalf("Checkmate appears %d times:\n%s", n, view)
	}
	if strings.Contains(view, "select") || strings.Contains(view, "cancel") {
		t.Fatalf("game-over help still offers move keys:\n%s", view)
	}
	if n := strings.Count(view, "new game"); n != 1 {
		t.Fatalf("new game hint appears %d times:\n%s", n, view)
	}
}
