// Package engine talks to a UCI opponent. Stockfish is the real backend;
// tests use a scripted Engine. Ratings and the UCI_Elo mapping live in elo.go.
package engine

import (
	"context"
	"time"

	"github.com/notnil/chess"
)

// Request is one search. Elo is a ladder rating. Budget is the clock cap
// applied on top of the plan's own movetime (0 leaves the plan unchanged).
type Request struct {
	Position *chess.Position
	Elo      int
	Budget   time.Duration
}

// Engine is a UCI opponent. Implementations must be safe for one search at a time.
type Engine interface {
	BestMove(ctx context.Context, req Request) (*chess.Move, error)
	Close() error
}

// ThinkBudget caps the level's preferred movetime by the clock.
// Unlimited games use the level time as-is. Timed games keep a small reserve
// so the engine does not flag itself, and may spend part of the increment.
func ThinkBudget(levelTime, remaining, increment time.Duration, unlimited bool) time.Duration {
	if levelTime <= 0 {
		levelTime = 100 * time.Millisecond
	}
	if unlimited || remaining <= 0 {
		if unlimited {
			return levelTime
		}
		if remaining < 10*time.Millisecond {
			return 10 * time.Millisecond
		}
		return remaining / 2
	}
	const reserve = 80 * time.Millisecond
	avail := remaining - reserve
	if avail < 20*time.Millisecond {
		avail = remaining / 2
	}
	want := levelTime
	cap := remaining/12 + increment/2
	if cap < 40*time.Millisecond {
		cap = 40 * time.Millisecond
	}
	if want > cap {
		want = cap
	}
	if want > avail {
		want = avail
	}
	if want < 10*time.Millisecond {
		want = 10 * time.Millisecond
	}
	return want
}
