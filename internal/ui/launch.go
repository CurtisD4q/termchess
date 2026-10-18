package ui

import (
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/colorprofile"
	"termchess/internal/config"
	"termchess/internal/game"
	"termchess/internal/gfx"
)

// Prepared is everything main derives from flags, the environment, and config
// before it builds the model. Graphics is off unless --graphics was passed.
type Prepared struct {
	Stockfish string
	ColorMode ColorMode
	Profile   colorprofile.Profile
	Colorless bool
	Graphics  gfx.Mode
	Blocks    BlockDetail
	Pieces    PieceStyle
	DebugDump bool
}

// Prepare parses args the same way the termchess binary does. env is the
// process environment. cfg is the loaded config; its graphics value does not
// enable images, and neither does the environment. Only --graphics kitty,
// iterm, or sixel does. --graphics off forces text.
func Prepare(args []string, env []string, cfg config.Config) (Prepared, error) {
	fs := flag.NewFlagSet("termchess", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sf := fs.String("stockfish", "", "path to the Stockfish binary (default: search PATH)")
	colorFlag := fs.String("color", "auto", "color mode: auto, truecolor, 256, 16, or none")
	graphicsFlag := fs.String("graphics", "", "piece images stay off unless this is kitty, iterm, or sixel; off forces text")
	blocksFlag := fs.String("blocks", "auto", "text pieces: auto, quadrant, sextant, or octant")
	piecesFlag := fs.String("pieces", "classic", "text pieces: classic (default) or silhouette")
	debugDump := fs.Bool("debug-dump", false, "print one frame of the start position and exit")
	if err := fs.Parse(args); err != nil {
		return Prepared{}, err
	}

	mode, err := ParseColorMode(*colorFlag)
	if err != nil {
		return Prepared{}, err
	}
	profile := ProfileFor(mode, env)

	graphicsArg := ""
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "graphics" {
			graphicsArg = *graphicsFlag
		}
	})
	gmode, err := gfx.Resolve(graphicsArg, cfg.Graphics, env)
	if err != nil {
		return Prepared{}, err
	}

	blocks, err := ResolveBlocks(*blocksFlag, cfg.Blocks, env)
	if err != nil {
		return Prepared{}, err
	}

	piecesArg := ""
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "pieces" {
			piecesArg = *piecesFlag
		}
	})
	pieceStyle, err := ResolvePieces(piecesArg, cfg.Pieces)
	if err != nil {
		return Prepared{}, err
	}

	return Prepared{
		Stockfish: *sf,
		ColorMode: mode,
		Profile:   profile,
		Colorless: profile <= colorprofile.ASCII,
		Graphics:  gmode,
		Blocks:    blocks,
		Pieces:    pieceStyle,
		DebugDump: *debugDump,
	}, nil
}

// DebugFrame installs the starting position and returns one play-screen frame.
// width and height are the terminal size; a non-positive value keeps the
// model's current size.
func (a *App) DebugFrame(width, height int) string {
	if width > 0 {
		a.width = width
	}
	if height > 0 {
		a.height = height
	}
	a.dark = true
	s, err := game.New(game.Options{Mode: game.ModeLocal, TC: game.PresetByID("none")})
	if err != nil {
		return err.Error()
	}
	model, _ := a.useSession(s)
	app, ok := model.(*App)
	if !ok {
		return fmt.Sprintf("internal error: %T", model)
	}
	return app.View().Content
}
