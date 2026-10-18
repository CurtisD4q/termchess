package clock

import (
	tea "charm.land/bubbletea/v2"
)

// applyMsg runs start/stop commands produced by the clock.
// Tick commands are not produced here; tests inject timer.TickMsg directly.
func applyCmd(c *Clock, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if msg == nil {
		return
	}
	switch m := msg.(type) {
	case tea.BatchMsg:
		for _, next := range m {
			applyCmd(c, next)
		}
	default:
		c.Update(msg)
	}
}
