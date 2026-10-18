package config

import (
	"os"
	"path/filepath"
	"testing"

	"termchess/internal/engine"
)

func TestEloPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	cfg.Theme = "blue"
	cfg.Last.Elo = 1650
	cfg.Last.Mode = "computer"
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Last.Elo != 1650 {
		t.Fatalf("elo = %d", got.Last.Elo)
	}
	if got.Theme != "blue" || got.Last.Mode != "computer" {
		t.Fatalf("round trip %+v", got)
	}
}

func TestEloSnapsOntoLadder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	cfg.Last.Elo = 1660
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Last.Elo != engine.Nearest(1660) {
		t.Fatalf("elo = %d, want %d", got.Last.Elo, engine.Nearest(1660))
	}
}

func TestMovePaceDefaultsAndBounds(t *testing.T) {
	if Default().MovePaceMs != 400 {
		t.Fatalf("default pace %d", Default().MovePaceMs)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{\"theme\":\"wood\",\"movePaceMs\":0}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.MovePaceMs != 0 {
		t.Fatalf("explicit zero became %d", got.MovePaceMs)
	}
	if err := os.WriteFile(path, []byte("{\"movePaceMs\":-5}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = Load(path)
	if err != nil || got.MovePaceMs != 400 {
		t.Fatalf("negative pace %d err %v", got.MovePaceMs, err)
	}
	if err := os.WriteFile(path, []byte("{\"movePaceMs\":9000}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = Load(path)
	if err != nil || got.MovePaceMs != 5000 {
		t.Fatalf("capped pace %d err %v", got.MovePaceMs, err)
	}
}

func TestMissingEloUsesDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{\"theme\":\"green\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Last.Elo != engine.DefaultElo {
		t.Fatalf("elo = %d, want %d", got.Last.Elo, engine.DefaultElo)
	}
	if got.Theme != "green" {
		t.Fatalf("theme %s", got.Theme)
	}
	if got.Graphics != "off" {
		t.Fatalf("graphics %q", got.Graphics)
	}
	if got.Blocks != "auto" {
		t.Fatalf("blocks %q", got.Blocks)
	}
	if got.Pieces != "classic" {
		t.Fatalf("pieces %q", got.Pieces)
	}
}

func TestOldSquareSizeIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{\"squareSize\":\"compact\",\"theme\":\"green\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "green" || got.Pieces != "classic" || got.Graphics != "off" {
		t.Fatalf("old square size disturbed the config: %+v", got)
	}
}

func TestGraphicsRoundTrip(t *testing.T) {
	if Default().Graphics != "off" {
		t.Fatalf("default %q", Default().Graphics)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	cfg.Graphics = "sixel"
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Graphics != "sixel" {
		t.Fatalf("graphics %q", got.Graphics)
	}
}
