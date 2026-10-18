package ui

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	"termchess/internal/config"
	"termchess/internal/engine"
	"termchess/internal/game"
	"termchess/internal/gfx"
)

// TestKittyEnvDrawsTextAfterComputerGame is the path a real Kitty launch
// takes: TERM and KITTY_WINDOW_ID are set, there is no --graphics flag, and
// the config does not ask for images. The play view after a vs-computer game
// must still contain the block pieces.
func TestKittyEnvDrawsTextAfterComputerGame(t *testing.T) {
	env := []string{
		"TERM=xterm-kitty",
		"KITTY_WINDOW_ID=1",
		"KITTY_PID=4242",
		"KITTY_PUBLIC_KEY=0123456789abcdef",
		"COLORTERM=truecolor",
	}
	// A missing file loads Default. Also reject a saved auto or kitty value,
	// which used to detect the terminal and replace every square.
	for _, saved := range []string{"", "off", "auto", "kitty"} {
		t.Run("config="+saved, func(t *testing.T) {
			cfg := config.Default()
			cfg.Graphics = saved
			prep, err := Prepare(nil, env, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if prep.Graphics != gfx.Off {
				t.Fatalf("graphics %s with no --graphics flag", prep.Graphics)
			}
			if prep.Profile != colorprofile.TrueColor {
				t.Fatalf("profile %s", prep.Profile)
			}
			a := appFromPrep(t, prep, cfg)
			a = startComputerGame(t, a)
			view := a.View().Content
			assertTextPieces(t, view)
			plain := stripSGR(view)
			if strings.Contains(plain, "kitty") {
				t.Fatalf("header selected a kitty renderer:\n%s", plain)
			}
			if strings.Contains(view, "\x1b_G") || strings.Contains(view, "1337;") || strings.Contains(view, "\x1bPq") {
				t.Fatal("omitted --graphics still emitted an image protocol")
			}
		})
	}
}

func TestExplicitKittyGraphicsStillDrawsText(t *testing.T) {
	env := []string{"TERM=xterm-kitty", "KITTY_WINDOW_ID=1", "KITTY_PID=4242", "COLORTERM=truecolor"}
	if _, err := Prepare([]string{"--graphics", "auto"}, env, config.Default()); err == nil {
		t.Fatal("--graphics auto was accepted")
	}
	prep, err := Prepare([]string{"--graphics", "kitty"}, env, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if prep.Graphics != gfx.Kitty {
		t.Fatalf("graphics %s", prep.Graphics)
	}
	a := appFromPrep(t, prep, config.Default())
	a = startComputerGame(t, a)
	view := a.View().Content
	assertTextPieces(t, view)
	if strings.Contains(view, "\U0010EEEE") {
		t.Fatal("explicit kitty replaced the pieces with placeholders")
	}
}

func TestDebugDumpFrameHasPieces(t *testing.T) {
	env := []string{"TERM=xterm-kitty", "KITTY_WINDOW_ID=1", "KITTY_PID=4242", "KITTY_PUBLIC_KEY=abc", "COLORTERM=truecolor"}
	prep, err := Prepare([]string{"--graphics", "off", "--debug-dump"}, env, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if !prep.DebugDump {
		t.Fatal("debug-dump was not set")
	}
	if prep.Graphics != gfx.Off {
		t.Fatalf("graphics %s", prep.Graphics)
	}
	frame := appFromPrep(t, prep, config.Default()).DebugFrame(140, 64)
	assertTextPieces(t, frame)
}

func appFromPrep(t *testing.T, prep Prepared, cfg config.Config) *App {
	t.Helper()
	dir := t.TempDir()
	cfg.MovePaceMs = 0
	a := New(Options{
		Logger:     log.New(io.Discard),
		Config:     cfg,
		ConfigPath: filepath.Join(dir, "config.json"),
		DataDir:    filepath.Join(dir, "games"),
		Engine:     &engine.Mock{},
		EngineOK:   true,
		ColorMode:  prep.ColorMode,
		Colorless:  prep.Colorless,
		Graphics:   prep.Graphics,
		Blocks:     prep.Blocks,
		Pieces:     prep.Pieces,
		Profile:    prep.Profile,
	})
	a.width = 140
	a.height = 64
	a.dark = true
	return a
}

func startComputerGame(t *testing.T, a *App) *App {
	t.Helper()
	model, cmd := a.openSetup(game.ModeComputer)
	a = pump(model.(*App), cmd)
	a = send(a, "enter") // colour
	a = send(a, "enter") // elo
	a = send(a, "enter") // time control
	if a.screen != screenPlay || a.session == nil {
		t.Fatalf("vs-computer did not start: screen %v", a.screen)
	}
	return a
}

func assertTextPieces(t *testing.T, view string) {
	t.Helper()
	plain := stripSGR(view)
	if !strings.Contains(plain, "Classic wood") || !strings.Contains(plain, "8×4") {
		t.Fatalf("header missing:\n%s", plain)
	}
	if !strings.Contains(plain, "█") {
		t.Fatalf("no block pieces:\n%s", plain)
	}
	if !strings.Contains(view, "38;2;255;255;255") || !strings.Contains(view, "38;2;0;0;0") {
		t.Fatal("piece foreground colours are missing")
	}
	if strings.Contains(view, "\U0010EEEE") {
		t.Fatal("squares were replaced with kitty placeholders")
	}
}
