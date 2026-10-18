package pieces

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/notnil/chess"
)

func TestPieceFillsMostOfTheSquare(t *testing.T) {
	for _, side := range []chess.Color{chess.White, chess.Black} {
		for _, pt := range Types() {
			w, h := FillSpan(Render(side, pt, 128))
			t.Logf("%s %s fill %.0f%% × %.0f%%", side, pt, w*100, h*100)
			if w < 0.75 || h < 0.75 || w > 0.92 || h > 0.92 {
				t.Fatalf("%s %s fills %.0f%% × %.0f%% of the square, want about 80%%", side, pt, w*100, h*100)
			}
		}
	}
}

func TestEmbeddedPiecesDiffer(t *testing.T) {
	white, err := EmbeddedPNG(chess.White, chess.Knight)
	if err != nil {
		t.Fatal(err)
	}
	black, err := EmbeddedPNG(chess.Black, chess.Knight)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(white, black) {
		t.Fatal("white and black knights are the same file")
	}
	img, err := png.Decode(bytes.NewReader(white))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 128 || img.Bounds().Dy() != 128 {
		t.Fatalf("bounds %v", img.Bounds())
	}
	svg := SVG(chess.White, chess.King)
	if !bytes.Contains([]byte(svg), []byte("<path")) || !bytes.Contains([]byte(svg), []byte("MIT")) {
		t.Fatalf("svg:\n%s", svg)
	}
	if ID(chess.White, chess.King) != 1 || ID(chess.Black, chess.Pawn) != 12 {
		t.Fatalf("ids %d %d", ID(chess.White, chess.King), ID(chess.Black, chess.Pawn))
	}
	sheet, err := Files.ReadFile("preview.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(sheet)); err != nil {
		t.Fatal(err)
	}
}
