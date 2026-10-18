// Package clock is a chess clock built on the Bubbles timer.
//
// Each side has a timer.Model. The Bubble Tea program drives the active timer
// by sending timer.TickMsg from a tea.Tick loop; this package does not schedule
// its own ticks, so the UI cannot run the clock twice.
package clock

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"
	"github.com/notnil/chess"
)

// Interval is how often the UI should tick the active clock.
const Interval = 100 * time.Millisecond

// Snapshot is a restorable clock reading, used by undo and saved games.
type Snapshot struct {
	White   time.Duration
	Black   time.Duration
	Active  chess.Color
	Running bool
}

// Clock tracks both players. A zero initial time means the game is untimed.
type Clock struct {
	white     timer.Model
	black     timer.Model
	inc       time.Duration
	unlimited bool
	active    chess.Color
	flagged   chess.Color
	stopped   bool
}

// New returns a clock. initial <= 0 selects an untimed game.
// Both timers are created running; call InitFor before the first tick so the
// side that is not to move is stopped.
func New(initial, increment time.Duration) *Clock {
	if initial <= 0 {
		return &Clock{unlimited: true, active: chess.White}
	}
	return &Clock{
		white:  timer.New(initial, timer.WithInterval(Interval)),
		black:  timer.New(initial, timer.WithInterval(Interval)),
		inc:    increment,
		active: chess.White,
	}
}

// Unlimited reports whether this game has no clock.
func (c *Clock) Unlimited() bool { return c.unlimited }

// Increment is the per-move bonus.
func (c *Clock) Increment() time.Duration { return c.inc }

// Active is the side whose clock is running.
func (c *Clock) Active() chess.Color { return c.active }

// Flagged is the side that has run out of time, or NoColor.
func (c *Clock) Flagged() chess.Color { return c.flagged }

// Halted reports whether the clock has been stopped for good (game over).
func (c *Clock) Halted() bool { return c.stopped }

// ActiveID is the Bubbles timer id of the side to move.
func (c *Clock) ActiveID() int {
	if c.active == chess.Black {
		return c.black.ID()
	}
	return c.white.ID()
}

// Remaining returns the time left for color. Untimed clocks report 0.
func (c *Clock) Remaining(color chess.Color) time.Duration {
	if c.unlimited {
		return 0
	}
	if color == chess.Black {
		if c.black.Timeout < 0 {
			return 0
		}
		return c.black.Timeout
	}
	if c.white.Timeout < 0 {
		return 0
	}
	return c.white.Timeout
}

// View formats one side for the board. The Bubbles timer's own View is the
// source of the duration; we reformat it as a chess clock.
func (c *Clock) View(color chess.Color) string {
	if c.unlimited {
		return "∞"
	}
	raw := c.white.View()
	if color == chess.Black {
		raw = c.black.View()
	}
	_ = raw
	return Format(c.Remaining(color))
}

// InitFor starts the side to move and stops the other side.
// The returned command must be handed to Bubble Tea; it only carries
// start/stop messages. Ticks come from the UI's tea.Tick loop.
func (c *Clock) InitFor(side chess.Color) tea.Cmd {
	c.active = side
	c.stopped = false
	c.flagged = chess.NoColor
	if c.unlimited {
		return nil
	}
	if side == chess.Black {
		return tea.Batch(c.black.Start(), c.white.Stop())
	}
	return tea.Batch(c.white.Start(), c.black.Stop())
}

// Halt freezes both clocks.
func (c *Clock) Halt() tea.Cmd {
	c.stopped = true
	if c.unlimited {
		return nil
	}
	return tea.Batch(c.white.Stop(), c.black.Stop())
}

// Update applies a Bubbles timer message. Follow-up tick commands from the
// timer are discarded; the UI is the only ticker.
func (c *Clock) Update(msg tea.Msg) {
	if c.unlimited || c == nil {
		return
	}
	c.white, _ = c.white.Update(msg)
	c.black, _ = c.black.Update(msg)
	if c.stopped || c.flagged != chess.NoColor {
		return
	}
	switch c.active {
	case chess.White:
		if c.white.Timedout() {
			c.flagged = chess.White
		}
	case chess.Black:
		if c.black.Timedout() {
			c.flagged = chess.Black
		}
	}
}

// Moved adds the increment to the player who just moved and starts the
// opponent's clock. The increment is applied even when it is the last move
// of the game; callers that already know the game is over should Halt instead.
func (c *Clock) Moved(mover chess.Color) tea.Cmd {
	if c.unlimited {
		c.active = mover.Other()
		return nil
	}
	if c.stopped {
		return nil
	}
	if mover == chess.White {
		c.white.Timeout += c.inc
		c.active = chess.Black
		return tea.Batch(c.white.Stop(), c.black.Start())
	}
	c.black.Timeout += c.inc
	c.active = chess.White
	return tea.Batch(c.black.Stop(), c.white.Start())
}

// Snapshot copies the current readings.
func (c *Clock) Snapshot() Snapshot {
	return Snapshot{
		White:   c.Remaining(chess.White),
		Black:   c.Remaining(chess.Black),
		Active:  c.active,
		Running: !c.unlimited && !c.stopped && c.flagged == chess.NoColor,
	}
}

// Apply restores a snapshot, replacing whatever the timers currently show.
func (c *Clock) Apply(s Snapshot) tea.Cmd {
	if c.unlimited {
		c.active = s.Active
		c.stopped = !s.Running
		return nil
	}
	c.white.Timeout = s.White
	c.black.Timeout = s.Black
	c.flagged = chess.NoColor
	c.stopped = !s.Running
	c.active = s.Active
	if !s.Running {
		return c.Halt()
	}
	return c.InitFor(s.Active)
}

// Format renders a duration as m:ss, or m:ss.t under ten seconds.
func Format(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	ms := d.Milliseconds()
	min := ms / 60000
	sec := (ms % 60000) / 1000
	tenths := (ms % 1000) / 100
	if d < 10*time.Second {
		return fmt.Sprintf("%d:%02d.%d", min, sec, tenths)
	}
	return fmt.Sprintf("%d:%02d", min, sec)
}
