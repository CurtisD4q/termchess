package ui

import "testing"

func TestDetectBlocksLeavesXfceOnQuadrants(t *testing.T) {
	if got := DetectBlocks([]string{"TERM=xterm-256color"}); got != BlockQuadrant {
		t.Fatalf("xfce %s", got)
	}
	if got := DetectBlocks([]string{"TERM=xterm-kitty"}); got != BlockQuadrant {
		t.Fatalf("kitty %s", got)
	}
	if got := DetectBlocks([]string{"KITTY_WINDOW_ID=1", "TERM=xterm-256color"}); got != BlockQuadrant {
		t.Fatalf("kitty window %s", got)
	}
	got, err := ResolveBlocks("auto", "octant", nil)
	if err != nil || got != BlockOctant {
		t.Fatalf("config %s %v", got, err)
	}
	got, err = ResolveBlocks("quadrant", "sextant", []string{"TERM=xterm-kitty"})
	if err != nil || got != BlockQuadrant {
		t.Fatalf("flag %s %v", got, err)
	}
}
