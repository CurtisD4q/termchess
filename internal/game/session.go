// Package game wraps github.com/notnil/chess with clocks, undo, draws, and PGN.
// Rules (legality, check, mate, stalemate, repetition, the 50-move rule,
// insufficient material, castling, en passant, promotion) all come from that
// library.
package game

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/notnil/chess"
	"termchess/internal/clock"
)

// Mode is who is sitting at the board.
type Mode int

const (
	ModeLocal Mode = iota
	ModeComputer
)

func (m Mode) String() string {
	if m == ModeComputer {
		return "computer"
	}
	return "local"
}

// TimeControl is a preset or a custom clock.
type TimeControl struct {
	ID        string
	Name      string
	Initial   time.Duration
	Increment time.Duration
}

// Presets offered on the setup form. Custom is filled in by the caller.
func Presets() []TimeControl {
	return []TimeControl{
		{ID: "none", Name: "None"},
		{ID: "1+0", Name: "Bullet 1+0", Initial: time.Minute},
		{ID: "3+2", Name: "Blitz 3+2", Initial: 3 * time.Minute, Increment: 2 * time.Second},
		{ID: "5+0", Name: "Blitz 5+0", Initial: 5 * time.Minute},
		{ID: "10+0", Name: "Rapid 10+0", Initial: 10 * time.Minute},
		{ID: "15+10", Name: "Rapid 15+10", Initial: 15 * time.Minute, Increment: 10 * time.Second},
		{ID: "custom", Name: "Custom"},
	}
}

// PresetByID returns a named control. Unknown ids become untimed.
func PresetByID(id string) TimeControl {
	for _, p := range Presets() {
		if p.ID == id {
			return p
		}
	}
	return Presets()[0]
}

// Custom builds a time control from minutes and increment seconds.
func Custom(minutes, incSeconds int) TimeControl {
	if minutes < 0 {
		minutes = 0
	}
	if incSeconds < 0 {
		incSeconds = 0
	}
	return TimeControl{
		ID:        "custom",
		Name:      fmt.Sprintf("Custom %d+%d", minutes, incSeconds),
		Initial:   time.Duration(minutes) * time.Minute,
		Increment: time.Duration(incSeconds) * time.Second,
	}
}

// PGNString is the TimeControl tag value ("-" when untimed).
func (tc TimeControl) PGNString() string {
	if tc.Initial <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d+%d", int(tc.Initial.Seconds()), int(tc.Increment.Seconds()))
}

// Options configures a new session.
type Options struct {
	Mode      Mode
	Human     chess.Color
	WhiteName string
	BlackName string
	TC        TimeControl
	AutoFlip  bool
	Elo       int
	FEN       string
}

// Session is one game.
type Session struct {
	g          *chess.Game
	mode       Mode
	human      chess.Color
	whiteName  string
	blackName  string
	tc         TimeControl
	clk        *clock.Clock
	autoFlip   bool
	manualFlip bool
	elo        int
	undo       []undoSnap
	drawOffer  chess.Color
	unsaved    bool
	timedOut   bool
	flagDraw   bool
	flagColor  chess.Color
	// resigned is the side that resigned, or NoColor. It is restored from
	// the Termination tag when a saved game is loaded.
	resigned chess.Color
}

type undoSnap struct {
	g     *chess.Game
	clock clock.Snapshot
}

// New starts a game from the standard position or a FEN.
func New(opt Options) (*Session, error) {
	var g *chess.Game
	if opt.FEN != "" {
		fn, err := chess.FEN(opt.FEN)
		if err != nil {
			return nil, err
		}
		g = chess.NewGame(fn)
	} else {
		g = chess.NewGame()
	}
	if opt.WhiteName == "" {
		opt.WhiteName = "White"
	}
	if opt.BlackName == "" {
		opt.BlackName = "Black"
	}
	s := &Session{
		g:         g,
		mode:      opt.Mode,
		human:     opt.Human,
		whiteName: opt.WhiteName,
		blackName: opt.BlackName,
		tc:        opt.TC,
		clk:       clock.New(opt.TC.Initial, opt.TC.Increment),
		autoFlip:  opt.AutoFlip,
		elo:       opt.Elo,
		drawOffer: chess.NoColor,
	}
	return s, nil
}

// LoadPGN restores a saved game, including clock readings stored in tags.
func LoadPGN(data string) (*Session, error) {
	fn, err := chess.PGN(strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	g := chess.NewGame(fn)
	s := &Session{
		g:         g,
		whiteName: "White",
		blackName: "Black",
		drawOffer: chess.NoColor,
		human:     chess.NoColor,
	}
	if tp := g.GetTagPair("White"); tp != nil && tp.Value != "" {
		s.whiteName = tp.Value
	}
	if tp := g.GetTagPair("Black"); tp != nil && tp.Value != "" {
		s.blackName = tp.Value
	}
	if tp := g.GetTagPair("Mode"); tp != nil && tp.Value == ModeComputer.String() {
		s.mode = ModeComputer
	}
	if tp := g.GetTagPair("AutoFlip"); tp != nil && tp.Value == "true" {
		s.autoFlip = true
	}
	if tp := g.GetTagPair("HumanColor"); tp != nil {
		switch tp.Value {
		case "w":
			s.human = chess.White
		case "b":
			s.human = chess.Black
		}
	}
	if tp := g.GetTagPair("Elo"); tp != nil {
		if n, err := strconv.Atoi(tp.Value); err == nil {
			s.elo = n
		}
	}
	if tp := g.GetTagPair("Termination"); tp != nil {
		switch tp.Value {
		case "White resigns":
			s.resigned = chess.White
		case "Black resigns":
			s.resigned = chess.Black
		}
	}
	s.tc = parseTC(tag(g, "TimeControl"))
	whiteRem, blackRem := s.tc.Initial, s.tc.Initial
	if ms, ok := parseMS(tag(g, "WhiteClock")); ok {
		whiteRem = ms
	}
	if ms, ok := parseMS(tag(g, "BlackClock")); ok {
		blackRem = ms
	}
	s.clk = clock.New(s.tc.Initial, s.tc.Increment)
	if s.tc.Initial <= 0 {
		s.clk = clock.New(0, 0)
	} else {
		// New() sized both timers to the original control. Restore each side.
		_ = s.clk.Apply(clock.Snapshot{
			White:   whiteRem,
			Black:   blackRem,
			Active:  g.Position().Turn(),
			Running: false,
		})
	}
	if g.Outcome() != chess.NoOutcome {
		_ = s.clk.Halt()
	}
	return s, nil
}

func tag(g *chess.Game, k string) string {
	tp := g.GetTagPair(k)
	if tp == nil {
		return ""
	}
	return tp.Value
}

func parseMS(s string) (time.Duration, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(n) * time.Millisecond, true
}

func parseTC(s string) TimeControl {
	if s == "" || s == "-" {
		return PresetByID("none")
	}
	base, inc := s, "0"
	if i := strings.IndexByte(s, '+'); i >= 0 {
		base, inc = s[:i], s[i+1:]
	}
	b, err1 := strconv.Atoi(base)
	n, err2 := strconv.Atoi(inc)
	if err1 != nil || err2 != nil {
		return PresetByID("none")
	}
	tc := TimeControl{
		ID:        s,
		Name:      s,
		Initial:   time.Duration(b) * time.Second,
		Increment: time.Duration(n) * time.Second,
	}
	for _, p := range Presets() {
		if p.PGNString() == tc.PGNString() && p.ID != "custom" && p.ID != "none" {
			return p
		}
	}
	tc.ID = "custom"
	tc.Name = "Custom " + s
	return tc
}

// Game is the underlying library game.
func (s *Session) Game() *chess.Game { return s.g }

// Mode returns local or computer.
func (s *Session) Mode() Mode { return s.mode }

// DemoteToLocal turns a computer game into pass-and-play (used when Hypersion
// is missing at load time).
func (s *Session) DemoteToLocal() {
	s.mode = ModeLocal
	s.human = chess.NoColor
}

// Human is the player's color against the computer, or NoColor for two-player.
func (s *Session) Human() chess.Color { return s.human }

// Elo is the engine rating chosen for a computer game, or 0 for two-player.
func (s *Session) Elo() int { return s.elo }

// Names returns the White and Black header names.
func (s *Session) Names() (string, string) { return s.whiteName, s.blackName }

// TimeControl returns the clock settings.
func (s *Session) TimeControl() TimeControl { return s.tc }

// Clock is the live clock.
func (s *Session) Clock() *clock.Clock { return s.clk }

// AutoFlip reports whether pass-and-play rotates the board each turn.
func (s *Session) AutoFlip() bool { return s.autoFlip }

// SetManualFlip toggles the extra orientation flip (the `f` key).
func (s *Session) SetManualFlip(v bool) { s.manualFlip = v }

// ManualFlip is the extra flip.
func (s *Session) ManualFlip() bool { return s.manualFlip }

// ToggleFlip flips the board relative to the automatic orientation.
func (s *Session) ToggleFlip() { s.manualFlip = !s.manualFlip }

// BoardFlipped reports whether black's back rank is at the bottom of the screen.
func (s *Session) BoardFlipped() bool {
	base := false
	if s.mode == ModeComputer && s.human == chess.Black {
		base = true
	}
	if s.autoFlip && s.mode == ModeLocal && s.Turn() == chess.Black {
		base = true
	}
	if s.manualFlip {
		base = !base
	}
	return base
}

// Turn is the side to move.
func (s *Session) Turn() chess.Color { return s.g.Position().Turn() }

// Piece returns the piece on a square.
func (s *Session) Piece(sq chess.Square) chess.Piece {
	return s.g.Position().Board().Piece(sq)
}

// MovesFrom returns legal moves starting on sq.
func (s *Session) MovesFrom(sq chess.Square) []*chess.Move {
	var out []*chess.Move
	for _, m := range s.g.ValidMoves() {
		if m.S1() == sq {
			out = append(out, m)
		}
	}
	return out
}

// FindPremove returns the legal move matching a pre-move, or nil.
func (s *Session) FindPremove(from, to chess.Square, promo chess.PieceType) *chess.Move {
	for _, m := range s.g.ValidMoves() {
		if m.S1() == from && m.S2() == to && m.Promo() == promo {
			return m
		}
	}
	return nil
}

// InCheck reports whether the side to move is in check.
// notnil/chess records this on the position but does not export it; the
// checkmate status is public, and the unexported flag covers plain check.
func (s *Session) InCheck() bool {
	pos := s.g.Position()
	if pos.Status() == chess.Checkmate {
		return true
	}
	v := reflect.ValueOf(pos).Elem().FieldByName("inCheck")
	if v.IsValid() && v.Kind() == reflect.Bool {
		return v.Bool()
	}
	moves := s.g.Moves()
	if len(moves) == 0 {
		return false
	}
	return moves[len(moves)-1].HasTag(chess.Check)
}

// KingSquare returns the square of color's king.
func KingSquare(b *chess.Board, color chess.Color) (chess.Square, bool) {
	for sq, p := range b.SquareMap() {
		if p.Type() == chess.King && p.Color() == color {
			return sq, true
		}
	}
	return chess.NoSquare, false
}

// LastMove returns the squares of the most recent move.
func (s *Session) LastMove() (from, to chess.Square, ok bool) {
	moves := s.g.Moves()
	if len(moves) == 0 {
		return chess.NoSquare, chess.NoSquare, false
	}
	m := moves[len(moves)-1]
	return m.S1(), m.S2(), true
}

// Over reports whether the game has ended.
func (s *Session) Over() bool {
	return s.g.Outcome() != chess.NoOutcome || s.timedOut || s.flagDraw
}

// Unsaved reports whether moves exist that have not been written out.
func (s *Session) Unsaved() bool { return s.unsaved }

// MarkSaved clears the dirty flag.
func (s *Session) MarkSaved() { s.unsaved = false }

// DrawOffer is the color that has offered a draw, or NoColor.
func (s *Session) DrawOffer() chess.Color { return s.drawOffer }

// Outcome and Method proxy the library.
func (s *Session) Outcome() chess.Outcome { return s.g.Outcome() }
func (s *Session) Method() chess.Method   { return s.g.Method() }

// Play applies a legal move, records an undo point, and switches the clock.
func (s *Session) Play(m *chess.Move) (tea.Cmd, error) {
	if s.Over() {
		return nil, fmt.Errorf("game is over")
	}
	mover := s.Turn()
	s.pushUndo()
	if err := s.g.Move(m); err != nil {
		s.undo = s.undo[:len(s.undo)-1]
		return nil, err
	}
	s.drawOffer = chess.NoColor
	s.unsaved = true
	cmd := s.clk.Moved(mover)
	if s.Over() {
		return tea.Batch(cmd, s.clk.Halt()), nil
	}
	return cmd, nil
}

// Undo steps back one ply in a local game, or a full pair against the computer
// when both plies are on the stack. It is refused after the game has ended.
func (s *Session) Undo() (tea.Cmd, error) {
	if s.Over() {
		return nil, fmt.Errorf("undo is disabled after the game ends")
	}
	n := 1
	if s.mode == ModeComputer && len(s.undo) >= 2 {
		n = 2
	}
	if len(s.undo) < n {
		return nil, fmt.Errorf("nothing to undo")
	}
	snap := s.undo[len(s.undo)-n]
	s.undo = s.undo[:len(s.undo)-n]
	s.g = snap.g
	s.unsaved = len(s.undo) > 0
	s.drawOffer = chess.NoColor
	return s.clk.Apply(snap.clock), nil
}

func (s *Session) pushUndo() {
	s.undo = append(s.undo, undoSnap{g: s.g.Clone(), clock: s.clk.Snapshot()})
}

// Resign makes color resign. The game ends 0-1 if White resigns and 1-0 if
// Black resigns. A second call, or a call after the game is already over,
// does nothing.
func (s *Session) Resign(color chess.Color) tea.Cmd {
	if s.Over() || (color != chess.White && color != chess.Black) {
		return nil
	}
	s.g.Resign(color)
	if s.g.Method() != chess.Resignation {
		return nil
	}
	s.resigned = color
	s.g.AddTagPair("Termination", color.Name()+" resigns")
	s.unsaved = true
	s.drawOffer = chess.NoColor
	return s.clk.Halt()
}

// Flag ends the game because color ran out of time.
// If the opponent cannot possibly checkmate, the result is a draw.
func (s *Session) Flag(color chess.Color) tea.Cmd {
	if s.Over() {
		return nil
	}
	s.flagColor = color
	opp := color.Other()
	if !MatingMaterial(s.g.Position().Board(), opp) {
		_ = s.g.Draw(chess.DrawOffer)
		s.flagDraw = true
		s.g.AddTagPair("Termination", "Time forfeit (insufficient mating material)")
	} else {
		s.g.Resign(color)
		s.timedOut = true
		s.g.AddTagPair("Termination", "Time forfeit")
	}
	s.unsaved = true
	return s.clk.Halt()
}

// ClaimableDraw is a threefold or fifty-move claim, if one is available.
func (s *Session) ClaimableDraw() (chess.Method, bool) {
	if s.Over() {
		return chess.NoMethod, false
	}
	for _, m := range s.g.EligibleDraws() {
		if m == chess.ThreefoldRepetition || m == chess.FiftyMoveRule {
			return m, true
		}
	}
	return chess.NoMethod, false
}

// ClaimDraw claims a threefold repetition or fifty-move draw.
func (s *Session) ClaimDraw() error {
	method, ok := s.ClaimableDraw()
	if !ok {
		return fmt.Errorf("no draw to claim")
	}
	if err := s.g.Draw(method); err != nil {
		return err
	}
	s.unsaved = true
	_ = s.clk.Halt()
	return nil
}

// OfferDraw records a draw offer from color. A later move clears it.
func (s *Session) OfferDraw(color chess.Color) { s.drawOffer = color }

// AcceptDraw agrees a draw that was offered.
func (s *Session) AcceptDraw() error {
	if s.drawOffer == chess.NoColor || s.Over() {
		return fmt.Errorf("no draw offer")
	}
	if err := s.g.Draw(chess.DrawOffer); err != nil {
		return err
	}
	s.unsaved = true
	_ = s.clk.Halt()
	return nil
}

// ArmClock starts the side to move. Call it when the game screen appears.
func (s *Session) ArmClock() tea.Cmd {
	if s.Over() {
		return s.clk.Halt()
	}
	return s.clk.InitFor(s.Turn())
}

// StopClock freezes the clock.
func (s *Session) StopClock() tea.Cmd { return s.clk.Halt() }

// PGN renders the game with the headers the spec asks for, plus clock tags
// so a loaded game can resume the times.
func (s *Session) PGN() string {
	g := s.g
	g.AddTagPair("Event", "Termchess")
	g.AddTagPair("Site", "local")
	g.AddTagPair("Date", time.Now().Format("2006.01.02"))
	g.AddTagPair("White", s.whiteName)
	g.AddTagPair("Black", s.blackName)
	g.AddTagPair("Result", string(g.Outcome()))
	g.AddTagPair("TimeControl", s.tc.PGNString())
	if !s.clk.Unlimited() {
		g.AddTagPair("WhiteClock", strconv.FormatInt(s.clk.Remaining(chess.White).Milliseconds(), 10))
		g.AddTagPair("BlackClock", strconv.FormatInt(s.clk.Remaining(chess.Black).Milliseconds(), 10))
	}
	g.AddTagPair("Mode", s.mode.String())
	g.AddTagPair("AutoFlip", strconv.FormatBool(s.autoFlip))
	if s.human == chess.White || s.human == chess.Black {
		g.AddTagPair("HumanColor", s.human.String())
	}
	if s.elo > 0 {
		g.AddTagPair("Elo", strconv.Itoa(s.elo))
		switch s.human {
		case chess.White:
			g.AddTagPair("BlackElo", strconv.Itoa(s.elo))
		case chess.Black:
			g.AddTagPair("WhiteElo", strconv.Itoa(s.elo))
		}
	}
	return g.String()
}

// ResultText is the status line once the game is over.
func (s *Session) ResultText() string {
	if s.flagDraw {
		return fmt.Sprintf("%s flagged — draw (opponent has insufficient mating material)", s.flagColor.Name())
	}
	if s.timedOut {
		return fmt.Sprintf("%s flagged — %s wins", s.flagColor.Name(), s.flagColor.Other().Name())
	}
	if s.resigned == chess.White || s.resigned == chess.Black {
		return s.resigned.Name() + " resigns — " + winnerName(s.g.Outcome()) + " wins"
	}
	switch s.g.Method() {
	case chess.Checkmate:
		winner := chess.White.Name()
		if s.g.Outcome() == chess.BlackWon {
			winner = chess.Black.Name()
		}
		return "Checkmate — " + winner + " wins"
	case chess.Resignation:
		loser := chess.White.Name()
		if s.g.Outcome() == chess.WhiteWon {
			loser = chess.Black.Name()
		}
		return loser + " resigns — " + winnerName(s.g.Outcome()) + " wins"
	case chess.Stalemate:
		return "Stalemate — draw"
	case chess.ThreefoldRepetition:
		return "Draw — threefold repetition"
	case chess.FivefoldRepetition:
		return "Draw — fivefold repetition"
	case chess.FiftyMoveRule:
		return "Draw — fifty-move rule"
	case chess.SeventyFiveMoveRule:
		return "Draw — seventy-five-move rule"
	case chess.InsufficientMaterial:
		return "Draw — insufficient material"
	case chess.DrawOffer:
		return "Draw — by agreement"
	default:
		if s.g.Outcome() == chess.NoOutcome {
			return ""
		}
		return "Game over — " + string(s.g.Outcome())
	}
}

func winnerName(o chess.Outcome) string {
	if o == chess.BlackWon {
		return chess.Black.Name()
	}
	return chess.White.Name()
}

// StatusLine describes the live position.
func (s *Session) StatusLine(thinking, premove bool) string {
	if s.Over() {
		return s.ResultText()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s to move", s.Turn().Name())
	if s.mode == ModeComputer && s.elo > 0 {
		fmt.Fprintf(&b, "  ·  Hypersion (%d)", s.elo)
	}
	if s.InCheck() {
		b.WriteString("  ·  check")
	}
	if thinking {
		b.WriteString("  ·  thinking…")
	}
	if premove {
		b.WriteString("  ·  pre-move")
	}
	if s.drawOffer != chess.NoColor {
		fmt.Fprintf(&b, "  ·  %s offers a draw", s.drawOffer.Name())
	}
	return b.String()
}

// SANHistory renders paired algebraic moves: "1. e4 e5".
func SANHistory(g *chess.Game) string {
	hist := g.MoveHistory()
	if len(hist) == 0 {
		return "No moves yet."
	}
	var b strings.Builder
	for i, h := range hist {
		san := chess.AlgebraicNotation{}.Encode(h.PrePosition, h.Move)
		if i%2 == 0 {
			if i > 0 {
				b.WriteByte('\n')
			}
			fmt.Fprintf(&b, "%2d. %-8s", i/2+1, san)
		} else {
			fmt.Fprintf(&b, "%s", san)
		}
	}
	return b.String()
}

// MoveCount is the number of plies.
func (s *Session) MoveCount() int { return len(s.g.Moves()) }

// MatingMaterial reports whether color has a position from which checkmate
// is possible (king and knight, or king and bishop, cannot mate a lone king).
// Two knights can mate with the opponent's help, so they count.
func MatingMaterial(b *chess.Board, color chess.Color) bool {
	knights, bishops, others := 0, 0, 0
	for _, p := range b.SquareMap() {
		if p.Color() != color || p.Type() == chess.King {
			continue
		}
		switch p.Type() {
		case chess.Knight:
			knights++
		case chess.Bishop:
			bishops++
		default:
			others++
		}
	}
	if others > 0 {
		return true
	}
	if knights+bishops == 0 {
		return false
	}
	if knights == 1 && bishops == 0 {
		return false
	}
	if bishops == 1 && knights == 0 {
		return false
	}
	return true
}

// pieceValue is the standard material weight. Kings are not counted.
func pieceValue(pt chess.PieceType) int {
	switch pt {
	case chess.Queen:
		return 9
	case chess.Rook:
		return 5
	case chess.Bishop, chess.Knight:
		return 3
	case chess.Pawn:
		return 1
	default:
		return 0
	}
}

// Captured lists pieces each side has actually taken, highest value first,
// and the on-board material difference (positive means White is ahead).
//
// The difference is computed from the pieces still on the board, so a
// promotion changes a pawn's value into the new piece's value. It is not
// inferred by comparing the board to the starting counts: that treats a
// promoted pawn as if it had replaced a captured queen, drops the queen from
// the captured list, and jumps the score.
func Captured(g *chess.Game) (whiteHas, blackHas []chess.Piece, diff int) {
	var whiteCaps, blackCaps []chess.Piece
	for _, h := range g.MoveHistory() {
		mv := h.Move
		if mv == nil || h.PrePosition == nil {
			continue
		}
		var taken chess.Piece
		switch {
		case mv.HasTag(chess.EnPassant):
			sq := mv.S2()
			if h.PrePosition.Turn() == chess.White {
				sq -= 8
			} else {
				sq += 8
			}
			if sq < chess.A1 || sq > chess.H8 {
				continue
			}
			taken = h.PrePosition.Board().Piece(sq)
		case mv.HasTag(chess.Capture):
			taken = h.PrePosition.Board().Piece(mv.S2())
		default:
			continue
		}
		if taken == chess.NoPiece {
			continue
		}
		if h.PrePosition.Turn() == chess.White {
			whiteCaps = append(whiteCaps, taken)
		} else {
			blackCaps = append(blackCaps, taken)
		}
	}
	sortPieces(whiteCaps)
	sortPieces(blackCaps)
	b := g.Position().Board()
	return whiteCaps, blackCaps, sideValue(b, chess.White) - sideValue(b, chess.Black)
}

func sideValue(b *chess.Board, color chess.Color) int {
	n := 0
	for _, p := range b.SquareMap() {
		if p.Color() == color {
			n += pieceValue(p.Type())
		}
	}
	return n
}

func sortPieces(ps []chess.Piece) {
	order := func(pt chess.PieceType) int {
		switch pt {
		case chess.Queen:
			return 0
		case chess.Rook:
			return 1
		case chess.Bishop:
			return 2
		case chess.Knight:
			return 3
		default:
			return 4
		}
	}
	for i := 1; i < len(ps); i++ {
		j := i
		for j > 0 && order(ps[j].Type()) < order(ps[j-1].Type()) {
			ps[j], ps[j-1] = ps[j-1], ps[j]
			j--
		}
	}
}
