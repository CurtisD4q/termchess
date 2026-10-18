package gfx

import (
	"strings"
	"testing"

	"github.com/notnil/chess"
	"termchess/assets/pieces"
)

func TestParseAndResolve(t *testing.T) {
	mode, err := Parse("sixel")
	if err != nil || mode != Sixel {
		t.Fatalf("sixel: mode %v err %v", mode, err)
	}
	if _, err := Parse("banana"); err == nil {
		t.Fatal("banana accepted")
	}
	if _, err := Parse("auto"); err == nil {
		t.Fatal("auto still selects a protocol")
	}
	kittyEnv := []string{
		"TERM=xterm-kitty",
		"KITTY_WINDOW_ID=1",
		"KITTY_PID=4242",
		"KITTY_PUBLIC_KEY=abc",
		"COLORTERM=truecolor",
	}
	got, err := Resolve("kitty", "off", nil)
	if err != nil || got != Kitty {
		t.Fatalf("flag wins: %v %v", got, err)
	}
	got, err = Resolve("", "iterm", kittyEnv)
	if err != nil || got != Off {
		t.Fatalf("config does not enable images: %v %v", got, err)
	}
	got, err = Resolve("", "kitty", kittyEnv)
	if err != nil || got != Off {
		t.Fatalf("saved kitty stays off: %v %v", got, err)
	}
	got, err = Resolve("", "auto", kittyEnv)
	if err != nil || got != Off {
		t.Fatalf("saved auto stays off: %v %v", got, err)
	}
	got, err = Resolve("", "off", kittyEnv)
	if err != nil || got != Off {
		t.Fatalf("default stays text: %v %v", got, err)
	}
	got, err = Resolve("off", "kitty", kittyEnv)
	if err != nil || got != Off {
		t.Fatalf("explicit off: %v %v", got, err)
	}
	if _, err := Resolve("auto", "off", kittyEnv); err == nil {
		t.Fatal("explicit auto still detects")
	}
	if _, err := Resolve("nope", "", nil); err == nil {
		t.Fatal("bad flag accepted")
	}
}

func TestKittyFrame(t *testing.T) {
	png := mustPNG(t)
	spot := Spot{Row: 22, Col: 32, Cols: 7, Rows: 3, ID: 6, PNG: png}
	// Two spots, one id: transmit and virtual placement happen once.
	first, sent := Frame(Kitty, []Spot{spot, spot}, nil)
	if !strings.Contains(first, "iVBORw0KGgo") {
		t.Fatal("transmit is missing the PNG")
	}
	if strings.Count(first, "a=t") != 1 {
		t.Fatalf("duplicate transmit:\n%s", first)
	}
	for _, want := range []string{"a=p,U=1,i=6,p=55,c=7,r=3,q=2", "f=100"} {
		if !strings.Contains(first, want) {
			t.Fatalf("missing %q\n%s", want, first)
		}
	}
	if strings.Contains(first, "a=d") || strings.Contains(first, "\x1b[22;32H") || strings.Contains(first, "z=-1") {
		t.Fatalf("frame still cursor-places or deletes:\n%s", first)
	}
	if strings.Index(first, "a=t") > strings.Index(first, "U=1") {
		t.Fatal("virtual placement was emitted before the transmit")
	}
	// Same grid on the next redraw: no sequences at all. The placeholders
	// already on screen keep the picture; a delete here would win the race.
	second, sent := Frame(Kitty, []Spot{spot}, sent)
	if second != "" {
		t.Fatalf("second frame emitted %q", second)
	}
	if _, ok := sent[6]; !ok {
		t.Fatal("image id was not remembered")
	}
	// A move that drops the spot emits nothing. The cell becoming a space is
	// what removes the picture.
	if moved, _ := Frame(Kitty, nil, sent); moved != "" {
		t.Fatalf("empty board deleted images: %q", moved)
	}
	// A resize creates a new virtual placement and does not resend the PNG.
	resized := spot
	resized.Cols, resized.Rows = 9, 4
	third, _ := Frame(Kitty, []Spot{resized}, sent)
	if strings.Contains(third, "a=t") || strings.Contains(third, "iVBORw0KGgo") || strings.Contains(third, "a=d") {
		t.Fatalf("resize retransmitted or deleted:\n%s", third)
	}
	if !strings.Contains(third, "a=p,U=1,i=6,p=73,c=9,r=4,q=2") {
		t.Fatalf("resize placement:\n%s", third)
	}
	if c := Cleanup(Kitty); c != "\x1b_Ga=d,d=A,q=2\x1b\\" {
		t.Fatalf("cleanup %q", c)
	}
	if Cleanup(Sixel) != "" || Cleanup(Off) != "" {
		t.Fatal("cleanup leaked onto a non-kitty protocol")
	}
}

func TestKittyPlaceholderCells(t *testing.T) {
	// The documented 2×2 grid: U+0305 is 0 and U+030D is 1.
	want := []struct {
		row, col int
		seq      string
	}{
		{0, 0, "\U0010EEEE\u0305\u0305"},
		{0, 1, "\U0010EEEE\u0305\u030D"},
		{1, 0, "\U0010EEEE\u030D\u0305"},
		{1, 1, "\U0010EEEE\u030D\u030D"},
	}
	for _, tc := range want {
		if got := Placeholder(tc.row, tc.col); got != tc.seq {
			t.Fatalf("(%d,%d) = %q, want %q", tc.row, tc.col, got, tc.seq)
		}
	}
	if PlacementID(8, 4) != 72 || PlacementID(7, 3) != 55 {
		t.Fatalf("placement ids %d %d", PlacementID(8, 4), PlacementID(7, 3))
	}
}

func TestITermAndSixel(t *testing.T) {
	png := mustPNG(t)
	spot := Spot{Row: 4, Col: 4, Cols: 2, Rows: 2, ID: 1, PNG: png}
	iterm, _ := Frame(ITerm, []Spot{spot}, nil)
	for _, want := range []string{"\x1b[4;4H", "\x1b]1337;File=inline=1;width=2;height=2;preserveAspectRatio=1:", "iVBORw0KGgo"} {
		if !strings.Contains(iterm, want) {
			t.Fatalf("iterm missing %q", want)
		}
	}
	sixel, _ := Frame(Sixel, []Spot{spot}, nil)
	if !strings.Contains(sixel, "\x1bPq\"1;1;16;32") || !strings.Contains(sixel, "\x1b\\") {
		t.Fatalf("sixel:\n%s", sixel)
	}
	if FrameOff, _ := Frame(Off, []Spot{spot}, nil); FrameOff != "" {
		t.Fatal("off mode emitted graphics")
	}
}

func mustPNG(t *testing.T) []byte {
	t.Helper()
	b, err := pieces.PNG(chess.White, chess.Pawn, 32)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
