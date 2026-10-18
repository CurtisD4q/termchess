package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"math/rand/v2"

	"github.com/notnil/chess"
	"github.com/notnil/chess/uci"
)

// Find locates a Stockfish binary. An explicit path wins. Otherwise PATH is
// searched, then a few well-known install locations (Homebrew, Debian's
// /usr/games, Windows default).
func Find(explicit string) (string, error) {
	if explicit != "" {
		st, err := os.Stat(explicit)
		if err != nil || st.IsDir() {
			return "", fmt.Errorf("stockfish not found at %s", explicit)
		}
		return explicit, nil
	}
	if p, err := exec.LookPath("stockfish"); err == nil {
		return p, nil
	}
	candidates := []string{
		"/usr/games/stockfish",
		"/usr/bin/stockfish",
		"/usr/local/bin/stockfish",
		"/opt/homebrew/bin/stockfish",
		`C:\Program Files\Stockfish\stockfish.exe`,
	}
	for _, c := range candidates {
		st, err := os.Stat(c)
		if err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("stockfish not found on PATH")
}

// Stockfish is a UCI engine process.
type Stockfish struct {
	path   string
	eng    *uci.Engine
	bounds Bounds
	rng    *rand.Rand
	mu     sync.Mutex
}

// NewStockfish returns an engine that starts the process on the first search.
func NewStockfish(path string) (*Stockfish, error) {
	if path == "" {
		return nil, errors.New("stockfish path is empty")
	}
	return &Stockfish{
		path: path,
		rng:  rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x9e3779b97f4a7c15)),
	}, nil
}

func (s *Stockfish) ensure() error {
	if s.eng != nil {
		return nil
	}
	eng, err := uci.New(s.path)
	if err != nil {
		return err
	}
	if err := eng.Run(uci.CmdUCI, uci.CmdIsReady, uci.CmdUCINewGame); err != nil {
		_ = eng.Close()
		return err
	}
	s.eng = eng
	s.bounds = boundsFrom(eng.Options())
	return nil
}

// Bounds starts the engine if needed and returns the UCI_Elo range it reported.
func (s *Stockfish) Bounds() (Bounds, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensure(); err != nil {
		return Bounds{}, err
	}
	return s.bounds, nil
}

func boundsFrom(opts map[string]uci.Option) Bounds {
	var b Bounds
	if _, ok := opts["UCI_LimitStrength"]; ok {
		b.LimitStrength = true
	}
	o, ok := opts["UCI_Elo"]
	if !ok {
		return b
	}
	min, err1 := strconv.Atoi(o.Min)
	max, err2 := strconv.Atoi(o.Max)
	if err1 != nil || err2 != nil || min <= 0 || max < min {
		return b
	}
	b.EloMin = min
	b.EloMax = max
	return b
}

// BestMove runs one search. It blocks the calling goroutine, which must not
// be the Bubble Tea update loop. The rating is turned into a Plan using the
// UCI_Elo min/max this process reported.
func (s *Stockfish) BestMove(ctx context.Context, req Request) (*chess.Move, error) {
	type result struct {
		mv  *chess.Move
		err error
	}
	ch := make(chan result, 1)
	go func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.ensure(); err != nil {
			ch <- result{err: err}
			return
		}
		if req.Position == nil {
			ch <- result{err: errors.New("missing position")}
			return
		}
		plan := PlanFor(req.Elo, s.bounds).CapMoveTime(req.Budget)
		if plan.RandomPermille > 0 {
			roll := s.rng.IntN(1000)
			if mv := Diverge(req.Position, nil, plan.RandomPermille, roll, s.rng); mv != nil {
				ch <- result{mv: mv}
				return
			}
		}
		if err := s.apply(plan); err != nil {
			ch <- result{err: err}
			return
		}
		goCmd := uci.CmdGo{MoveTime: plan.MoveTime}
		if !plan.UseElo {
			goCmd.Depth = plan.Depth
			goCmd.Nodes = plan.Nodes
		}
		if goCmd.MoveTime <= 0 {
			goCmd.MoveTime = 40 * time.Millisecond
		}
		err := s.eng.Run(uci.CmdPosition{Position: req.Position}, goCmd)
		if err != nil {
			ch <- result{err: err}
			return
		}
		ch <- result{mv: s.eng.SearchResults().BestMove}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		if r.mv == nil {
			return nil, errors.New("stockfish returned no move")
		}
		return r.mv, nil
	}
}

// apply pushes the plan's UCI options. LimitStrength is only sent when the
// engine advertised it, so an engine without UCI_Elo still receives Skill Level.
func (s *Stockfish) apply(plan Plan) error {
	if plan.UseElo {
		return s.eng.Run(
			uci.CmdSetOption{Name: "UCI_LimitStrength", Value: "true"},
			uci.CmdSetOption{Name: "UCI_Elo", Value: strconv.Itoa(plan.UCIElo)},
		)
	}
	var cmds []uci.Cmd
	if s.bounds.LimitStrength {
		cmds = append(cmds, uci.CmdSetOption{Name: "UCI_LimitStrength", Value: "false"})
	}
	cmds = append(cmds, uci.CmdSetOption{Name: "Skill Level", Value: strconv.Itoa(plan.Skill)})
	return s.eng.Run(cmds...)
}

// Close stops the engine process.
func (s *Stockfish) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eng == nil {
		return nil
	}
	err := s.eng.Close()
	s.eng = nil
	return err
}
