package ui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/notnil/chess"
	"golang.org/x/sys/unix"
	"termchess/internal/coord"
	"termchess/internal/game"
	"termchess/internal/interact"
	"termchess/internal/theme"
)

// TestStartPositionPieceColours renders the opening board and checks every
// occupied square from the parsed cells: piece glyphs use the side colour as
// foreground, that colour is not the background, and the tile background is
// the same on every cell of the square.
func TestStartPositionPieceColours(t *testing.T) {
	board, err := renderStartBoard(colorprofile.TrueColor)
	if err != nil {
		t.Fatal(err)
	}
	assertStartPieceColours(t, board, colorprofile.TrueColor)
}

// TestStartPositionOnKittyPTY runs the same render on a real pty whose
// environment is Kitty with true colour, then parses what the child wrote.
func TestStartPositionOnKittyPTY(t *testing.T) {
	if os.Getenv("TERMCHESS_PTY_CHILD") == "1" {
		runPTYChild()
		return
	}
	out, errText := runBoardOnKittyPTY(t)
	if errText != "" {
		t.Fatal(errText)
	}
	assertStartPieceColours(t, out, colorprofile.TrueColor)
}

func TestPieceColoursDownsample(t *testing.T) {
	for _, profile := range []colorprofile.Profile{colorprofile.ANSI256, colorprofile.ANSI} {
		t.Run(profile.String(), func(t *testing.T) {
			board, err := renderStartBoard(profile)
			if err != nil {
				t.Fatal(err)
			}
			sq := squareANSI(cellCtx{
				session: mustPos(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"),
				machine: interact.New(chess.A1, chess.NoColor),
				styles:  theme.For(theme.Wood, true),
				profile: profile,
			}, chess.E2, sizeW, sizeH)
			if strings.Contains(sq, "38;2;") || strings.Contains(sq, "48;2;") {
				t.Fatalf("%s square still contains a 24-bit colour: %q", profile, sq)
			}
			assertStartPieceColours(t, board, profile)
		})
	}
}

func renderStartBoard(profile colorprofile.Profile) (string, error) {
	s, err := game.New(game.Options{TC: game.PresetByID("none")})
	if err != nil {
		return "", err
	}
	ctx := cellCtx{
		session: s,
		machine: interact.New(chess.E2, chess.White),
		styles:  theme.For(theme.Wood, true),
		profile: profile,
	}
	return renderBoard(ctx, sizeW, sizeH, false), nil
}

func runPTYChild() {
	env := os.Environ()
	p := ProfileFor(ColorAuto, env)
	if p != colorprofile.TrueColor {
		fmt.Fprintf(os.Stderr, "profile %s\n", p)
		os.Exit(2)
	}
	if got := colorprofile.Detect(os.Stdout, env); got != colorprofile.TrueColor {
		fmt.Fprintf(os.Stderr, "detect %s\n", got)
		os.Exit(3)
	}
	board, err := renderStartBoard(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
	if _, err := io.WriteString(os.Stdout, board); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
	os.Exit(0)
}

func runBoardOnKittyPTY(t *testing.T) (string, string) {
	t.Helper()
	master, slave, err := openPTY()
	if err != nil {
		t.Fatalf("pty: %v", err)
	}
	defer master.Close()

	cmd := exec.Command(os.Args[0], "-test.run=^TestStartPositionOnKittyPTY$", "-test.count=1")
	cmd.Env = []string{
		"TERMCHESS_PTY_CHILD=1",
		"TERM=xterm-kitty",
		"COLORTERM=truecolor",
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
	cmd.Stdin = slave
	cmd.Stdout = slave
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		slave.Close()
		t.Fatal(err)
	}
	slave.Close()

	var out bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				out.Write(buf[:n])
			}
			if err != nil {
				// Linux reports EIO on the master once the slave side has
				// closed, after the bytes already written have been delivered.
				if errors.Is(err, io.EOF) || errors.Is(err, syscall.EIO) {
					readDone <- nil
					return
				}
				readDone <- err
				return
			}
		}
	}()
	waitErr := cmd.Wait()
	select {
	case err := <-readDone:
		if err != nil {
			return "", err.Error()
		}
	case <-time.After(5 * time.Second):
		return "", "timed out reading the pty"
	}
	if waitErr != nil {
		return "", fmt.Sprintf("%v\n%s", waitErr, stderr.String())
	}
	if stderr.Len() != 0 {
		return "", stderr.String()
	}
	return out.String(), ""
}

func openPTY() (master, slave *os.File, err error) {
	master, err = os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	if err = unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		master.Close()
		return nil, nil, err
	}
	n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		master.Close()
		return nil, nil, err
	}
	slave, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, nil, err
	}
	return master, slave, nil
}

type termCell struct {
	r            rune
	fg, bg       rgb
	hasFg, hasBg bool
	reverse      bool
}

func shownColour(c rgb, profile colorprofile.Profile) rgb {
	sample := paintTermCell('X', c, rgb{1, 2, 3}, profile)
	grid := parseANSIGrid(sample)
	if len(grid) == 0 || len(grid[0]) == 0 {
		return rgb{}
	}
	return grid[0][0].fg
}

func assertStartPieceColours(t *testing.T, board string, profile colorprofile.Profile) {
	t.Helper()
	if strings.Contains(board, "\x1b[7m") || strings.Contains(board, "\x1b[7;") {
		t.Fatal("board uses reverse video")
	}
	grid := parseANSIGrid(board)
	s, err := game.New(game.Options{TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			sq := coord.Square(row, col, false)
			p := s.Piece(sq)
			if p == chess.NoPiece {
				continue
			}
			body := shownColour(pieceInk(p), profile)
			var tile rgb
			var tileSet bool
			pieceN, bodyN := 0, 0
			for sub := 0; sub < sizeH; sub++ {
				lineNo := 1 + row*sizeH + sub
				if lineNo >= len(grid) {
					t.Fatalf("%s row %d is past the grid (%d lines)", sq, lineNo, len(grid))
				}
				line := grid[lineNo]
				for x := 0; x < sizeW; x++ {
					at := 3 + col*sizeW + x
					if at >= len(line) {
						t.Fatalf("%s line %d has %d cells", sq, lineNo, len(line))
					}
					cell := line[at]
					if !cell.hasFg || !cell.hasBg {
						t.Fatalf("%s cell %d,%d has no explicit colour: %+v", sq, x, sub, cell)
					}
					if cell.reverse {
						t.Fatalf("%s cell %d,%d is reverse video", sq, x, sub)
					}
					if !tileSet {
						tile = cell.bg
						tileSet = true
					} else if cell.bg != tile {
						t.Fatalf("%s background changes inside the square (%v then %v)", sq, tile, cell.bg)
					}
					if !isPieceRune(cell.r) {
						continue
					}
					pieceN++
					if cell.fg == cell.bg {
						t.Fatalf("%s piece cell fg == bg (%v) rune %q", sq, cell.fg, string(cell.r))
					}
					if cell.fg != body {
						t.Fatalf("%s piece fg %v, want %v", sq, cell.fg, body)
					}
					bodyN++
					if cell.bg == body {
						t.Fatalf("%s piece colour %v is the tile background", sq, cell.bg)
					}
				}
			}
			if pieceN == 0 || bodyN != pieceN {
				t.Fatalf("%s visible cells %d body %d", sq, pieceN, bodyN)
			}
		}
	}
}

func isPieceRune(r rune) bool {
	switch r {
	case '█', '▀', '▄', '▌', '▐', '░', '▒', '▓',
		'▘', '▝', '▖', '▗', '▚', '▞', '▙', '▛', '▜', '▟':
		return true
	default:
		return false
	}
}

type sgrPen struct {
	fg, bg       rgb
	hasFg, hasBg bool
	reverse      bool
}

func parseANSIGrid(s string) [][]termCell {
	var rows [][]termCell
	var line []termCell
	var pen sgrPen
	flush := func() {
		rows = append(rows, line)
		line = nil
	}
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := strings.IndexByte(s[i:], 'm')
			if j < 0 {
				break
			}
			applyCellSGR(s[i+2:i+j], &pen)
			i += j + 1
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r == '\n' {
			flush()
			pen = sgrPen{}
			continue
		}
		if r == '\r' {
			continue
		}
		line = append(line, termCell{
			r: r, fg: pen.fg, bg: pen.bg,
			hasFg: pen.hasFg, hasBg: pen.hasBg, reverse: pen.reverse,
		})
	}
	if len(line) > 0 {
		flush()
	}
	return rows
}

func applyCellSGR(params string, pen *sgrPen) {
	if params == "" || params == "0" {
		*pen = sgrPen{}
		return
	}
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}
		switch {
		case n == 0:
			*pen = sgrPen{}
		case n == 7:
			pen.reverse = true
		case n == 27:
			pen.reverse = false
		case n == 39:
			pen.hasFg = false
		case n == 49:
			pen.hasBg = false
		case n >= 30 && n <= 37:
			pen.fg, pen.hasFg = basicRGB(n-30), true
		case n >= 90 && n <= 97:
			pen.fg, pen.hasFg = basicRGB(n-90+8), true
		case n >= 40 && n <= 47:
			pen.bg, pen.hasBg = basicRGB(n-40), true
		case n >= 100 && n <= 107:
			pen.bg, pen.hasBg = basicRGB(n-100+8), true
		case n == 38 || n == 48:
			if i+1 >= len(parts) {
				return
			}
			var c rgb
			var ok bool
			switch parts[i+1] {
			case "2":
				if i+4 >= len(parts) {
					return
				}
				c = rgb{u8(parts[i+2]), u8(parts[i+3]), u8(parts[i+4])}
				ok = true
				i += 4
			case "5":
				if i+2 >= len(parts) {
					return
				}
				c = indexRGB(u8(parts[i+2]))
				ok = true
				i += 2
			default:
				i++
			}
			if !ok {
				continue
			}
			if n == 38 {
				pen.fg, pen.hasFg = c, true
			} else {
				pen.bg, pen.hasBg = c, true
			}
		}
	}
}

func u8(s string) uint8 {
	n, _ := strconv.Atoi(s)
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return uint8(n)
}

func basicRGB(n int) rgb {
	r, g, b, _ := ansi.BasicColor(n).RGBA()
	return rgb{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

func indexRGB(n uint8) rgb {
	r, g, b, _ := ansi.IndexedColor(n).RGBA()
	return rgb{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}
