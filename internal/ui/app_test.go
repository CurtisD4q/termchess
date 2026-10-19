package ui

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/notnil/chess"
	"termchess/internal/config"
	"termchess/internal/engine"
	"termchess/internal/game"
	"termchess/internal/gfx"
	"termchess/internal/interact"
)

func testApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.MovePaceMs = 0
	a := New(Options{
		Logger:     log.New(io.Discard),
		Config:     cfg,
		ConfigPath: filepath.Join(dir, "config.json"),
		DataDir:    filepath.Join(dir, "games"),
		Engine:     &engine.Mock{},
		EngineOK:   true,
	})
	a.width = 140
	a.height = 64
	a.dark = true
	return a
}

func press(s string) tea.KeyPressMsg {
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	default:
		r := []rune(s)[0]
		return tea.KeyPressMsg{Text: s, Code: r}
	}
}

func send(a *App, s string) *App {
	m, cmd := a.Update(press(s))
	return pump(m.(*App), cmd)
}

// pump runs a command and the messages it produces. tea.Tick blocks for the
// clock interval, so a command that does not return quickly is abandoned.
func pump(a *App, cmd tea.Cmd) *App {
	return pumpDepth(a, cmd, 10)
}

func pumpDepth(a *App, cmd tea.Cmd, depth int) *App {
	if cmd == nil || depth <= 0 || a == nil {
		return a
	}
	msg := runCmd(cmd)
	if msg == nil {
		return a
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			a = pumpDepth(a, c, depth-1)
		}
		return a
	}
	model, next := a.Update(msg)
	return pumpDepth(model.(*App), next, depth-1)
}

func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() {
		ch <- cmd()
	}()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(40 * time.Millisecond):
		return nil
	}
}

func TestKeyStrings(t *testing.T) {
	if press("up").String() != "up" || press("enter").String() != "enter" || press("space").String() != "space" {
		t.Fatalf("up=%q enter=%q space=%q", press("up").String(), press("enter").String(), press("space").String())
	}
	if press("ctrl+c").String() != "ctrl+c" || press("?").String() != "?" {
		t.Fatalf("ctrl=%q question=%q", press("ctrl+c").String(), press("?").String())
	}
}

func TestCustomFieldsHiddenUnlessCustom(t *testing.T) {
	a := testApp(t)
	model, cmd := a.openSetup(game.ModeLocal)
	a = pump(model.(*App), cmd)
	model, cmd = a.Update(tea.WindowSizeMsg{Width: 100, Height: 36})
	a = pump(model.(*App), cmd)

	a = send(a, "enter") // flip
	a = send(a, "enter") // Blitz 5+0, not Custom. The form finishes.
	view := stripSGR(a.View().Content)
	if strings.Contains(view, "Custom minutes") || strings.Contains(view, "Square size") {
		t.Fatalf("preset setup still asks for a clock or a square size:\n%s", view)
	}

	a = testApp(t)
	model, cmd = a.openSetup(game.ModeLocal)
	a = pump(model.(*App), cmd)
	model, cmd = a.Update(tea.WindowSizeMsg{Width: 100, Height: 36})
	a = pump(model.(*App), cmd)
	a = send(a, "enter") // flip
	for i := 0; i < 3; i++ {
		a = send(a, "down") // 5+0 → 10+0 → 15+10 → Custom
	}
	a = send(a, "enter")
	view = stripSGR(a.View().Content)
	if !strings.Contains(view, "┃ Custom minutes") {
		t.Fatalf("custom minutes not focused after choosing Custom:\n%s", view)
	}
	a = send(a, "enter")
	view = stripSGR(a.View().Content)
	if !strings.Contains(view, "┃ Custom increment") {
		t.Fatalf("enter did not leave custom minutes:\n%s", view)
	}
	a = send(a, "enter")
	view = stripSGR(a.View().Content)
	if strings.Contains(view, "Custom minutes") || strings.Contains(view, "Square size") {
		t.Fatalf("custom clock did not finish the form:\n%s", view)
	}

	// The computer form uses the same split.
	a = testApp(t)
	model, cmd = a.openSetup(game.ModeComputer)
	a = pump(model.(*App), cmd)
	model, cmd = a.Update(tea.WindowSizeMsg{Width: 100, Height: 36})
	a = pump(model.(*App), cmd)
	a = send(a, "enter") // colour
	a = send(a, "enter") // elo
	a = send(a, "enter") // time control
	view = stripSGR(a.View().Content)
	if strings.Contains(view, "Custom minutes") || strings.Contains(view, "Square size") {
		t.Fatalf("computer setup still asks for a clock or a square size:\n%s", view)
	}
}

func TestSetupEloPicker(t *testing.T) {
	a := testApp(t)
	a.cfg.Last.Elo = 1650
	model, cmd := a.openSetup(game.ModeComputer)
	a = pump(model.(*App), cmd)
	view := a.View().Content
	if !strings.Contains(view, "Engine Elo") || !strings.Contains(view, "1650") || !strings.Contains(view, "/ filters") {
		t.Fatalf("setup view:\n%s", view)
	}
}

func TestMenuMentionsModes(t *testing.T) {
	a := testApp(t)
	view := a.View().Content
	if !strings.Contains(view, "two-player") || !strings.Contains(view, "computer") {
		t.Fatalf("menu view:\n%s", view)
	}
}

func TestCursorMoveAndFlipWithKeys(t *testing.T) {
	a := testApp(t)
	s, err := game.New(game.Options{Mode: game.ModeLocal, TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	if a.machine.Cursor != chess.E2 {
		t.Fatalf("start cursor %s", a.machine.Cursor)
	}
	a = send(a, "up")
	if a.machine.Cursor != chess.E3 {
		t.Fatalf("up -> %s", a.machine.Cursor)
	}
	a = send(a, "f")
	a = send(a, "up")
	// Flipped, up is toward the top of the screen, which is now toward rank 1.
	if a.machine.Cursor != chess.E2 {
		t.Fatalf("flipped up -> %s, want e2", a.machine.Cursor)
	}
}

func TestPlayE4WithKeys(t *testing.T) {
	a := testApp(t)
	s, err := game.New(game.Options{Mode: game.ModeLocal, TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	a = send(a, "enter")
	if a.machine.Phase != interact.PhaseSelected {
		t.Fatalf("phase %s", a.machine.Phase)
	}
	a = send(a, "up")
	a = send(a, "up")
	a = send(a, "enter")
	if a.session.Piece(chess.E4).Type() != chess.Pawn {
		t.Fatal("e4 was not played")
	}
	if a.machine.Cursor != chess.E4 {
		t.Fatalf("cursor stayed at %s", a.machine.Cursor)
	}
	view := a.View().Content
	if !strings.Contains(view, "e4") {
		t.Fatalf("history missing e4:\n%s", view)
	}
	// Illegal square keeps the selection.
	a = send(a, "enter") // black to move, cursor on e4 which is white's pawn
	if !strings.Contains(a.toast, "Illegal") {
		t.Fatalf("toast %q", a.toast)
	}
}

func TestCursorStaysForOpponentPremoveAndUndo(t *testing.T) {
	// Own move still lands on the destination. The engine reply does not
	// pull the cursor onto that reply.
	a := computerApp(t, "e7e5")
	a, think := confirmE4(t, a)
	if a.machine.Cursor != chess.E4 {
		t.Fatalf("own move cursor %s", a.machine.Cursor)
	}
	a = send(a, "left")
	a = send(a, "left")
	parked := a.machine.Cursor
	if parked == chess.E5 {
		t.Fatal("parked on the reply")
	}
	a = pump(a, think)
	if a.session.Piece(chess.E5).Type() != chess.Pawn {
		t.Fatalf("engine reply missing: %s", a.session.Game().FEN())
	}
	if a.machine.Cursor != parked {
		t.Fatalf("engine reply moved cursor from %s to %s", parked, a.machine.Cursor)
	}

	// A pre-move plays without touching the cursor.
	a = computerApp(t, "e7e5")
	a, think = confirmE4(t, a)
	for _, k := range []string{"right", "right", "down", "down", "down", "enter", "left", "up", "up", "enter"} {
		a = send(a, k)
	}
	if a.machine.Premove == nil || a.machine.Premove.To != chess.F3 {
		t.Fatalf("premove %+v", a.machine.Premove)
	}
	a = send(a, "left")
	parked = a.machine.Cursor
	a = pump(a, think)
	if a.session.Piece(chess.F3).Type() != chess.Knight {
		t.Fatalf("premove did not play: %s", a.session.Game().FEN())
	}
	if a.machine.Cursor != parked {
		t.Fatalf("premove moved cursor from %s to %s", parked, a.machine.Cursor)
	}

	// Undo does not send the cursor to the remaining move.
	a = testApp(t)
	s, err := game.New(game.Options{Mode: game.ModeLocal, TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	a = playE4(t, a)
	if a.machine.Cursor != chess.E4 {
		t.Fatalf("white cursor %s", a.machine.Cursor)
	}
	// Black plays e5. The cursor returns to e4, where white left it.
	for _, k := range []string{"up", "up", "up", "enter", "down", "down", "enter"} {
		a = send(a, k)
	}
	if a.session.Piece(chess.E5).Type() != chess.Pawn {
		t.Fatalf("e5 missing: %s", a.session.Game().FEN())
	}
	if a.machine.Cursor != chess.E4 {
		t.Fatalf("black's move moved cursor to %s", a.machine.Cursor)
	}
	// White's next move sits on its own destination, not on e5.
	for _, k := range []string{"down", "down", "down", "right", "right", "enter", "left", "up", "up", "enter"} {
		a = send(a, k)
	}
	if a.session.Piece(chess.F3).Type() != chess.Knight {
		t.Fatalf("Nf3 missing: %s", a.session.Game().FEN())
	}
	if a.machine.Cursor != chess.F3 {
		t.Fatalf("white's second move cursor %s", a.machine.Cursor)
	}
	a = send(a, "down")
	a = send(a, "left")
	parked = a.machine.Cursor
	a = send(a, "u")
	if a.session.MoveCount() != 2 {
		t.Fatalf("plies %d", a.session.MoveCount())
	}
	if a.machine.Cursor != parked {
		t.Fatalf("undo moved cursor from %s to %s", parked, a.machine.Cursor)
	}

	// Two-player: applying the opponent's move while the cursor is elsewhere
	// does not pull it onto their destination.
	a = testApp(t)
	s, err = game.New(game.Options{Mode: game.ModeLocal, TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	a = playE4(t, a)
	a.machine.Cursor = chess.A5
	reply, err := (chess.UCINotation{}).Decode(a.session.Game().Position(), "e7e5")
	if err != nil {
		t.Fatal(err)
	}
	model, cmd := a.applyPlayed(reply, false, false)
	a = pump(model.(*App), cmd)
	if a.session.Piece(chess.E5).Type() != chess.Pawn {
		t.Fatalf("opponent move missing: %s", a.session.Game().FEN())
	}
	if a.machine.Cursor != chess.A5 {
		t.Fatalf("opponent move moved cursor to %s", a.machine.Cursor)
	}
}

func computerApp(t *testing.T, reply string) *App {
	t.Helper()
	mock := &engine.Mock{Moves: []string{reply}}
	dir := t.TempDir()
	cfg := config.Default()
	cfg.MovePaceMs = 0
	a := New(Options{
		Logger:     log.New(io.Discard),
		Config:     cfg,
		ConfigPath: filepath.Join(dir, "c.json"),
		DataDir:    filepath.Join(dir, "g"),
		Engine:     mock,
		EngineOK:   true,
	})
	a.width, a.height = 140, 64
	s, err := game.New(game.Options{
		Mode:      game.ModeComputer,
		Human:     chess.White,
		WhiteName: "You",
		BlackName: "Hypersion",
		TC:        game.PresetByID("none"),
		Elo:       1500,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, cmd := a.useSession(s)
	return pump(a, cmd)
}

func playE4(t *testing.T, a *App) *App {
	t.Helper()
	a, cmd := confirmE4(t, a)
	return pump(a, cmd)
}

// confirmE4 plays 1.e4 and returns the follow-up command without running it.
// Against the computer that command is the engine search.
func confirmE4(t *testing.T, a *App) (*App, tea.Cmd) {
	t.Helper()
	a = send(a, "enter")
	a = send(a, "up")
	a = send(a, "up")
	model, cmd := a.Update(press("enter"))
	a = model.(*App)
	if a.session.Piece(chess.E4).Type() != chess.Pawn {
		t.Fatalf("e4 was not played: %s", a.session.Game().FEN())
	}
	return a, cmd
}

func TestPromotionKeys(t *testing.T) {
	a := testApp(t)
	s, err := game.New(game.Options{
		FEN: "k7/4P3/8/8/8/8/8/4K3 w - - 0 1",
		TC:  game.PresetByID("none"),
	})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	a.machine.Cursor = chess.E7
	a = send(a, "enter")
	a = send(a, "up")
	a = send(a, "enter")
	if a.machine.Phase != interact.PhasePromotion {
		t.Fatalf("phase %s", a.machine.Phase)
	}
	view := a.View().Content
	if !strings.Contains(view, "Promote") {
		t.Fatalf("no picker:\n%s", view)
	}
	a = send(a, "right")
	a = send(a, "left")
	a = send(a, "enter")
	if a.session.Piece(chess.E8) != chess.WhiteQueen {
		t.Fatalf("piece %s", a.session.Piece(chess.E8))
	}
}

func TestUndoAndHelp(t *testing.T) {
	a := testApp(t)
	s, err := game.New(game.Options{Mode: game.ModeLocal, TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	a = send(a, "enter")
	a = send(a, "up")
	a = send(a, "up")
	a = send(a, "enter")
	a = send(a, "u")
	if a.session.MoveCount() != 0 {
		t.Fatalf("plies %d", a.session.MoveCount())
	}
	a = send(a, "?")
	if !a.fullHelp || !strings.Contains(a.View().Content, "resign") {
		t.Fatal("full help did not open")
	}
	a = send(a, "tab")
	if !a.focusHist {
		t.Fatal("history not focused")
	}
	before := a.machine.Cursor
	a = send(a, "up")
	if a.machine.Cursor != before {
		t.Fatalf("history focus moved the cursor from %s to %s", before, a.machine.Cursor)
	}
}

func TestQuitConfirm(t *testing.T) {
	a := testApp(t)
	s, err := game.New(game.Options{Mode: game.ModeLocal, TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	m, cmd := a.Update(press("e")) // not a binding; play a move first so the game is underway
	a = m.(*App)
	_ = cmd
	a = send(a, "enter")
	a = send(a, "up")
	a = send(a, "up")
	a = send(a, "enter")
	a = send(a, "q")
	if a.formKind != formConfirm || a.confirmKind != confirmQuit {
		t.Fatalf("confirm %v %v", a.formKind, a.confirmKind)
	}
	if !strings.Contains(a.View().Content, "Quit this game?") {
		t.Fatalf("dialog missing:\n%s", a.View().Content)
	}
	a = send(a, "n")
	if a.formKind != formNone || a.screen != screenPlay {
		t.Fatalf("after decline form=%v screen=%v", a.formKind, a.screen)
	}
	if a.session == nil || a.session.MoveCount() != 1 {
		t.Fatal("the game should still be in progress")
	}
}

func TestQuitConfirmDecline(t *testing.T) {
	a := testApp(t)
	s, err := game.New(game.Options{Mode: game.ModeLocal, TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	a = send(a, "enter")
	a = send(a, "up")
	a = send(a, "up")
	a = send(a, "enter")
	model, cmd := a.Update(press("q"))
	a = model.(*App)
	a = pump(a, cmd)
	if a.formKind != formConfirm {
		t.Fatalf("form %v", a.formKind)
	}
	model, cmd = a.Update(press("n"))
	a = pump(model.(*App), cmd)
	if a.formKind != formNone || a.screen != screenPlay {
		t.Fatalf("after decline form=%v screen=%v", a.formKind, a.screen)
	}
	if a.session.MoveCount() != 1 {
		t.Fatalf("plies %d", a.session.MoveCount())
	}
}

func TestPremoveAndEngine(t *testing.T) {
	mock := &engine.Mock{Moves: []string{"e7e5"}}
	dir := t.TempDir()
	cfg := config.Default()
	cfg.MovePaceMs = 0
	a := New(Options{
		Logger:     log.New(io.Discard),
		Config:     cfg,
		ConfigPath: filepath.Join(dir, "c.json"),
		DataDir:    filepath.Join(dir, "g"),
		Engine:     mock,
		EngineOK:   true,
	})
	a.width, a.height = 140, 64
	s, err := game.New(game.Options{
		Mode:      game.ModeComputer,
		Human:     chess.White,
		WhiteName: "You",
		BlackName: "Hypersion",
		TC:        game.PresetByID("none"),
		Elo:       1500,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, cmd := a.useSession(s)
	a = pump(a, cmd)
	a = send(a, "enter")
	a = send(a, "up")
	a = send(a, "up")
	model, cmd := a.Update(press("enter"))
	a = model.(*App)
	if a.machine.Phase != interact.PhaseEngine {
		t.Fatalf("phase %s", a.machine.Phase)
	}
	// Park the engine command; set a pre-move first.
	think := cmd
	// g1 from e4: right right down down down
	for _, k := range []string{"right", "right", "down", "down", "down", "enter", "left", "up", "up", "enter"} {
		a = send(a, k)
	}
	if a.machine.Premove == nil || a.machine.Premove.To != chess.F3 {
		t.Fatalf("premove %+v cursor %s phase %s", a.machine.Premove, a.machine.Cursor, a.machine.Phase)
	}
	// The player leaves the cursor off both the engine reply and the pre-move.
	for _, k := range []string{"down", "down", "left", "left"} {
		a = send(a, k)
	}
	parked := a.machine.Cursor
	if parked == chess.E5 || parked == chess.F3 {
		t.Fatalf("parked on a move square %s", parked)
	}
	a = pump(a, think)
	if a.machine.Cursor != parked {
		t.Fatalf("cursor moved from %s to %s", parked, a.machine.Cursor)
	}
	if a.session.Piece(chess.E5).Type() != chess.Pawn || a.session.Piece(chess.F3).Type() != chess.Knight {
		t.Fatalf("fen %s", a.session.Game().FEN())
	}
	if mock.Calls < 1 {
		t.Fatal("engine was not asked")
	}
}

func TestFlagFallThroughTick(t *testing.T) {
	a := testApp(t)
	s, err := game.New(game.Options{
		Mode: game.ModeLocal,
		TC:   game.Custom(0, 0), // minutes 0 -> untimed; set a tiny clock instead
	})
	if err != nil {
		t.Fatal(err)
	}
	// Rebuild with a 200ms clock.
	s, err = game.New(game.Options{Mode: game.ModeLocal, TC: game.TimeControl{ID: "t", Name: "t", Initial: 200 * time.Millisecond}})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	// Drive the bubbles timer the same way the tick loop does.
	for i := 0; i < 5 && !a.session.Over(); i++ {
		a.onTick()
	}
	if !a.session.Over() || !strings.Contains(a.session.ResultText(), "flagged") {
		t.Fatalf("result %q over %v remaining %s", a.session.ResultText(), a.session.Over(), a.session.Clock().Remaining(chess.White))
	}
}

func TestTerminalTooSmall(t *testing.T) {
	a := testApp(t)
	s, err := game.New(game.Options{TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	a.width, a.height = 20, 10
	if !strings.Contains(a.View().Content, "please enlarge the terminal") {
		t.Fatalf("view:\n%s", a.View().Content)
	}
}

func TestBoardAlignment(t *testing.T) {
	a := testApp(t)
	s, err := game.New(game.Options{TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	a.themeID = "mono"
	view := a.View().Content
	if !strings.Contains(view, "a") || !strings.Contains(view, "8") {
		t.Fatal("labels missing")
	}
	// The grid lines (those that start with a rank digit after styling) are
	// checked via the pure renderer.
	st := a.styles()
	ctx := cellCtx{session: s, machine: a.machine, styles: st}
	board := renderBoard(ctx, 3, 1, false)
	lines := strings.Split(board, "\n")
	width := -1
	for i, ln := range lines {
		w := lipWidth(ln)
		if width < 0 {
			width = w
		}
		if w != width {
			t.Fatalf("line %d width %d, want %d\n%s", i, w, width, board)
		}
	}
}

func lipWidth(s string) int {
	return lipgloss.Width(s)
}

func TestMovePaceHoldsTheEngineReply(t *testing.T) {
	a := testApp(t)
	a.pace = 400 * time.Millisecond
	s, err := game.New(game.Options{Mode: game.ModeLocal, TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	model, cmd := a.commit(mustUCI(t, s, "e2e4"))
	a = model.(*App)
	if cmd == nil || a.session.MoveCount() != 0 || len(a.pending) != 1 {
		t.Fatalf("first move plies %d pending %d", a.session.MoveCount(), len(a.pending))
	}
	model, cmd = a.Update(paceMsg{at: a.pending[0].when})
	a = pump(model.(*App), cmd)
	if a.session.MoveCount() != 1 {
		t.Fatal("first move should land on the opening beat")
	}
	a.thinkGen = 4
	model, cmd = a.onEngine(engineMsg{gen: 4, move: mustUCI(t, a.session, "e7e5")})
	a = model.(*App)
	if cmd == nil || a.session.MoveCount() != 1 || len(a.pending) != 1 {
		t.Fatalf("reply landed early: plies %d pending %d", a.session.MoveCount(), len(a.pending))
	}
	if gap := a.pending[0].when.Sub(a.lastApply); gap < 400*time.Millisecond {
		t.Fatalf("gap %s", gap)
	}
	early := a.pending[0].when.Add(-time.Millisecond)
	model, cmd = a.Update(paceMsg{at: early})
	a = pump(model.(*App), cmd)
	if a.session.MoveCount() != 1 {
		t.Fatal("reply landed before the beat")
	}
	due := a.pending[0].when
	model, cmd = a.Update(paceMsg{at: due})
	a = pump(model.(*App), cmd)
	if a.session.MoveCount() != 2 || a.session.Piece(chess.E5).Type() != chess.Pawn {
		t.Fatalf("fen %s plies %d", a.session.Game().FEN(), a.session.MoveCount())
	}
}

func TestKittyOverlayOnE2(t *testing.T) {
	a := testApp(t)
	a.graphics = gfx.Kitty
	s, err := game.New(game.Options{TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	a.width, a.height = 140, 64
	view := a.View().Content
	// Explicit kitty graphics may prefix a transmit, but the squares themselves
	// stay the block pieces. A placeholder would be invisible when the image
	// does not land.
	plain := stripSGR(view)
	if !strings.Contains(plain, "█") || !strings.Contains(view, "38;2;255;255;255") || !strings.Contains(view, "38;2;0;0;0") {
		t.Fatal("kitty graphics hid the block pieces")
	}
	if strings.Contains(view, "\U0010EEEE") {
		t.Fatal("squares were replaced with kitty placeholders")
	}
	if strings.Contains(view, "a=d") || strings.Contains(view, "\x1b[22;32H") || strings.Contains(view, "z=-1") {
		t.Fatal("view still deletes or cursor-places images")
	}
	a = send(a, "q")
	dialog := a.View().Content
	if !strings.Contains(dialog, "Quit this game?") {
		t.Fatal("quit dialog missing")
	}
	if !strings.Contains(stripSGR(dialog), "█") || strings.Contains(dialog, "\U0010EEEE") {
		t.Fatal("quit dialog lost the block pieces")
	}
}

func TestGraphicsOffStaysText(t *testing.T) {
	a := testApp(t)
	s, err := game.New(game.Options{TC: game.PresetByID("none")})
	if err != nil {
		t.Fatal(err)
	}
	a.useSession(s)
	a.width, a.height = 140, 64
	view := a.View().Content
	if strings.Contains(view, "\x1b_G") || strings.Contains(view, "1337;") || strings.Contains(view, "\x1bPq") {
		t.Fatal("graphics off still emitted an image protocol")
	}
}

func mustUCI(t *testing.T, s *game.Session, u string) *chess.Move {
	t.Helper()
	mv, err := (chess.UCINotation{}).Decode(s.Game().Position(), u)
	if err != nil {
		t.Fatal(err)
	}
	return mv
}
