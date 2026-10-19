package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"math/rand/v2"

	"github.com/notnil/chess"
	"github.com/notnil/chess/uci"
)

// Networks Hypersion loads from its working directory. They are not in the
// Hypersion git tree. When both files sit next to the binary, the process is
// pointed at those absolute paths so the search does not depend on the
// directory termchess was started from.
const (
	bigNetwork   = "nn-c288c895ea92.nnue"
	smallNetwork = "nn-37f18f62d772.nnue"
)

// Find locates a Hypersion binary. An explicit path wins. Otherwise PATH is
// searched for hypersion and Hypersion, then a few well-known install locations.
func Find(explicit string) (string, error) {
	if explicit != "" {
		st, err := os.Stat(explicit)
		if err != nil || st.IsDir() {
			return "", fmt.Errorf("engine not found at %s", explicit)
		}
		return explicit, nil
	}
	for _, name := range []string{"hypersion", "Hypersion"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	candidates := []string{
		"/usr/local/bin/hypersion",
		"/usr/local/bin/Hypersion",
		"/usr/bin/hypersion",
		"/usr/bin/Hypersion",
		"/opt/homebrew/bin/hypersion",
		"/opt/homebrew/bin/Hypersion",
		`C:\Program Files\Hypersion\Hypersion.exe`,
	}
	for _, c := range candidates {
		st, err := os.Stat(c)
		if err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("hypersion not found on PATH")
}

// Process is a UCI engine process. The vs-computer opponent is Hypersion.
type Process struct {
	path   string
	eng    *uci.Engine
	bounds Bounds
	rng    *rand.Rand
	mu     sync.Mutex
}

// New returns an engine that starts the process on the first search.
func New(path string) (*Process, error) {
	if path == "" {
		return nil, errors.New("engine path is empty")
	}
	return &Process{
		path: path,
		rng:  rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x9e3779b97f4a7c15)),
	}, nil
}

func (s *Process) ensure() error {
	if s.eng != nil {
		return nil
	}
	eng, err := uci.New(s.path)
	if err != nil {
		return err
	}
	if err := eng.Run(uci.CmdUCI); err != nil {
		_ = eng.Close()
		return err
	}
	s.eng = eng
	s.bounds = boundsFrom(eng.Options())
	if err := s.pinNetworks(); err != nil {
		_ = eng.Close()
		s.eng = nil
		return err
	}
	if err := eng.Run(uci.CmdIsReady, uci.CmdUCINewGame); err != nil {
		_ = eng.Close()
		s.eng = nil
		return err
	}
	return nil
}

// pinNetworks points EvalFile options at the networks beside the binary.
// Missing files are left alone: Hypersion then uses its classical evaluator.
func (s *Process) pinNetworks() error {
	dir := filepath.Dir(s.path)
	var cmds []uci.Cmd
	opts := s.eng.Options()
	if _, ok := opts["EvalFile"]; ok {
		if p := filepath.Join(dir, bigNetwork); fileExists(p) {
			cmds = append(cmds, uci.CmdSetOption{Name: "EvalFile", Value: p})
		}
	}
	if _, ok := opts["EvalFileSmall"]; ok {
		if p := filepath.Join(dir, smallNetwork); fileExists(p) {
			cmds = append(cmds, uci.CmdSetOption{Name: "EvalFileSmall", Value: p})
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return s.eng.Run(cmds...)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// Bounds starts the engine if needed and returns the UCI_Elo range it reported.
func (s *Process) Bounds() (Bounds, error) {
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
func (s *Process) BestMove(ctx context.Context, req Request) (*chess.Move, error) {
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
			return nil, errors.New("engine returned no move")
		}
		return r.mv, nil
	}
}

// apply pushes the plan's UCI options. LimitStrength is only sent when the
// engine advertised it. The UCI_Elo path is preferred whenever the plan
// selected it. Skill Level is the fallback for an engine with no usable range.
func (s *Process) apply(plan Plan) error {
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
func (s *Process) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eng == nil {
		return nil
	}
	err := s.eng.Close()
	s.eng = nil
	return err
}
