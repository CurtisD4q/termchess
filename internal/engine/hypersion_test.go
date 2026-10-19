package engine

import (
	"context"
	"testing"
	"time"

	"github.com/notnil/chess"
)

func TestLiveEloBounds(t *testing.T) {
	path, err := Find("")
	if err != nil {
		t.Skip(err)
	}
	eng, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	b, err := eng.Bounds()
	if err != nil {
		t.Fatal(err)
	}
	if !b.LimitStrength || b.EloMin > 500 || b.EloMax < 2500 {
		t.Fatalf("bounds %+v do not cover the 500–2500 ladder", b)
	}
	for _, elo := range Elos() {
		p := PlanFor(elo, b)
		if !p.UseElo || p.UCIElo != elo || p.Depth != 0 || p.Nodes != 0 || p.Skill != 0 || p.RandomPermille != 0 {
			t.Fatalf("%d plan %+v bounds %+v", elo, p, b)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	mv, err := eng.BestMove(ctx, Request{
		Position: chess.NewGame().Position(),
		Elo:      DefaultElo,
		Budget:   80 * time.Millisecond,
	})
	if err != nil || mv == nil {
		t.Fatalf("search: %v %v", mv, err)
	}
}
