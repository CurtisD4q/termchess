package engine

import (
	"context"
	"fmt"

	"github.com/notnil/chess"
)

// Mock is a scripted engine. Moves are UCI strings ("e7e5", "e7e8q").
type Mock struct {
	Moves []string
	Err   error
	Calls int
	Last  Request
	// Reported is returned by Bounds. The zero value means "no UCI_Elo".
	Reported Bounds
}

// BestMove pops the next scripted move and decodes it in the given position.
func (m *Mock) BestMove(_ context.Context, req Request) (*chess.Move, error) {
	m.Calls++
	m.Last = req
	if m.Err != nil {
		return nil, m.Err
	}
	if len(m.Moves) == 0 {
		return nil, fmt.Errorf("mock engine has no move")
	}
	uci := m.Moves[0]
	m.Moves = m.Moves[1:]
	mv, err := (chess.UCINotation{}).Decode(req.Position, uci)
	if err != nil {
		return nil, err
	}
	return mv, nil
}

// Bounds returns the scripted UCI_Elo range.
func (m *Mock) Bounds() (Bounds, error) { return m.Reported, nil }

// Close implements Engine.
func (m *Mock) Close() error { return nil }
