package game

import (
	"strings"
	"testing"
	"time"

	"github.com/notnil/chess"
)

func local() *Session {
	s, err := New(Options{
		Mode:      ModeLocal,
		WhiteName: "Ada",
		BlackName: "Grace",
		TC:        PresetByID("5+0"),
	})
	if err != nil {
		panic(err)
	}
	return s
}

func mustMove(t *testing.T, s *Session, uci string) {
	t.Helper()
	m, err := (chess.UCINotation{}).Decode(s.g.Position(), uci)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Play(m); err != nil {
		t.Fatalf("play %s: %v", uci, err)
	}
}

func TestUndoLocalAndComputer(t *testing.T) {
	s := local()
	mustMove(t, s, "e2e4")
	mustMove(t, s, "e7e5")
	if _, err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if s.MoveCount() != 1 {
		t.Fatalf("local undo went to %d plies, want 1", s.MoveCount())
	}
	if s.Turn() != chess.Black {
		t.Fatalf("turn = %s", s.Turn())
	}

	c, err := New(Options{Mode: ModeComputer, Human: chess.White, TC: PresetByID("none"), Elo: 1500})
	if err != nil {
		t.Fatal(err)
	}
	mustMove(t, c, "e2e4")
	mustMove(t, c, "e7e5")
	mustMove(t, c, "g1f3")
	if _, err := c.Undo(); err != nil {
		t.Fatal(err)
	}
	// A pair: Nf3 and e5 come off, e4 remains.
	if c.MoveCount() != 1 {
		t.Fatalf("computer undo went to %d plies, want 1", c.MoveCount())
	}
	if _, err := c.Undo(); err != nil {
		t.Fatal(err)
	}
	if c.MoveCount() != 0 {
		t.Fatalf("second undo left %d plies", c.MoveCount())
	}

	mustMove(t, c, "e2e4")
	c.g.Resign(chess.Black)
	if _, err := c.Undo(); err == nil {
		t.Fatal("undo after game end should fail")
	}
}

func TestUndoRestoresClock(t *testing.T) {
	s, err := New(Options{Mode: ModeLocal, TC: Custom(1, 1)})
	if err != nil {
		t.Fatal(err)
	}
	before := s.Clock().Remaining(chess.White)
	mustMove(t, s, "e2e4")
	if s.Clock().Remaining(chess.White) != before+time.Second {
		t.Fatalf("increment not applied: %s", s.Clock().Remaining(chess.White))
	}
	if _, err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if s.Clock().Remaining(chess.White) != before {
		t.Fatalf("undo clock = %s, want %s", s.Clock().Remaining(chess.White), before)
	}
}

func TestPGNRoundTrip(t *testing.T) {
	s := local()
	s.autoFlip = true
	mustMove(t, s, "e2e4")
	mustMove(t, s, "e7e5")
	mustMove(t, s, "g1f3")
	pgn := s.PGN()
	for _, h := range []string{"[Event \"Termchess\"]", "[White \"Ada\"]", "[Black \"Grace\"]", "[TimeControl \"300+0\"]", "e4", "e5", "Nf3"} {
		if !strings.Contains(pgn, h) {
			t.Fatalf("pgn missing %q\n%s", h, pgn)
		}
	}
	loaded, err := LoadPGN(pgn)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.g.FEN() != s.g.FEN() {
		t.Fatalf("fen %s\nwant %s", loaded.g.FEN(), s.g.FEN())
	}
	if loaded.whiteName != "Ada" || loaded.blackName != "Grace" {
		t.Fatalf("names %s %s", loaded.whiteName, loaded.blackName)
	}
	if !loaded.autoFlip || loaded.tc.PGNString() != "300+0" {
		t.Fatalf("settings flip=%v tc=%s", loaded.autoFlip, loaded.tc.PGNString())
	}
	if loaded.MoveCount() != 3 {
		t.Fatalf("plies %d", loaded.MoveCount())
	}
	hist := SANHistory(loaded.g)
	if !strings.Contains(hist, "e4") || !strings.Contains(hist, "Nf3") {
		t.Fatalf("history %q", hist)
	}
}

func TestMaterialSurvivesPromotion(t *testing.T) {
	// White queen takes the pawn, the black king takes the queen, then the
	// b-pawn promotes. The captured queen must stay on Black's list, and the
	// score must follow the pieces on the board (a new queen is +9, not a
	// restored starting queen).
	s, err := New(Options{FEN: "k7/pP6/8/8/Q7/8/8/4K3 w - - 0 1", TC: PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	mustMove(t, s, "a4a7")
	white, black, diff := Captured(s.Game())
	// Queen on a7 (9) plus the pawn still on b7 (1).
	if len(white) != 1 || white[0].Type() != chess.Pawn || len(black) != 0 || diff != 10 {
		t.Fatalf("after Qxa7: white %v black %v diff %d", white, black, diff)
	}
	mustMove(t, s, "a8a7")
	white, black, diff = Captured(s.Game())
	if len(black) != 1 || black[0].Type() != chess.Queen || diff != 1 {
		t.Fatalf("after Kxa7: white %v black %v diff %d", white, black, diff)
	}
	mustMove(t, s, "b7b8q")
	white, black, diff = Captured(s.Game())
	if len(black) != 1 || black[0].Type() != chess.Queen {
		t.Fatalf("promotion dropped the captured queen: %v", black)
	}
	if len(white) != 1 || white[0].Type() != chess.Pawn {
		t.Fatalf("white captures %v", white)
	}
	if diff != 9 {
		t.Fatalf("diff after promotion = %d, want 9", diff)
	}
}

func TestEloHeaderAndStatus(t *testing.T) {
	s, err := New(Options{
		Mode:      ModeComputer,
		Human:     chess.White,
		WhiteName: "You",
		BlackName: "Hypersion (1650)",
		Elo:       1650,
		TC:        PresetByID("none"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.StatusLine(true, false), "Hypersion (1650)") {
		t.Fatalf("status %q", s.StatusLine(true, false))
	}
	pgn := s.PGN()
	for _, h := range []string{`[Black "Hypersion (1650)"]`, `[Elo "1650"]`, `[BlackElo "1650"]`} {
		if !strings.Contains(pgn, h) {
			t.Fatalf("pgn missing %s\n%s", h, pgn)
		}
	}
	loaded, err := LoadPGN(pgn)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Elo() != 1650 || loaded.Mode() != ModeComputer {
		t.Fatalf("loaded elo %d mode %s", loaded.Elo(), loaded.Mode())
	}
}

func TestCheckmateStalemateAndRules(t *testing.T) {
	// Fool's mate.
	s := local()
	for _, u := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
		mustMove(t, s, u)
	}
	if s.Method() != chess.Checkmate || s.Outcome() != chess.BlackWon {
		t.Fatalf("fool's mate: %s %s", s.Outcome(), s.ResultText())
	}
	if !strings.Contains(s.ResultText(), "Checkmate") {
		t.Fatal(s.ResultText())
	}

	// 1. e4 f6 2. Qh5+ leaves the black king in check (f7 is empty).
	s = local()
	mustMove(t, s, "e2e4")
	mustMove(t, s, "f7f6")
	mustMove(t, s, "d1h5")
	if !s.InCheck() {
		t.Fatal("expected check after Qh5+")
	}

	// En passant is a legal library move.
	s = local()
	for _, u := range []string{"e2e4", "a7a6", "e4e5", "d7d5"} {
		mustMove(t, s, u)
	}
	var ep *chess.Move
	for _, m := range s.g.ValidMoves() {
		if m.HasTag(chess.EnPassant) {
			ep = m
		}
	}
	if ep == nil {
		t.Fatal("expected an en passant move")
	}
	if _, err := s.Play(ep); err != nil {
		t.Fatal(err)
	}
	whiteCaps, _, _ := Captured(s.Game())
	if len(whiteCaps) != 1 || whiteCaps[0].Type() != chess.Pawn {
		t.Fatalf("en passant should record the pawn, got %v", whiteCaps)
	}

	// Castling.
	s, err := New(Options{FEN: "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", TC: PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	mustMove(t, s, "e1g1")
	if s.Piece(chess.G1).Type() != chess.King || s.Piece(chess.F1).Type() != chess.Rook {
		t.Fatal("castle did not move king and rook")
	}

	// Fifty-move claim from a FEN the library does not auto-draw.
	s, err = New(Options{FEN: "7k/8/8/8/8/8/P7/7K w - - 100 80", TC: PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.ClaimableDraw(); !ok {
		t.Fatal("expected fifty-move claim")
	}
	if err := s.ClaimDraw(); err != nil {
		t.Fatal(err)
	}
	if s.Method() != chess.FiftyMoveRule {
		t.Fatalf("method %v", s.Method())
	}
}

func TestFlagInsufficientMaterial(t *testing.T) {
	// White has a queen, black has a lone king. If white flags, black cannot mate.
	s, err := New(Options{FEN: "7k/8/8/8/8/8/8/4K2Q w - - 0 1", TC: Custom(1, 0)})
	if err != nil {
		t.Fatal(err)
	}
	s.Flag(chess.White)
	if !s.flagDraw || s.Outcome() != chess.Draw {
		t.Fatalf("white flag vs lone king: draw=%v outcome=%s text=%s", s.flagDraw, s.Outcome(), s.ResultText())
	}
	if _, err := s.Undo(); err == nil {
		t.Fatal("undo after flag should fail")
	}

	s, err = New(Options{FEN: "7k/8/8/8/8/8/8/4K2Q w - - 0 1", TC: Custom(1, 0)})
	if err != nil {
		t.Fatal(err)
	}
	s.Flag(chess.Black)
	if s.Outcome() != chess.WhiteWon || !s.timedOut {
		t.Fatalf("black flag should lose: %s %s", s.Outcome(), s.ResultText())
	}
	if !strings.Contains(s.ResultText(), "flagged") {
		t.Fatal(s.ResultText())
	}
}

func TestPromotionAndThreefold(t *testing.T) {
	s, err := New(Options{FEN: "k7/4P3/8/8/8/8/8/4K3 w - - 0 1", TC: PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	mustMove(t, s, "e7e8q")
	if s.Piece(chess.E8) != chess.WhiteQueen {
		t.Fatalf("promotion piece = %s", s.Piece(chess.E8))
	}

	// Knight out and back, twice, repeats the start position three times
	// (initial + two returns).
	s = local()
	for i := 0; i < 2; i++ {
		mustMove(t, s, "b1c3")
		mustMove(t, s, "b8c6")
		mustMove(t, s, "c3b1")
		mustMove(t, s, "c6b8")
	}
	method, ok := s.ClaimableDraw()
	if !ok || method != chess.ThreefoldRepetition {
		t.Fatalf("claimable = %v %v", method, ok)
	}
}

func TestPremoveMatch(t *testing.T) {
	s := local()
	mustMove(t, s, "e2e4")
	if s.FindPremove(chess.E7, chess.E5, chess.NoPieceType) == nil {
		t.Fatal("e7e5 should be legal for black")
	}
	if s.FindPremove(chess.E7, chess.E4, chess.NoPieceType) != nil {
		t.Fatal("e7e4 is not legal")
	}
}
