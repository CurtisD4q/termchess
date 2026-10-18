package engine

import (
	"testing"
	"time"
)

func TestThinkBudgetRespectsClock(t *testing.T) {
	level := 10 * time.Second
	// The level wants 10s, but the clock only allows about remaining/12.
	got := ThinkBudget(level, 30*time.Second, 0, false)
	if got > 30*time.Second/12 {
		t.Fatalf("budget %s ignored remaining time", got)
	}

	// Increment lets the engine think a little longer.
	withInc := ThinkBudget(level, 30*time.Second, 10*time.Second, false)
	if withInc <= got {
		t.Fatalf("increment did not extend budget: %s vs %s", withInc, got)
	}

	// Never plan to use the entire remaining time.
	tight := ThinkBudget(level, 200*time.Millisecond, 0, false)
	if tight >= 200*time.Millisecond {
		t.Fatalf("budget %s ate the whole clock", tight)
	}

	if unlimited := ThinkBudget(level, 0, 0, true); unlimited != level {
		t.Fatalf("unlimited = %s, want %s", unlimited, level)
	}
}
