package interact

import (
	"testing"

	"github.com/notnil/chess"
	"termchess/internal/coord"
	"termchess/internal/game"
)

func newLocal(t *testing.T) *game.Session {
	t.Helper()
	s, err := game.New(game.Options{Mode: game.ModeLocal, TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSelectionAndCursorFlip(t *testing.T) {
	s := newLocal(t)
	m := New(chess.E2, chess.NoColor)

	// Up the board with white at the bottom lands on e4 after two steps.
	m.MoveCursor(coord.Up, false)
	m.MoveCursor(coord.Up, false)
	if m.Cursor != chess.E4 {
		t.Fatalf("cursor %s", m.Cursor)
	}
	m.Cursor = chess.E2
	m.MoveCursor(coord.Up, true)
	if m.Cursor != chess.E1 {
		t.Fatalf("flipped cursor %s, want e1", m.Cursor)
	}

	m.Cursor = chess.E2
	res := m.Confirm(s)
	if res.Move != nil || m.Phase != PhaseSelected {
		t.Fatalf("select: %+v phase %s", res, m.Phase)
	}
	var sawE4, sawE3 bool
	for _, d := range m.Targets {
		if d.Sq == chess.E4 {
			sawE4 = true
		}
		if d.Sq == chess.E3 {
			sawE3 = true
		}
	}
	if !sawE3 || !sawE4 {
		t.Fatalf("pawn targets = %+v", m.Targets)
	}

	// Illegal square keeps the selection.
	m.Cursor = chess.D4
	res = m.Confirm(s)
	if res.Flash == "" || m.Phase != PhaseSelected || m.Selected != chess.E2 {
		t.Fatalf("illegal: %+v phase %s sel %s", res, m.Phase, m.Selected)
	}

	// Switching to another own piece.
	m.Cursor = chess.G1
	m.Confirm(s)
	if m.Selected != chess.G1 || m.Phase != PhaseSelected {
		t.Fatalf("switch sel %s phase %s", m.Selected, m.Phase)
	}

	m.Cancel()
	if m.Phase != PhaseIdle || m.HasSel {
		t.Fatalf("esc phase %s", m.Phase)
	}
}

func TestPlayMoveFromSelection(t *testing.T) {
	s := newLocal(t)
	m := New(chess.E2, chess.NoColor)
	m.Confirm(s)
	m.Cursor = chess.E4
	res := m.Confirm(s)
	if res.Move == nil || res.Move.S2() != chess.E4 {
		t.Fatalf("move = %v", res.Move)
	}
	if _, err := s.Play(res.Move); err != nil {
		t.Fatal(err)
	}
	if s.Piece(chess.E4).Type() != chess.Pawn {
		t.Fatal("pawn did not arrive")
	}
}

func TestPromotionFlow(t *testing.T) {
	s, err := game.New(game.Options{FEN: "k7/4P3/8/8/8/8/8/4K3 w - - 0 1", TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	m := New(chess.E7, chess.NoColor)
	m.Confirm(s)
	m.MoveCursor(coord.Up, false)
	if m.Cursor != chess.E8 {
		t.Fatalf("cursor %s", m.Cursor)
	}
	res := m.Confirm(s)
	if res.Move != nil || m.Phase != PhasePromotion {
		t.Fatalf("expected promotion, got %+v %s", res, m.Phase)
	}
	if m.Promo[0] != chess.Queen {
		t.Fatalf("promo choices %v", m.Promo)
	}
	m.PromoRight()
	if m.Promo[m.PromoIndex] != chess.Rook {
		t.Fatalf("index piece %s", m.Promo[m.PromoIndex])
	}
	m.PromoLeft()
	res = m.Confirm(s)
	if res.Move == nil || res.Move.Promo() != chess.Queen || res.Move.S2() != chess.E8 {
		t.Fatalf("promo move %+v", res.Move)
	}
	// Esc from the picker cancels back to the selection.
	m = New(chess.E7, chess.NoColor)
	m.Confirm(s)
	m.Cursor = chess.E8
	m.Confirm(s)
	m.Cancel()
	if m.Phase != PhaseSelected || m.Selected != chess.E7 {
		t.Fatalf("cancel promo phase %s", m.Phase)
	}
}

func TestPremove(t *testing.T) {
	s := newLocal(t)
	m := New(chess.E2, chess.White)
	m.Confirm(s)
	m.Cursor = chess.E4
	res := m.Confirm(s)
	if _, err := s.Play(res.Move); err != nil {
		t.Fatal(err)
	}
	m.Cursor = res.Move.S2()
	m.BeginEngine()
	if m.Phase != PhaseEngine {
		t.Fatal(m.Phase)
	}
	// Normal enemy piece is not selectable as a real move; it starts a pre-move
	// only for the human's own pieces.
	m.Cursor = chess.E7
	if res := m.Confirm(s); res.Flash == "" {
		t.Fatal("black pawn should not be a white pre-move")
	}
	m.Cursor = chess.G1
	m.Confirm(s)
	if m.Phase != PhasePremove || m.Selected != chess.G1 {
		t.Fatalf("premove select %s %s", m.Phase, m.Selected)
	}
	m.Cursor = chess.F3
	res = m.Confirm(s)
	if !res.PremoveSet || m.Premove == nil || m.Premove.To != chess.F3 {
		t.Fatalf("premove %+v res %+v", m.Premove, res)
	}
	if m.Phase != PhaseEngine {
		t.Fatalf("phase after premove %s", m.Phase)
	}

	// Engine plays e5; the pre-move Nf3 is still legal.
	reply, err := (chess.UCINotation{}).Decode(s.Game().Position(), "e7e5")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Play(reply); err != nil {
		t.Fatal(err)
	}
	follow := s.FindPremove(m.Premove.From, m.Premove.To, m.Premove.Promo)
	if follow == nil || follow.S2() != chess.F3 {
		t.Fatalf("premove was not legal after e5: %v", follow)
	}

	// A pre-move that the engine's reply makes illegal is cleared by the caller.
	m.Premove = &Premove{From: chess.E4, To: chess.E5, Promo: chess.NoPieceType}
	if s.FindPremove(m.Premove.From, m.Premove.To, m.Premove.Promo) != nil {
		t.Fatal("e4e5 should be illegal once black has played e5")
	}
}
