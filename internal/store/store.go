// Package store reads and writes PGN files in the user's data directory.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/notnil/chess"
)

// Entry is one saved game.
type Entry struct {
	Name    string
	Path    string
	Title   string
	ModTime time.Time
}

// SafeName keeps a file inside the games directory.
func SafeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("name is empty")
	}
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid name")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".pgn") {
		name += ".pgn"
	}
	return name, nil
}

// Save writes pgn to dir/name and returns the full path.
func Save(dir, name, pgn string) (string, error) {
	safe, err := SafeName(name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, safe)
	if err := os.WriteFile(path, []byte(pgn), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Read loads a saved game by file name.
func Read(dir, name string) (string, error) {
	safe, err := SafeName(name)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(dir, safe))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// List returns saved games, newest first.
func List(dir string) ([]Entry, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".pgn") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, e.Name())
		out = append(out, Entry{
			Name:    e.Name(),
			Path:    path,
			Title:   describe(path, e.Name()),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out, nil
}

func describe(path, name string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return name
	}
	fn, err := chess.PGN(strings.NewReader(string(b)))
	if err != nil {
		return name
	}
	g := chess.NewGame(fn)
	white, black, result, date := "White", "Black", string(g.Outcome()), ""
	if tp := g.GetTagPair("White"); tp != nil && tp.Value != "" {
		white = tp.Value
	}
	if tp := g.GetTagPair("Black"); tp != nil && tp.Value != "" {
		black = tp.Value
	}
	if tp := g.GetTagPair("Date"); tp != nil {
		date = tp.Value
	}
	title := fmt.Sprintf("%s vs %s  %s", white, black, result)
	if date != "" {
		title += "  " + date
	}
	return title
}
