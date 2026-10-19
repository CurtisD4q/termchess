package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/x/term"
	"termchess/internal/config"
	"termchess/internal/engine"
	"termchess/internal/gfx"
	"termchess/internal/ui"
)

func main() {
	logger := openLog()
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		logger.Error("config", "err", err)
		cfg = config.Default()
	}
	prep, err := ui.Prepare(os.Args[1:], os.Environ(), cfg)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		os.Exit(2)
	}
	logger.Info("color", "mode", prep.ColorMode.String(), "profile", prep.Profile.String())
	logger.Info("graphics", "mode", prep.Graphics.String())
	logger.Info("blocks", "detail", prep.Blocks.String())
	logger.Info("pieces", "style", prep.Pieces.String())

	path, err := engine.Find(prep.Engine)
	if err != nil {
		logger.Info("hypersion unavailable", "err", err)
		path = ""
	} else {
		logger.Info("hypersion", "path", path)
	}

	app := ui.New(ui.Options{
		EnginePath: path,
		EngineOK:   path != "",
		Config:     cfg,
		ConfigPath: config.ConfigPath(),
		DataDir:    config.GamesDir(),
		Logger:     logger,
		ColorMode:  prep.ColorMode,
		Colorless:  prep.Colorless,
		Graphics:   prep.Graphics,
		Blocks:     prep.Blocks,
		Pieces:     prep.Pieces,
		Profile:    prep.Profile,
	})
	if prep.DebugDump {
		w, h := 140, 64
		if tw, th, err := term.GetSize(os.Stdout.Fd()); err == nil && tw >= 20 && th >= 10 {
			w, h = tw, th
		}
		fmt.Fprint(os.Stdout, app.DebugFrame(w, h))
		app.Shutdown()
		return
	}
	p := tea.NewProgram(app, tea.WithColorProfile(prep.Profile), tea.WithEnvironment(os.Environ()))
	_, runErr := p.Run()
	fmt.Fprint(os.Stdout, gfx.Cleanup(prep.Graphics))
	app.Shutdown()
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		os.Exit(1)
	}
}

func openLog() *log.Logger {
	path := config.LogPath()
	if err := os.MkdirAll(config.DataDir(), 0o755); err != nil {
		return log.New(io.Discard)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return log.New(io.Discard)
	}
	return log.NewWithOptions(f, log.Options{
		ReportTimestamp: true,
		Level:           log.DebugLevel,
	})
}
