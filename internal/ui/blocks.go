package ui

import (
	"fmt"
	"strings"
)

// BlockDetail is how a silhouette square is drawn when images are off.
// The zero value is quadrants, which every chess-capable terminal font has.
type BlockDetail int

const (
	BlockQuadrant BlockDetail = iota
	BlockSextant
	BlockOctant
)

func (d BlockDetail) String() string {
	switch d {
	case BlockSextant:
		return "sextant"
	case BlockOctant:
		return "octant"
	default:
		return "quadrant"
	}
}

func (d BlockDetail) subs() (wide, tall int) {
	switch d {
	case BlockSextant:
		return 2, 3
	case BlockOctant:
		return 2, 4
	default:
		return 2, 2
	}
}

// ParseBlocks reads a flag or config value. auto is reported by the bool.
func ParseBlocks(s string) (BlockDetail, bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return BlockQuadrant, true, nil
	case "quadrant", "quad":
		return BlockQuadrant, false, nil
	case "sextant":
		return BlockSextant, false, nil
	case "octant":
		return BlockOctant, false, nil
	default:
		return BlockQuadrant, false, fmt.Errorf("unknown --blocks %q (want auto, quadrant, sextant, or octant)", s)
	}
}

// ResolveBlocks applies the flag, then the config value, then a terminal guess.
func ResolveBlocks(flag, cfg string, env []string) (BlockDetail, error) {
	mode, auto, err := ParseBlocks(flag)
	if err != nil {
		return BlockQuadrant, err
	}
	if !auto {
		return mode, nil
	}
	if strings.TrimSpace(cfg) != "" && !strings.EqualFold(strings.TrimSpace(cfg), "auto") {
		mode, _, err = ParseBlocks(cfg)
		return mode, err
	}
	return DetectBlocks(env), nil
}

// DetectBlocks leaves a plain xterm and Kitty on quadrants. xfce4-terminal
// reports xterm-256color, and Kitty's usual DejaVu font has no U+1FB00
// sextants: those cells are blank, so the silhouette disappears.
// WezTerm, Ghostty, and foot are given sextants. The painter still folds
// those masks onto quadrant blocks, so a missing sextant glyph cannot
// erase the piece.
func DetectBlocks(env []string) BlockDetail {
	term := strings.ToLower(envGet(env, "TERM"))
	prog := strings.ToLower(envGet(env, "TERM_PROGRAM"))
	switch {
	case envGet(env, "KITTY_WINDOW_ID") != "" || strings.Contains(term, "kitty"):
		return BlockQuadrant
	case strings.Contains(prog, "wezterm") || term == "wezterm":
		return BlockSextant
	case strings.Contains(prog, "ghostty") || envGet(env, "GHOSTTY_RESOURCES_DIR") != "":
		return BlockSextant
	case term == "foot" || strings.HasPrefix(term, "foot-"):
		return BlockSextant
	default:
		return BlockQuadrant
	}
}
