package ui

import (
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestParseColorMode(t *testing.T) {
	for _, in := range []string{"", "auto", "AUTO"} {
		m, err := ParseColorMode(in)
		if err != nil || m != ColorAuto {
			t.Fatalf("%q -> %v %v", in, m, err)
		}
	}
	m, err := ParseColorMode("256")
	if err != nil || m != Color256 {
		t.Fatalf("256 -> %v %v", m, err)
	}
	if _, err := ParseColorMode("neon"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestProfileForTerminals(t *testing.T) {
	xfce := []string{"TERM=xterm-256color"}
	if p := ProfileFor(ColorAuto, xfce); p != colorprofile.ANSI256 {
		t.Fatalf("xfce auto = %s, want ANSI256", p)
	}
	if p := ProfileFor(Color256, []string{"TERM=dumb"}); p != colorprofile.ANSI256 {
		t.Fatalf("forced 256 on dumb = %s", p)
	}
	if p := ProfileFor(ColorTrue, xfce); p != colorprofile.TrueColor {
		t.Fatalf("forced truecolor = %s", p)
	}
	if p := ProfileFor(Color16, xfce); p != colorprofile.ANSI {
		t.Fatalf("forced 16 = %s", p)
	}
	if p := ProfileFor(ColorNone, xfce); p != colorprofile.ASCII {
		t.Fatalf("forced none = %s", p)
	}
	trueTerm := []string{"TERM=xterm-256color", "COLORTERM=truecolor"}
	if p := ProfileFor(ColorAuto, trueTerm); p != colorprofile.TrueColor {
		t.Fatalf("COLORTERM auto = %s", p)
	}
	if p := ProfileFor(ColorAuto, []string{"TERM=dumb"}); p != colorprofile.ASCII {
		t.Fatalf("dumb auto = %s", p)
	}
	if p := ProfileFor(ColorAuto, []string{"TERM=xterm-256color", "NO_COLOR=1"}); p != colorprofile.ASCII {
		t.Fatalf("NO_COLOR auto = %s", p)
	}
}
