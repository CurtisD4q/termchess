package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindExplicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stockfish")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Find(path)
	if err != nil || got != path {
		t.Fatalf("Find = %q %v", got, err)
	}
	if _, err := Find(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected an error for a missing binary")
	}
}
