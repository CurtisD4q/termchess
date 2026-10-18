package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/colorprofile"
)

// ColorMode is the --color flag.
type ColorMode int

const (
	ColorAuto ColorMode = iota
	ColorTrue
	Color256
	Color16
	ColorNone
)

// ParseColorMode accepts auto, truecolor, 256, 16, and none.
func ParseColorMode(s string) (ColorMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return ColorAuto, nil
	case "truecolor", "true", "24bit":
		return ColorTrue, nil
	case "256", "ansi256":
		return Color256, nil
	case "16", "ansi":
		return Color16, nil
	case "none", "mono", "off":
		return ColorNone, nil
	default:
		return ColorAuto, fmt.Errorf("unknown --color %q (want auto, truecolor, 256, 16, or none)", s)
	}
}

func (m ColorMode) String() string {
	switch m {
	case ColorTrue:
		return "truecolor"
	case Color256:
		return "256"
	case Color16:
		return "16"
	case ColorNone:
		return "none"
	default:
		return "auto"
	}
}

// ProfileFor picks the palette the renderer should emit.
//
// auto never falls through to a colorless profile when TERM names a color
// terminal. xfce4-terminal reports TERM=xterm-256color; treating that as
// no-color strips every square background and the cursor. True color is used
// only when COLORTERM says the terminal can draw it, or when the user passes
// --color truecolor. --color none is the monochrome fallback (cursor brackets).
func ProfileFor(mode ColorMode, env []string) colorprofile.Profile {
	switch mode {
	case ColorTrue:
		return colorprofile.TrueColor
	case Color256:
		return colorprofile.ANSI256
	case Color16:
		return colorprofile.ANSI
	case ColorNone:
		return colorprofile.ASCII
	default:
		return autoProfile(env)
	}
}

func autoProfile(env []string) colorprofile.Profile {
	term := envGet(env, "TERM")
	ct := strings.ToLower(envGet(env, "COLORTERM"))
	if envGet(env, "NO_COLOR") != "" {
		return colorprofile.ASCII
	}
	if ct == "truecolor" || ct == "24bit" || strings.HasSuffix(term, "direct") {
		return colorprofile.TrueColor
	}
	if strings.Contains(term, "256color") || strings.Contains(term, "256colour") {
		return colorprofile.ANSI256
	}
	if term == "" || term == "dumb" {
		return colorprofile.ASCII
	}
	p := colorprofile.Env(env)
	if p <= colorprofile.ASCII {
		// A named color terminal that the library still called colorless.
		return colorprofile.ANSI256
	}
	if p < colorprofile.ANSI256 && strings.Contains(term, "color") {
		return colorprofile.ANSI256
	}
	return p
}

func envGet(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return env[i][len(prefix):]
		}
	}
	return ""
}
