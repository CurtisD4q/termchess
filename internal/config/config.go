// Package config persists the theme and last game settings.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"termchess/internal/engine"
)

// Config is the on-disk file.
type Config struct {
	Theme string `json:"theme"`
	// Graphics is recorded for compatibility. Images stay off unless the
	// process is started with an explicit --graphics flag.
	Graphics string `json:"graphics"`
	// Blocks is auto, quadrant, sextant, or octant. auto keeps xfce on quadrants.
	Blocks string `json:"blocks"`
	// Pieces is classic, silhouette, or set1..set5. classic is the default block set.
	Pieces string `json:"pieces"`
	// MovePaceMs is the minimum gap between showing two moves, in milliseconds.
	// Zero applies a move the moment it is ready. The default is 400.
	MovePaceMs int  `json:"movePaceMs"`
	Last       Last `json:"last"`
}

// Last remembers the previous setup form.
type Last struct {
	Mode            string `json:"mode"`
	AutoFlip        bool   `json:"autoFlip"`
	HumanColor      string `json:"humanColor"`
	Elo             int    `json:"elo"`
	TimeControl     string `json:"timeControl"`
	CustomMinutes   int    `json:"customMinutes"`
	CustomIncrement int    `json:"customIncrement"`
}

// Default is the first-run configuration.
func Default() Config {
	return Config{
		Theme:      "wood",
		Graphics:   "off",
		Blocks:     "auto",
		Pieces:     "classic",
		MovePaceMs: 400,
		Last: Last{
			Mode:            "local",
			HumanColor:      "white",
			Elo:             engine.DefaultElo,
			TimeControl:     "5+0",
			CustomMinutes:   10,
			CustomIncrement: 0,
		},
	}
}

// Load reads path. A missing file returns Default.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Default(), err
	}
	cfg := Default()
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Default(), err
	}
	if cfg.Theme == "" {
		cfg.Theme = "wood"
	}
	if cfg.Graphics == "" {
		cfg.Graphics = "off"
	}
	if cfg.Blocks == "" {
		cfg.Blocks = "auto"
	}
	if cfg.Pieces == "" {
		cfg.Pieces = "classic"
	}
	if cfg.MovePaceMs < 0 {
		cfg.MovePaceMs = 400
	}
	if cfg.MovePaceMs > 5000 {
		cfg.MovePaceMs = 5000
	}
	if cfg.Last.Elo == 0 {
		cfg.Last.Elo = engine.DefaultElo
	} else {
		cfg.Last.Elo = engine.Nearest(cfg.Last.Elo)
	}
	return cfg, nil
}

// Save writes path, creating parent directories.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// ConfigPath is the user config file.
func ConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "termchess", "config.json")
}

// DataDir is the OS data directory (.../termchess).
func DataDir() string {
	if runtime.GOOS == "windows" {
		if x := os.Getenv("LOCALAPPDATA"); x != "" {
			return filepath.Join(x, "termchess")
		}
	}
	if runtime.GOOS == "darwin" {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, "Library", "Application Support", "termchess")
		}
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "termchess")
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".local", "share", "termchess")
	}
	return "termchess"
}

// GamesDir holds saved PGN files.
func GamesDir() string { return filepath.Join(DataDir(), "games") }

// LogPath is the debug log. The TUI never writes this to stdout.
func LogPath() string { return filepath.Join(DataDir(), "termchess.log") }
