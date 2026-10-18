package clock

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/timer"
	"github.com/notnil/chess"
)

func TestIncrement(t *testing.T) {
	c := New(time.Second, 500*time.Millisecond)
	applyCmd(c, c.InitFor(chess.White))

	for i := 0; i < 3; i++ {
		c.Update(timer.TickMsg{ID: c.white.ID()})
	}
	if got := c.Remaining(chess.White); got != 700*time.Millisecond {
		t.Fatalf("white remaining = %s, want 700ms", got)
	}
	applyCmd(c, c.Moved(chess.White))
	if got := c.Remaining(chess.White); got != 1200*time.Millisecond {
		t.Fatalf("white after increment = %s, want 1.2s", got)
	}
	if c.Active() != chess.Black {
		t.Fatalf("active = %s, want black", c.Active())
	}
	// A tick addressed to white must not move white's clock once black is active.
	c.Update(timer.TickMsg{ID: c.white.ID()})
	if got := c.Remaining(chess.White); got != 1200*time.Millisecond {
		t.Fatalf("white changed while inactive: %s", got)
	}
	c.Update(timer.TickMsg{ID: c.black.ID()})
	if got := c.Remaining(chess.Black); got != 900*time.Millisecond {
		t.Fatalf("black remaining = %s, want 900ms", got)
	}
}

func TestFlagFall(t *testing.T) {
	c := New(500*time.Millisecond, 0)
	applyCmd(c, c.InitFor(chess.White))
	for i := 0; i < 6; i++ {
		c.Update(timer.TickMsg{ID: c.ActiveID()})
		if c.Flagged() != chess.NoColor {
			break
		}
	}
	if c.Flagged() != chess.White {
		t.Fatalf("flagged = %s, want white (remaining %s)", c.Flagged(), c.Remaining(chess.White))
	}
	c.Update(timer.TickMsg{ID: c.black.ID()})
	if c.Flagged() != chess.White {
		t.Fatalf("flagged changed to %s", c.Flagged())
	}
}

func TestFormat(t *testing.T) {
	if got := Format(5 * time.Minute); got != "5:00" {
		t.Fatalf("Format(5m) = %s", got)
	}
	if got := Format(1500 * time.Millisecond); got != "0:01.5" {
		t.Fatalf("Format(1.5s) = %s", got)
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	c := New(time.Minute, time.Second)
	applyCmd(c, c.InitFor(chess.Black))
	c.Update(timer.TickMsg{ID: c.black.ID()})
	snap := c.Snapshot()
	applyCmd(c, c.Moved(chess.Black))
	applyCmd(c, c.Apply(snap))
	if c.Active() != chess.Black {
		t.Fatalf("restored active = %s", c.Active())
	}
	if c.Remaining(chess.Black) != snap.Black || c.Remaining(chess.White) != snap.White {
		t.Fatalf("white %s black %s, snap %+v", c.Remaining(chess.White), c.Remaining(chess.Black), snap)
	}
}

func TestTimerViewMatchesRemaining(t *testing.T) {
	c := New(90*time.Second, 0)
	// Bubbles formats the timeout; our chess format is what the board shows.
	if c.View(chess.White) != "1:30" {
		t.Fatalf("view = %s, raw timer %s", c.View(chess.White), c.white.View())
	}
}
