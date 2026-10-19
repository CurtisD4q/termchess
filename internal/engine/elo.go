package engine

import (
	"time"

	"math/rand/v2"

	"github.com/notnil/chess"
)

// SearchCeiling is the longest preferred movetime on the ladder, before the
// clock trims it. Weaker ratings ask for less; see PlanFor.
const SearchCeiling = 2 * time.Second

// Elos is the vs-computer ladder: 52 ratings.
//
//	500, 600, …, 1000          step 100  (6)
//	1050, 1100, …, 1700        step 50   (14)
//	1725, 1750, …, 2500        step 25   (32)
func Elos() []int {
	out := make([]int, 0, 52)
	for e := 500; e <= 1000; e += 100 {
		out = append(out, e)
	}
	for e := 1050; e <= 1700; e += 50 {
		out = append(out, e)
	}
	for e := 1725; e <= 2500; e += 25 {
		out = append(out, e)
	}
	return out
}

// Nearest snaps elo onto the ladder. Ties round toward the higher rating.
func Nearest(elo int) int {
	elos := Elos()
	best := elos[0]
	bestD := abs(elo - best)
	for _, e := range elos[1:] {
		d := abs(elo - e)
		if d < bestD || (d == bestD && e > best) {
			best, bestD = e, d
		}
	}
	return best
}

// DefaultElo is the rating selected for a new config.
const DefaultElo = 1500

// OpponentName is the clock label and PGN player name, e.g. "Hypersion (1650)".
func OpponentName(elo int) string {
	return "Hypersion (" + itoa(Nearest(elo)) + ")"
}

// Bounds is the UCI_Elo range reported by an engine at runtime.
// A zero EloMin means the engine did not advertise a usable range.
type Bounds struct {
	// LimitStrength is true when the engine accepts UCI_LimitStrength.
	LimitStrength bool
	EloMin        int
	EloMax        int
}

// EloSupported reports whether UCI_LimitStrength and a real UCI_Elo range
// are both available. The numbers come from the engine. Hypersion reports
// 500–3300, which covers this ladder.
func (b Bounds) EloSupported() bool {
	return b.LimitStrength && b.EloMin > 0 && b.EloMax >= b.EloMin
}

// Plan is how one rating is played.
//
// Mapping (also described in the README):
//
//   - Snap the requested rating onto Elos().
//
//   - If the engine supports UCI_Elo and the rating is inside [EloMin, EloMax],
//     set UCI_LimitStrength and UCI_Elo to that rating. When EloMin is at most
//     500 and EloMax is at least 2500, that is every step of this ladder:
//     no Skill Level, depth, nodes, or random-move fallback. Movetime grows
//     with the rating, from 400ms at 500 up to SearchCeiling at 2500, and is
//     then capped by the clock. Depth and nodes are left unlimited.
//
//   - Ratings above EloMax still use UCI_Elo, clamped to EloMax. Movetime
//     keeps growing with the requested rating so the ladder does not flatten.
//
//   - Ratings below EloMin cannot be sent as UCI_Elo (the engine would clamp
//     them all to the floor and they would play alike). They use a fallback
//     that gets stronger at every step:
//
//     skill stays 0. Skill Level 1 or more, searched deeply, can play above
//     the engine's own floor and would invert the ladder.
//     depth    = 1 + 5*i/(n-1)         // 1 .. 6
//     nodes    = 8 + 792*i/(n-1)       // 8 .. 800
//     movetime = 40ms + 460ms*i/(n-1)  // 40ms .. 500ms
//     random   = 850*(n-1-i)/(n-1)     // 850 .. 0 per mille
//
//     i is the index among ratings that fall below the floor, n is how many
//     of those there are. random is the chance of playing a random legal move
//     instead of the search result.
//
//   - If the engine has no UCI_Elo range, the same shape covers the whole
//     ladder and Skill Level rises too:
//
//     skill    = 20*i/(n-1)            // 0 .. 20
//     depth    = 1 + 11*i/(n-1)        // 1 .. 12
//     nodes    = 8 + 3992*i/(n-1)      // 8 .. 4000
//     movetime = 40ms + 1960ms*i/(n-1) // 40ms .. 2s
//     random   = 800*(n-1-i)/(n-1)     // 800 .. 0 per mille
type Plan struct {
	Elo int

	// UseElo selects the UCI_LimitStrength + UCI_Elo path.
	UseElo bool
	// UCIElo is the value to send, already clamped to the engine range.
	UCIElo int

	Skill          int
	Depth          int
	Nodes          int
	MoveTime       time.Duration
	RandomPermille int
}

// PlanFor builds the plan for a rating given the engine's reported bounds.
func PlanFor(elo int, bounds Bounds) Plan {
	elo = Nearest(elo)
	p := Plan{Elo: elo}
	if bounds.EloSupported() && elo >= bounds.EloMin {
		p.UseElo = true
		p.UCIElo = elo
		if p.UCIElo > bounds.EloMax {
			p.UCIElo = bounds.EloMax
		}
		p.MoveTime = uciMoveTime(elo)
		return p
	}
	below := ratingsBelow(bounds)
	i, n := 0, len(below)
	for idx, e := range below {
		if e == elo {
			i = idx
			break
		}
	}
	if n == 0 {
		n = 1
	}
	if bounds.EloSupported() {
		// Stay under the engine floor: skill 0, a short search, fewer blunders as i grows.
		p.Skill = 0
		p.Depth = 1 + scale(i, n, 5)
		p.Nodes = 8 + scale(i, n, 792)
		p.MoveTime = 40*time.Millisecond + time.Duration(scale(i, n, 460))*time.Millisecond
		p.RandomPermille = scale(n-1-i, n, 850)
		return p
	}
	p.Skill = scale(i, n, 20)
	p.Depth = 1 + scale(i, n, 11)
	p.Nodes = 8 + scale(i, n, 3992)
	p.MoveTime = 40*time.Millisecond + time.Duration(scale(i, n, 1960))*time.Millisecond
	p.RandomPermille = scale(n-1-i, n, 800)
	return p
}

// Strength ranks plans. It is a sort key, not an Elo estimate. Higher is stronger.
func (p Plan) Strength() int {
	ms := int(p.MoveTime / time.Millisecond)
	if p.UseElo {
		// Requested Elo, not the clamped UCI value, so a rating above the
		// engine max still outranks the one below it via movetime and Elo.
		return 2_000_000 + p.Elo*10 + ms
	}
	return p.Skill*100_000 + p.Depth*1_000 + p.Nodes*10 + ms - p.RandomPermille
}

// CapMoveTime shortens the plan so it fits the clock budget.
func (p Plan) CapMoveTime(budget time.Duration) Plan {
	if budget > 0 && budget < p.MoveTime {
		p.MoveTime = budget
	}
	return p
}

// Diverge returns a random legal move when roll falls inside permille.
// Otherwise it returns searched. roll is in [0, 1000).
func Diverge(pos *chess.Position, searched *chess.Move, permille, roll int, rng *rand.Rand) *chess.Move {
	if pos == nil || permille <= 0 || roll < 0 || roll >= permille || rng == nil {
		return searched
	}
	moves := pos.ValidMoves()
	if len(moves) == 0 {
		return searched
	}
	return moves[rng.IntN(len(moves))]
}

func ratingsBelow(bounds Bounds) []int {
	if !bounds.EloSupported() {
		return Elos()
	}
	var below []int
	for _, e := range Elos() {
		if e < bounds.EloMin {
			below = append(below, e)
		}
	}
	return below
}

func uciMoveTime(elo int) time.Duration {
	if elo < 500 {
		elo = 500
	}
	if elo > 2500 {
		elo = 2500
	}
	const span = 2500 - 500
	grow := time.Duration(elo-500) * (SearchCeiling - 400*time.Millisecond) / span
	return 400*time.Millisecond + grow
}

// scale maps i in 0..n-1 onto 0..span. n <= 1 yields 0.
func scale(i, n, span int) int {
	if n <= 1 || i <= 0 || span <= 0 {
		return 0
	}
	if i >= n-1 {
		return span
	}
	return (span * i) / (n - 1)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
