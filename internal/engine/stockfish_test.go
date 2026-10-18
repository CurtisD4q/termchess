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
	sf, err := NewStockfish(path)
	if err != nil {
		t.Fatal(err)
	}
	defer sf.Close()
	b, err := sf.Bounds()
	if err != nil {
		t.Fatal(err)
	}
	if !b.EloSupported() || b.EloMin >= b.EloMax {
		t.Fatalf("bounds %+v", b)
	}
	if p := PlanFor(500, b); p.UseElo {
		t.Fatalf("500 used UCI_Elo with live floor %d", b.EloMin)
	}
	var covered int
	for _, elo := range Elos() {
		if elo >= b.EloMin && elo <= b.EloMax {
			covered = elo
			break
		}
	}
	if covered == 0 {
		t.Fatalf("no ladder rating inside %+v", b)
	}
	if p := PlanFor(covered, b); !p.UseElo || p.UCIElo != covered {
		t.Fatalf("plan %+v bounds %+v", p, b)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	mv, err := sf.BestMove(ctx, Request{
		Position: chess.NewGame().Position(),
		Elo:      covered,
		Budget:   80 * time.Millisecond,
	})
	if err != nil || mv == nil {
		t.Fatalf("search: %v %v", mv, err)
	}
}
