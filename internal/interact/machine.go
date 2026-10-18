// Package interact is the board state machine.
//
//	idle → piece selected → awaiting promotion → move committed
//
// While the engine thinks the machine sits in engine-thinking. The player may
// set one pre-move (a second selected state). Esc cancels the current layer.
// Enter on an illegal square flashes and keeps the selection.
package interact

import (
	"github.com/notnil/chess"
	"termchess/internal/coord"
	"termchess/internal/game"
)

// Phase is the interaction state.
type Phase int

const (
	PhaseIdle Phase = iota
	PhaseSelected
	PhasePromotion
	PhaseEngine
	PhasePremove
	PhasePremovePromo
	PhaseOver
)

func (p Phase) String() string {
	switch p {
	case PhaseIdle:
		return "idle"
	case PhaseSelected:
		return "selected"
	case PhasePromotion:
		return "promotion"
	case PhaseEngine:
		return "engine"
	case PhasePremove:
		return "premove"
	case PhasePremovePromo:
		return "premove-promotion"
	case PhaseOver:
		return "over"
	default:
		return "?"
	}
}

// Premove is a move the player queued during the engine's search.
type Premove struct {
	From  chess.Square
	To    chess.Square
	Promo chess.PieceType
}

// Dest is a highlighted target square.
type Dest struct {
	Sq      chess.Square
	Capture bool
}

// Result is what Confirm decided. Move is set when a real move should be played.
type Result struct {
	Move       *chess.Move
	Flash      string
	PremoveSet bool
}

// Machine holds the cursor and the selection.
type Machine struct {
	Phase      Phase
	Cursor     chess.Square
	Selected   chess.Square
	HasSel     bool
	Targets    []Dest
	Promo      []chess.PieceType
	PromoIndex int
	Premove    *Premove
	Human      chess.Color
	moves      []*chess.Move
	pendingTo  chess.Square
}

// New starts at cursor with the human color (NoColor means both sides play).
func New(cursor chess.Square, human chess.Color) *Machine {
	return &Machine{Phase: PhaseIdle, Cursor: cursor, Human: human}
}

// MoveCursor steps in a screen direction. flipped is the current orientation.
func (m *Machine) MoveCursor(dir coord.Dir, flipped bool) {
	next, _ := coord.Move(m.Cursor, dir, flipped)
	m.Cursor = next
}

// PromoLeft / PromoRight walk the promotion picker and stop at the ends.
func (m *Machine) PromoLeft() {
	if m.PromoIndex > 0 {
		m.PromoIndex--
	}
}

func (m *Machine) PromoRight() {
	if m.PromoIndex+1 < len(m.Promo) {
		m.PromoIndex++
	}
}

// Cancel handles Esc.
func (m *Machine) Cancel() {
	switch m.Phase {
	case PhasePromotion:
		m.Phase = PhaseSelected
	case PhasePremovePromo:
		m.Phase = PhasePremove
	case PhaseSelected:
		m.clearSel()
		m.Phase = PhaseIdle
	case PhasePremove:
		m.clearSel()
		m.Premove = nil
		m.Phase = PhaseEngine
	case PhaseEngine:
		m.Premove = nil
	}
}

// BeginEngine locks normal moves and clears a selection.
func (m *Machine) BeginEngine() {
	m.clearSel()
	m.Premove = nil
	m.Phase = PhaseEngine
}

// SetOver freezes the board.
func (m *Machine) SetOver() {
	m.clearSel()
	m.Premove = nil
	m.Phase = PhaseOver
}

// SetIdle is used after undo or when it becomes the human's turn.
func (m *Machine) SetIdle() {
	m.clearSel()
	m.Premove = nil
	if m.Phase != PhaseOver {
		m.Phase = PhaseIdle
	}
}

// Thinking reports whether the engine is searching.
func (m *Machine) Thinking() bool {
	switch m.Phase {
	case PhaseEngine, PhasePremove, PhasePremovePromo:
		return true
	default:
		return false
	}
}

// Confirm handles Enter / Space.
func (m *Machine) Confirm(s *game.Session) Result {
	if m.Phase == PhaseOver || s.Over() {
		return Result{}
	}
	switch m.Phase {
	case PhasePromotion:
		return m.confirmPromo(false)
	case PhasePremovePromo:
		return m.confirmPromo(true)
	}

	premove := m.Phase == PhaseEngine || m.Phase == PhasePremove
	selectable := s.Turn()
	if premove {
		selectable = m.Human
	}
	sq := m.Cursor
	piece := s.Piece(sq)

	switch m.Phase {
	case PhaseIdle, PhaseEngine:
		if piece == chess.NoPiece || piece.Color() != selectable {
			return Result{Flash: "Illegal square"}
		}
		m.selectAt(s, sq, premove)
		return Result{}
	case PhaseSelected, PhasePremove:
		if piece != chess.NoPiece && piece.Color() == selectable && (!m.HasSel || sq != m.Selected) {
			m.selectAt(s, sq, premove)
			return Result{}
		}
		if m.HasSel && sq == m.Selected {
			return Result{}
		}
		if premove {
			return m.choosePremove(s, sq)
		}
		mv, needsPromo := m.match(sq)
		if mv == nil && !needsPromo {
			return Result{Flash: "Illegal square"}
		}
		if needsPromo {
			m.Phase = PhasePromotion
			m.pendingTo = sq
			m.Promo = m.promos(sq)
			m.PromoIndex = 0
			return Result{}
		}
		m.clearSel()
		m.Phase = PhaseIdle
		return Result{Move: mv}
	default:
		return Result{}
	}
}

func (m *Machine) selectAt(s *game.Session, sq chess.Square, premove bool) {
	m.Selected = sq
	m.HasSel = true
	m.Premove = nil
	if premove {
		m.Phase = PhasePremove
		m.moves = nil
		m.Targets = nil
		return
	}
	m.Phase = PhaseSelected
	m.moves = s.MovesFrom(sq)
	m.Targets = dests(m.moves)
}

func (m *Machine) choosePremove(s *game.Session, to chess.Square) Result {
	occ := s.Piece(to)
	if occ != chess.NoPiece && occ.Color() == m.Human {
		return Result{Flash: "Illegal square"}
	}
	mover := s.Piece(m.Selected)
	if mover.Type() == chess.Pawn && (to.Rank() == chess.Rank8 || to.Rank() == chess.Rank1) {
		m.Phase = PhasePremovePromo
		m.pendingTo = to
		m.Promo = []chess.PieceType{chess.Queen, chess.Rook, chess.Bishop, chess.Knight}
		m.PromoIndex = 0
		return Result{}
	}
	m.Premove = &Premove{From: m.Selected, To: to, Promo: chess.NoPieceType}
	m.clearSel()
	m.Phase = PhaseEngine
	return Result{PremoveSet: true}
}

func (m *Machine) confirmPromo(premove bool) Result {
	if len(m.Promo) == 0 {
		return Result{Flash: "Illegal square"}
	}
	pt := m.Promo[m.PromoIndex]
	if premove {
		m.Premove = &Premove{From: m.Selected, To: m.pendingTo, Promo: pt}
		m.clearSel()
		m.Phase = PhaseEngine
		return Result{PremoveSet: true}
	}
	for _, mv := range m.moves {
		if mv.S2() == m.pendingTo && mv.Promo() == pt {
			m.clearSel()
			m.Phase = PhaseIdle
			return Result{Move: mv}
		}
	}
	return Result{Flash: "Illegal square"}
}

func (m *Machine) match(to chess.Square) (*chess.Move, bool) {
	var found []*chess.Move
	for _, mv := range m.moves {
		if mv.S2() == to {
			found = append(found, mv)
		}
	}
	if len(found) == 0 {
		return nil, false
	}
	for _, mv := range found {
		if mv.Promo() != chess.NoPieceType {
			return nil, true
		}
	}
	return found[0], false
}

func (m *Machine) promos(to chess.Square) []chess.PieceType {
	seen := map[chess.PieceType]bool{}
	for _, mv := range m.moves {
		if mv.S2() == to && mv.Promo() != chess.NoPieceType {
			seen[mv.Promo()] = true
		}
	}
	order := []chess.PieceType{chess.Queen, chess.Rook, chess.Bishop, chess.Knight}
	var out []chess.PieceType
	for _, pt := range order {
		if seen[pt] {
			out = append(out, pt)
		}
	}
	return out
}

func (m *Machine) clearSel() {
	m.HasSel = false
	m.moves = nil
	m.Targets = nil
	m.Promo = nil
}

func dests(moves []*chess.Move) []Dest {
	seen := map[chess.Square]bool{}
	var out []Dest
	for _, mv := range moves {
		if seen[mv.S2()] {
			continue
		}
		seen[mv.S2()] = true
		out = append(out, Dest{
			Sq:      mv.S2(),
			Capture: mv.HasTag(chess.Capture) || mv.HasTag(chess.EnPassant),
		})
	}
	return out
}

// IsTarget reports whether sq is a legal destination of the current selection.
func (m *Machine) IsTarget(sq chess.Square) (Dest, bool) {
	for _, d := range m.Targets {
		if d.Sq == sq {
			return d, true
		}
	}
	return Dest{}, false
}
