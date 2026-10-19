package engine

import (
	"testing"
	"time"

	"math/rand/v2"

	"github.com/notnil/chess"
)

func TestEloLadder(t *testing.T) {
	got := Elos()
	if len(got) != 52 {
		t.Fatalf("len = %d, want 52", len(got))
	}
	if got[0] != 500 || got[len(got)-1] != 2500 {
		t.Fatalf("ends = %d .. %d", got[0], got[len(got)-1])
	}
	want := map[int]bool{}
	for e := 500; e <= 1000; e += 100 {
		want[e] = true
	}
	for e := 1050; e <= 1700; e += 50 {
		want[e] = true
	}
	for e := 1725; e <= 2500; e += 25 {
		want[e] = true
	}
	if len(want) != 52 {
		t.Fatalf("expected set = %d", len(want))
	}
	prev := 0
	for _, e := range got {
		if !want[e] {
			t.Fatalf("unexpected rating %d", e)
		}
		if e <= prev {
			t.Fatalf("not strictly increasing at %d", e)
		}
		prev = e
		delete(want, e)
	}
	if len(want) != 0 {
		t.Fatalf("missing %d ratings", len(want))
	}
	if Nearest(0) != 500 || Nearest(9000) != 2500 || Nearest(1650) != 1650 {
		t.Fatalf("nearest endpoints failed: %d %d %d", Nearest(0), Nearest(9000), Nearest(1650))
	}
	// 1660 is closer to 1650 than to 1675. 1025 is equidistant and rounds up.
	if Nearest(1660) != 1650 {
		t.Fatalf("nearest 1660 = %d", Nearest(1660))
	}
	if Nearest(1025) != 1050 {
		t.Fatalf("tie 1025 snapped to %d", Nearest(1025))
	}
}

// narrowFloor is an engine whose UCI_Elo minimum sits above the bottom of the
// ladder (the old Stockfish 16 shape: 1320–3190). The production code reads
// the live engine; the test feeds the same shape in.
func narrowFloor() Bounds { return Bounds{LimitStrength: true, EloMin: 1320, EloMax: 3190} }

// hypersionBounds is what Hypersion reports: LimitStrength and UCI_Elo 500–3300.
func hypersionBounds() Bounds { return Bounds{LimitStrength: true, EloMin: 500, EloMax: 3300} }

func TestPlanMonotonicAndFallback(t *testing.T) {
	bounds := narrowFloor()
	var prev Plan
	for i, elo := range Elos() {
		p := PlanFor(elo, bounds)
		if p.Elo != elo {
			t.Fatalf("plan elo %d, want %d", p.Elo, elo)
		}
		if elo < bounds.EloMin {
			if p.UseElo {
				t.Fatalf("%d used UCI_Elo below the floor", elo)
			}
			if p.Skill != 0 {
				t.Fatalf("%d skill = %d, want 0 so it stays under the floor", elo, p.Skill)
			}
			if p.RandomPermille < 0 || p.RandomPermille > 1000 {
				t.Fatalf("%d random %d", elo, p.RandomPermille)
			}
		} else {
			if !p.UseElo || p.UCIElo != elo {
				t.Fatalf("%d UCI path = %+v", elo, p)
			}
			if p.RandomPermille != 0 || p.Depth != 0 || p.Nodes != 0 {
				t.Fatalf("%d UCI plan still has a fallback limit: %+v", elo, p)
			}
		}
		if i == 0 {
			prev = p
			continue
		}
		if p.Strength() <= prev.Strength() {
			t.Fatalf("strength fell from %d (%d) to %d (%d)", prev.Elo, prev.Strength(), p.Elo, p.Strength())
		}
		if !prev.UseElo && !p.UseElo {
			if p.Depth < prev.Depth || p.Nodes < prev.Nodes || p.MoveTime < prev.MoveTime || p.Skill < prev.Skill {
				t.Fatalf("fallback limits fell: %+v -> %+v", prev, p)
			}
			if p.RandomPermille > prev.RandomPermille {
				t.Fatalf("random rose: %d -> %d", prev.RandomPermille, p.RandomPermille)
			}
		}
		if !prev.UseElo && p.UseElo && p.Strength() <= prev.Strength() {
			t.Fatalf("crossing the floor did not get stronger")
		}
		prev = p
	}

	low := PlanFor(500, bounds)
	high := PlanFor(1300, bounds)
	if low.RandomPermille <= high.RandomPermille {
		t.Fatalf("500 should blunder more than 1300: %d vs %d", low.RandomPermille, high.RandomPermille)
	}
	if low.Depth > high.Depth || low.Nodes > high.Nodes || low.MoveTime > high.MoveTime {
		t.Fatalf("500 searched harder than 1300: %+v vs %+v", low, high)
	}
	floorElo := 0
	for _, e := range Elos() {
		if e >= bounds.EloMin {
			floorElo = e
			break
		}
	}
	floor := PlanFor(floorElo, bounds)
	if !floor.UseElo || floor.UCIElo != floorElo {
		t.Fatalf("floor plan %+v", floor)
	}
	if high.Strength() >= floor.Strength() {
		t.Fatalf("fallback %d (%d) is not weaker than UCI floor %d (%d)", high.Elo, high.Strength(), floorElo, floor.Strength())
	}
}

func TestPlanHypersionCoversLadder(t *testing.T) {
	bounds := hypersionBounds()
	if bounds.EloMin > 500 || bounds.EloMax < 2500 || !bounds.LimitStrength {
		t.Fatalf("fixture does not cover the ladder: %+v", bounds)
	}
	var prev Plan
	for i, elo := range Elos() {
		p := PlanFor(elo, bounds)
		if !p.UseElo || p.UCIElo != elo {
			t.Fatalf("%d did not use UCI_Elo: %+v", elo, p)
		}
		if p.Skill != 0 || p.Depth != 0 || p.Nodes != 0 || p.RandomPermille != 0 {
			t.Fatalf("%d still has a skill/depth fallback: %+v", elo, p)
		}
		if i > 0 && p.Strength() <= prev.Strength() {
			t.Fatalf("strength fell at %d", elo)
		}
		prev = p
	}
	if OpponentName(1500) != "Hypersion (1500)" || OpponentName(1640) != "Hypersion (1650)" {
		t.Fatalf("names %q %q", OpponentName(1500), OpponentName(1640))
	}
}

func TestPlanClampsAboveEngineMax(t *testing.T) {
	bounds := Bounds{LimitStrength: true, EloMin: 1320, EloMax: 2000}
	top := PlanFor(2500, bounds)
	if !top.UseElo || top.UCIElo != 2000 {
		t.Fatalf("clamp %+v", top)
	}
	below := PlanFor(1975, bounds)
	if top.Strength() <= below.Strength() {
		t.Fatalf("clamped 2500 (%d) did not outrank 1975 (%d)", top.Strength(), below.Strength())
	}
}

func TestPlanWithoutUCIElo(t *testing.T) {
	var prev Plan
	for i, elo := range Elos() {
		p := PlanFor(elo, Bounds{})
		if p.UseElo {
			t.Fatalf("%d used UCI_Elo with no range", elo)
		}
		if i > 0 && p.Strength() <= prev.Strength() {
			t.Fatalf("unsupported ladder fell at %d", elo)
		}
		prev = p
	}
	if bottom, top := PlanFor(500, Bounds{}), PlanFor(2500, Bounds{}); bottom.Skill != 0 || top.Skill != 20 {
		t.Fatalf("skill ends = %d .. %d", bottom.Skill, top.Skill)
	}
	if PlanFor(500, Bounds{}).RandomPermille <= PlanFor(2500, Bounds{}).RandomPermille {
		t.Fatal("random did not fall across the full ladder")
	}
}

func TestDiverge(t *testing.T) {
	pos := chess.NewGame().Position()
	searched := pos.ValidMoves()[0]
	rng := rand.New(rand.NewPCG(1, 2))
	if got := Diverge(pos, searched, 0, 0, rng); got != searched {
		t.Fatal("permille 0 should keep the search move")
	}
	if got := Diverge(pos, searched, 1000, 1000, rng); got != searched {
		t.Fatal("roll outside the window should keep the search move")
	}
	got := Diverge(pos, searched, 1000, 0, rng)
	if got == nil {
		t.Fatal("expected a legal move")
	}
	ok := false
	for _, m := range pos.ValidMoves() {
		if m.String() == got.String() {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatalf("diverged to a move not in the legal list: %s", got)
	}
}

func TestUCIMoveTimeGrows(t *testing.T) {
	bounds := narrowFloor()
	var prev time.Duration
	for _, elo := range Elos() {
		if elo < bounds.EloMin {
			continue
		}
		mt := PlanFor(elo, bounds).MoveTime
		if mt < prev {
			t.Fatalf("movetime fell at %d: %s < %s", elo, mt, prev)
		}
		prev = mt
	}
	if PlanFor(2500, bounds).MoveTime != SearchCeiling {
		t.Fatalf("top movetime %s", PlanFor(2500, bounds).MoveTime)
	}
}
