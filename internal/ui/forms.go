package ui

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"charm.land/huh/v2"
	"github.com/notnil/chess"
	"termchess/internal/engine"
	"termchess/internal/game"
	"termchess/internal/store"
)

func newForm(groups ...*huh.Group) *huh.Form {
	// TERM=dumb makes huh switch to its line-oriented accessible mode, which
	// fights a full-screen TUI. Keep the visual form.
	return huh.NewForm(groups...).
		WithAccessible(false).
		WithWidth(56).
		WithShowHelp(true)
}

func (a *App) menuForm() *huh.Form {
	desc := "Arrow keys or hjkl move the cursor. Enter selects. No typed moves."
	if !a.engineOK {
		desc += "\n\nHypersion was not found. Two-player and saved games still work. Build Hypersion or pass --engine to play the computer."
	}
	a.choice = "local"
	return newForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Termchess").
			Description(desc).
			Options(
				huh.NewOption("Local two-player", "local"),
				huh.NewOption("Play vs computer", "computer"),
				huh.NewOption("Load a saved game", "load"),
			).
			Value(&a.choice),
	))
}

func (a *App) setupForm(mode game.Mode) *huh.Form {
	a.tcID = a.cfg.Last.TimeControl
	if a.tcID == "" {
		a.tcID = "5+0"
	}
	a.minutes = strconv.Itoa(a.cfg.Last.CustomMinutes)
	a.incSec = strconv.Itoa(a.cfg.Last.CustomIncrement)
	a.autoFlipChoice = "no"
	if a.cfg.Last.AutoFlip {
		a.autoFlipChoice = "yes"
	}
	a.colorChoice = a.cfg.Last.HumanColor
	if a.colorChoice == "" {
		a.colorChoice = "white"
	}
	a.eloChoice = strconv.Itoa(engine.Nearest(a.cfg.Last.Elo))
	if a.cfg.Last.Elo == 0 {
		a.eloChoice = strconv.Itoa(engine.DefaultElo)
	}

	var tcOpts []huh.Option[string]
	for _, p := range game.Presets() {
		tcOpts = append(tcOpts, huh.NewOption(p.Name, p.ID))
	}
	tc := huh.NewSelect[string]().Title("Time control").Options(tcOpts...).Value(&a.tcID)
	// Minutes and increment apply only to Custom. They live in their own group
	// so a preset skips them and Enter on the time control finishes the form.
	mins := huh.NewInput().Title("Custom minutes").Description("Minutes on each clock.").Value(&a.minutes).CharLimit(3).Placeholder("10")
	inc := huh.NewInput().Title("Custom increment (seconds)").Value(&a.incSec).CharLimit(3).Placeholder("0")
	custom := huh.NewGroup(mins, inc).WithHideFunc(func() bool {
		return a.tcID != "custom"
	})

	if mode == game.ModeLocal {
		flip := huh.NewSelect[string]().
			Title("Flip the board each turn").
			Options(huh.NewOption("Yes", "yes"), huh.NewOption("No", "no")).
			Value(&a.autoFlipChoice)
		return newForm(huh.NewGroup(flip, tc), custom)
	}

	var eloOpts []huh.Option[string]
	for _, e := range engine.Elos() {
		label := strconv.Itoa(e)
		eloOpts = append(eloOpts, huh.NewOption(label, label))
	}
	color := huh.NewSelect[string]().
		Title("Your colour").
		Options(
			huh.NewOption("White", "white"),
			huh.NewOption("Black", "black"),
			huh.NewOption("Random", "random"),
		).
		Value(&a.colorChoice)
	// Fifty-two ratings. "/" opens Huh's filter; type 1650 to jump there.
	// Arrows and j/k step one rating. Ctrl+U / Ctrl+D page.
	elo := huh.NewSelect[string]().
		Title("Engine Elo").
		Description("52 steps from 500 to 2500.  / filters, ↑↓ steps, ctrl+u/d pages.").
		Options(eloOpts...).
		Height(10).
		Value(&a.eloChoice)
	return newForm(huh.NewGroup(color, elo, tc), custom)
}

func (a *App) saveForm() *huh.Form {
	if strings.TrimSpace(a.filename) == "" {
		a.filename = fmt.Sprintf("termchess-%s.pgn", time.Now().Format("20060102-150405"))
	}
	return newForm(huh.NewGroup(
		huh.NewInput().
			Title("Save game").
			Description("Written to the termchess games folder.").
			Value(&a.filename).
			CharLimit(80).
			Validate(func(s string) error {
				_, err := store.SafeName(s)
				return err
			}),
	)).WithShowHelp(false)
}

func (a *App) loadForm(entries []store.Entry) *huh.Form {
	opts := []huh.Option[string]{huh.NewOption("Back", "back")}
	for _, e := range entries {
		opts = append(opts, huh.NewOption(e.Title+"  ·  "+e.Name, e.Name))
	}
	a.loadChoice = "back"
	if len(entries) > 0 {
		a.loadChoice = entries[0].Name
	}
	title := "Saved games"
	if len(entries) == 0 {
		title = "No saved games"
	}
	return newForm(huh.NewGroup(
		huh.NewSelect[string]().Title(title).Options(opts...).Value(&a.loadChoice).Height(10),
	))
}

func (a *App) confirmForm(title string) *huh.Form {
	a.confirmed = false
	return newForm(huh.NewGroup(
		huh.NewConfirm().
			Title(title).
			Affirmative("Yes").
			Negative("No").
			Value(&a.confirmed),
	)).WithShowHelp(false)
}

func (a *App) timeControl() game.TimeControl {
	if a.tcID == "custom" {
		mins, _ := strconv.Atoi(strings.TrimSpace(a.minutes))
		inc, _ := strconv.Atoi(strings.TrimSpace(a.incSec))
		return game.Custom(mins, inc)
	}
	return game.PresetByID(a.tcID)
}

func (a *App) humanColor() chess.Color {
	switch a.colorChoice {
	case "black":
		return chess.Black
	case "random":
		if rand.IntN(2) == 1 {
			return chess.Black
		}
		return chess.White
	default:
		return chess.White
	}
}
