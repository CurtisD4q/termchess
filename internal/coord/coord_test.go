package coord

import (
	"testing"

	"github.com/notnil/chess"
)

func TestMoveScreenRelative(t *testing.T) {
	// White at the bottom: up from e2 is e3.
	next, ok := Move(chess.E2, Up, false)
	if !ok || next != chess.E3 {
		t.Fatalf("up from e2 = %s ok=%v, want e3", next, ok)
	}
	next, ok = Move(chess.E2, Left, false)
	if !ok || next != chess.D2 {
		t.Fatalf("left from e2 = %s, want d2", next)
	}
	next, ok = Move(chess.E2, Right, false)
	if !ok || next != chess.F2 {
		t.Fatalf("right from e2 = %s, want f2", next)
	}
	next, ok = Move(chess.E2, Down, false)
	if !ok || next != chess.E1 {
		t.Fatalf("down from e2 = %s, want e1", next)
	}

	// Black at the bottom. Up still means toward the top of the screen,
	// which is now toward white's back rank: e2 -> e1.
	next, ok = Move(chess.E2, Up, true)
	if !ok || next != chess.E1 {
		t.Fatalf("flipped up from e2 = %s ok=%v, want e1", next, ok)
	}
	next, ok = Move(chess.E2, Down, true)
	if !ok || next != chess.E3 {
		t.Fatalf("flipped down from e2 = %s, want e3", next)
	}
	next, ok = Move(chess.E2, Left, true)
	if !ok || next != chess.F2 {
		t.Fatalf("flipped left from e2 = %s, want f2", next)
	}
	next, ok = Move(chess.E2, Right, true)
	if !ok || next != chess.D2 {
		t.Fatalf("flipped right from e2 = %s, want d2", next)
	}
}

func TestMoveStopsAtEdge(t *testing.T) {
	next, ok := Move(chess.A8, Up, false)
	if ok || next != chess.A8 {
		t.Fatalf("up from a8 (white at bottom) = %s ok=%v, want stay", next, ok)
	}
	next, ok = Move(chess.H1, Down, false)
	if ok || next != chess.H1 {
		t.Fatalf("down from h1 = %s ok=%v, want stay", next, ok)
	}
	next, ok = Move(chess.A1, Left, false)
	if ok || next != chess.A1 {
		t.Fatalf("left from a1 = %s ok=%v, want stay", next, ok)
	}
	next, ok = Move(chess.H8, Right, false)
	if ok || next != chess.H8 {
		t.Fatalf("right from h8 = %s ok=%v, want stay", next, ok)
	}

	// When flipped, a8 is on the bottom rank visually, so up is allowed
	// and down is not.
	next, ok = Move(chess.A8, Up, true)
	if !ok || next != chess.A7 {
		t.Fatalf("flipped up from a8 = %s ok=%v, want a7", next, ok)
	}
	next, ok = Move(chess.A8, Down, true)
	if ok || next != chess.A8 {
		t.Fatalf("flipped down from a8 = %s ok=%v, want stay", next, ok)
	}
}

func TestVisualRoundTrip(t *testing.T) {
	for _, flipped := range []bool{false, true} {
		for sq := chess.A1; sq <= chess.H8; sq++ {
			row, col := Visual(sq, flipped)
			if got := Square(row, col, flipped); got != sq {
				t.Fatalf("round trip flipped=%v sq=%s -> %d,%d -> %s", flipped, sq, row, col, got)
			}
		}
	}
}
