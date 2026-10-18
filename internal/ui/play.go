package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/notnil/chess"
	"termchess/assets/pieces"
	"termchess/internal/coord"
	"termchess/internal/game"
	"termchess/internal/gfx"
	"termchess/internal/interact"
	"termchess/internal/theme"
)

func (a *App) onPlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.machine == nil || a.session == nil {
		return a, nil
	}
	a.syncKeys()
	promo := a.machine.Phase == interact.PhasePromotion || a.machine.Phase == interact.PhasePremovePromo
	if promo {
		switch {
		case key.Matches(msg, a.keys.Left):
			a.machine.PromoLeft()
		case key.Matches(msg, a.keys.Right):
			a.machine.PromoRight()
		case key.Matches(msg, a.keys.Select):
			return a.afterConfirm(a.machine.Confirm(a.session))
		case key.Matches(msg, a.keys.Cancel):
			a.machine.Cancel()
		}
		return a, nil
	}
	switch {
	case key.Matches(msg, a.keys.Help):
		a.fullHelp = !a.fullHelp
		return a, nil
	case key.Matches(msg, a.keys.Focus):
		a.focusHist = !a.focusHist
		return a, nil
	case key.Matches(msg, a.keys.Cancel):
		a.machine.Cancel()
		return a, nil
	case a.focusHist && (key.Matches(msg, a.keys.Up) || key.Matches(msg, a.keys.Down) || key.Matches(msg, a.keys.Left) || key.Matches(msg, a.keys.Right)):
		a.scrollHistory(msg)
		return a, nil
	case key.Matches(msg, a.keys.Up):
		a.machine.MoveCursor(coord.Up, a.session.BoardFlipped())
		return a, nil
	case key.Matches(msg, a.keys.Down):
		a.machine.MoveCursor(coord.Down, a.session.BoardFlipped())
		return a, nil
	case key.Matches(msg, a.keys.Left):
		a.machine.MoveCursor(coord.Left, a.session.BoardFlipped())
		return a, nil
	case key.Matches(msg, a.keys.Right):
		a.machine.MoveCursor(coord.Right, a.session.BoardFlipped())
		return a, nil
	case key.Matches(msg, a.keys.Select):
		return a.afterConfirm(a.machine.Confirm(a.session))
	case key.Matches(msg, a.keys.Undo):
		return a.undo()
	case key.Matches(msg, a.keys.Flip):
		a.session.ToggleFlip()
		return a, nil
	case key.Matches(msg, a.keys.Theme):
		a.themeID = theme.Next(a.themeID)
		a.cfg.Theme = string(a.themeID)
		return a, a.saveCfg()
	case key.Matches(msg, a.keys.Save):
		a.filename = ""
		a.form = a.saveForm()
		a.formKind = formSave
		return a, a.form.Init()
	case key.Matches(msg, a.keys.Open):
		if a.session.Unsaved() {
			return a.ask(confirmLoad, "Load over this unsaved game?")
		}
		return a, a.listCmd()
	case key.Matches(msg, a.keys.New):
		if a.session.MoveCount() > 0 && (a.session.Unsaved() || !a.session.Over()) {
			return a.ask(confirmNew, "Start a new game?")
		}
		return a.openMenu()
	case key.Matches(msg, a.keys.Resign):
		if a.session.Over() {
			return a, nil
		}
		return a.ask(confirmResign, "Resign?")
	case key.Matches(msg, a.keys.Draw):
		return a.onDraw()
	}
	return a, nil
}

func (a *App) scrollHistory(msg tea.KeyPressMsg) {
	switch {
	case key.Matches(msg, a.keys.Up):
		a.history.ScrollUp(1)
	case key.Matches(msg, a.keys.Down):
		a.history.ScrollDown(1)
	case key.Matches(msg, a.keys.Left):
		a.history.ScrollUp(a.history.Height())
	case key.Matches(msg, a.keys.Right):
		a.history.ScrollDown(a.history.Height())
	}
}

func (a *App) afterConfirm(res interact.Result) (tea.Model, tea.Cmd) {
	if res.Flash != "" {
		a.toast = res.Flash
		a.toastUntil = time.Now().Add(700 * time.Millisecond)
		return a, nil
	}
	if res.PremoveSet {
		a.setToast("Pre-move set")
		return a, nil
	}
	if res.Move != nil {
		return a.commit(res.Move)
	}
	return a, nil
}

func (a *App) commit(mv *chess.Move) (tea.Model, tea.Cmd) {
	if a.pace <= 0 {
		return a.applyPlayed(mv, false, true)
	}
	a.enqueue(mv, false, true)
	return a, a.armPace()
}

// applyPlayed writes a move onto the board. Human moves and engine replies both
// come through here, so the clock, the cursor, and the next search stay in one
// place. When a pace is set, a queued pre-move waits for the next beat instead
// of appearing in the same frame as the engine move.
//
// committed is a move the player just confirmed. The cursor then sits on that
// destination. Stockfish's reply, the other side in a two-player game when the
// move was not confirmed from the cursor, a pre-move as it plays, and undo do
// not move it.
func (a *App) applyPlayed(mv *chess.Move, premove, committed bool) (tea.Model, tea.Cmd) {
	pre := a.machine.Premove
	mover := a.session.Turn()
	cmd, err := a.session.Play(mv)
	if err != nil {
		a.toast = "Illegal square"
		a.toastUntil = time.Now().Add(700 * time.Millisecond)
		return a, nil
	}
	a.lastApply = time.Now()
	a.placeCursor(mv, mover, premove, committed)
	a.log.Debug("move", "uci", mv.String(), "fen", a.session.Game().FEN())
	var cmds []tea.Cmd
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	if a.session.Over() {
		a.machine.SetOver()
		cmds = append(cmds, a.session.StopClock())
		return a, tea.Batch(cmds...)
	}
	if pre != nil {
		if legal := a.session.FindPremove(pre.From, pre.To, pre.Promo); legal != nil {
			a.machine.SetIdle()
			if a.pace <= 0 {
				_, next := a.applyPlayed(legal, true, false)
				return a, tea.Batch(append(cmds, next)...)
			}
			a.enqueue(legal, true, false)
			cmds = append(cmds, a.armPace())
			return a, tea.Batch(cmds...)
		}
		a.setToast("Pre-move cancelled")
	}
	a.machine.SetIdle()
	if a.session.Mode() == game.ModeComputer && a.session.Turn() != a.session.Human() {
		cmds = append(cmds, a.think())
	}
	return a, tea.Batch(cmds...)
}

// placeCursor follows a move the player just confirmed. Anything else leaves
// the cursor where they put it, and only pulls it back onto the board if it
// has somehow left.
func (a *App) placeCursor(mv *chess.Move, mover chess.Color, premove, committed bool) {
	if a.machine == nil {
		return
	}
	if premove || !committed {
		a.clampCursor()
		return
	}
	if a.session.Mode() == game.ModeComputer {
		if mover != a.session.Human() {
			a.clampCursor()
			return
		}
		a.machine.Cursor = mv.S2()
		return
	}
	// Two-player. The side that moves first keeps the cursor: their own
	// move sits on the destination, and the other side's move puts it back
	// there instead of leaving it on the reply.
	a.left[mover] = mv.S2()
	a.leftOK[mover] = true
	if !a.cursorSideSet {
		a.cursorSide = mover
		a.cursorSideSet = true
	}
	if mover == a.cursorSide {
		a.machine.Cursor = mv.S2()
		return
	}
	if a.leftOK[a.cursorSide] {
		a.machine.Cursor = a.left[a.cursorSide]
		return
	}
	a.clampCursor()
}

func (a *App) clampCursor() {
	if a.machine == nil {
		return
	}
	if a.machine.Cursor > chess.H8 {
		a.machine.Cursor = chess.A1
	}
}

// enqueue holds a move until the pace beat. The first move of a game is due
// immediately. Later moves wait until movePace has passed since the previous
// one was shown, so a fast Stockfish reply does not snap onto the board.
func (a *App) enqueue(mv *chess.Move, premove, committed bool) {
	when := time.Now()
	if a.pace > 0 && !a.lastApply.IsZero() {
		if earliest := a.lastApply.Add(a.pace); when.Before(earliest) {
			when = earliest
		}
	}
	if n := len(a.pending); n > 0 && a.pace > 0 {
		if earliest := a.pending[n-1].when.Add(a.pace); when.Before(earliest) {
			when = earliest
		}
	}
	a.pending = append(a.pending, pacedMove{mv: mv, when: when, premove: premove, committed: committed})
}

func (a *App) armPace() tea.Cmd {
	if len(a.pending) == 0 {
		return nil
	}
	wait := time.Until(a.pending[0].when)
	if wait < 0 {
		wait = 0
	}
	due := a.pending[0].when
	return tea.Tick(wait, func(t time.Time) tea.Msg {
		if t.Before(due) {
			t = due
		}
		return paceMsg{at: t}
	})
}

func (a *App) onPace(now time.Time) (tea.Model, tea.Cmd) {
	if len(a.pending) == 0 {
		return a, nil
	}
	if now.Before(a.pending[0].when) {
		return a, a.armPace()
	}
	mv := a.pending[0].mv
	premove, committed := a.pending[0].premove, a.pending[0].committed
	a.pending = a.pending[1:]
	_, cmd := a.applyPlayed(mv, premove, committed)
	return a, tea.Batch(cmd, a.armPace())
}

func (a *App) undo() (tea.Model, tea.Cmd) {
	if a.session.Over() {
		a.setToast("Undo is disabled after the game ends")
		return a, nil
	}
	a.thinkGen++
	a.pending = nil
	cmd, err := a.session.Undo()
	if err != nil {
		a.setToast(err.Error())
		return a, nil
	}
	a.machine.SetIdle()
	a.clampCursor()
	var cmds []tea.Cmd
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	if a.session.Mode() == game.ModeComputer && a.session.Turn() != a.session.Human() {
		cmds = append(cmds, a.think())
	}
	return a, tea.Batch(cmds...)
}

func (a *App) onDraw() (tea.Model, tea.Cmd) {
	if a.session.Over() {
		return a, nil
	}
	if method, ok := a.session.ClaimableDraw(); ok {
		title := "Claim draw?"
		switch method {
		case chess.ThreefoldRepetition:
			title = "Claim draw by threefold repetition?"
		case chess.FiftyMoveRule:
			title = "Claim draw by the fifty-move rule?"
		}
		return a.ask(confirmDrawClaim, title)
	}
	if a.session.Mode() == game.ModeComputer {
		a.setToast("Stockfish declines the draw")
		return a, nil
	}
	if offer := a.session.DrawOffer(); offer != chess.NoColor {
		if offer != a.session.Turn() {
			return a.ask(confirmDrawAccept, "Accept the draw?")
		}
		a.setToast("Draw already offered")
		return a, nil
	}
	return a.ask(confirmDrawOffer, "Offer a draw?")
}

func (a *App) onTick() (tea.Model, tea.Cmd) {
	a.animate()
	if a.session != nil && !a.session.Over() && !a.session.Clock().Unlimited() && !a.session.Clock().Halted() {
		a.session.Clock().Update(timer.TickMsg{ID: a.session.Clock().ActiveID()})
		if f := a.session.Clock().Flagged(); f != chess.NoColor {
			cmd := a.session.Flag(f)
			if a.machine != nil {
				a.machine.SetOver()
			}
			a.thinkGen++
			a.log.Info("flag", "color", f.Name())
			return a, cmd
		}
	}
	if a.screen == screenPlay && a.session != nil && !a.session.Over() {
		return a, a.tick()
	}
	return a, nil
}

func (a *App) animate() {
	inCheck := a.session != nil && a.session.InCheck() && !a.session.Over()
	if inCheck && !a.wasChk {
		a.cTgt = 1
	}
	if !inCheck {
		a.cTgt = 0
	}
	a.cPos, a.cVel = a.spring.Update(a.cPos, a.cVel, a.cTgt)
	if a.cPos > 0.82 {
		a.cTgt = 0
	}
	a.wasChk = inCheck
}

func (a *App) onEngine(msg engineMsg) (tea.Model, tea.Cmd) {
	if msg.gen != a.thinkGen || a.session == nil || a.session.Over() {
		return a, nil
	}
	if msg.err != nil || msg.move == nil {
		if msg.err != nil {
			a.setToast("Engine error: " + msg.err.Error())
			a.log.Error("engine", "err", msg.err)
		} else {
			a.setToast("Engine returned no move")
		}
		if a.machine.Phase != interact.PhaseOver {
			a.machine.Phase = interact.PhaseEngine
		}
		return a, nil
	}
	if a.pace <= 0 {
		return a.applyPlayed(msg.move, false, false)
	}
	a.enqueue(msg.move, false, false)
	return a, a.armPace()
}

func (a *App) syncKeys() {
	playing := a.screen == screenPlay && a.session != nil
	live := playing && !a.session.Over()
	a.keys.over = playing && a.session.Over()
	a.keys.Select.SetEnabled(live)
	a.keys.Cancel.SetEnabled(live)
	a.keys.Undo.SetEnabled(live)
	a.keys.Resign.SetEnabled(live)
	a.keys.Draw.SetEnabled(live)
	a.keys.Flip.SetEnabled(playing)
	a.keys.Save.SetEnabled(playing)
	a.keys.Open.SetEnabled(true)
	a.keys.New.SetEnabled(true)
	a.keys.Theme.SetEnabled(playing)
	a.keys.Focus.SetEnabled(playing)
}

func (a *App) View() tea.View {
	st := a.styles()
	var body string
	switch a.screen {
	case screenPlay:
		body = a.playView(st)
	default:
		body = a.menuView(st)
	}
	if a.form != nil && a.screen == screenPlay && (a.formKind == formConfirm || a.formKind == formSave || a.formKind == formLoad) {
		box := st.Panel.Render(strings.TrimRight(a.form.View(), "\n"))
		body += "\n" + box
	}
	// An explicit Kitty graphics mode prefixes transmits. The squares themselves
	// stay text pieces. iTerm and Sixel append cursor placements.
	if a.graphics == gfx.Kitty {
		body = a.graphicsSetup() + body
	} else {
		body += a.graphicsSuffix()
	}
	v := tea.NewView(body)
	v.AltScreen = true
	v.WindowTitle = "Termchess"
	return v
}

func (a *App) menuView(st theme.Styles) string {
	var b strings.Builder
	b.WriteString(st.Title.Render("Termchess"))
	b.WriteString("\n")
	b.WriteString(st.Muted.Render("Keyboard chess"))
	b.WriteString("\n\n")
	if a.form != nil {
		b.WriteString(a.form.View())
	}
	if a.toast != "" && time.Now().Before(a.toastUntil) {
		b.WriteString("\n")
		b.WriteString(st.Accent.Render(a.toast))
	}
	b.WriteString("\n")
	b.WriteString(st.Muted.Render("q quit"))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (a *App) playView(st theme.Styles) string {
	if a.session == nil || a.machine == nil {
		return "No game."
	}
	mono := st.Mono || a.colorless
	cw, ch, side, tooSmall := layout(a.width, a.height)
	if tooSmall {
		return st.Title.Render("please enlarge the terminal") + "\n\n" +
			st.Text.Render("Each square is 8×4 cells. Make the window larger to play.")
	}
	illegal := a.toast == "Illegal square" && time.Now().Before(a.toastUntil)
	ctx := cellCtx{
		session:  a.session,
		machine:  a.machine,
		styles:   st,
		checkHot: a.cPos > 0.45,
		illegal:  illegal,
		mono:     mono,
		pieces:   a.pieces,
		detail:   a.blocks,
		profile:  a.profile,
	}
	flipped := a.session.BoardFlipped()
	board := renderBoard(ctx, cw, ch, flipped)
	whiteName, blackName := a.session.Names()
	topColor, botColor := chess.Black, chess.White
	topName, botName := blackName, whiteName
	if flipped {
		topColor, botColor = chess.White, chess.Black
		topName, botName = whiteName, blackName
	}
	active := a.session.Turn()
	if a.session.Over() {
		active = chess.NoColor
	}
	clk := a.session.Clock()
	topClock := clockLine(topName, clk.View(topColor), active == topColor, st)
	botClock := clockLine(botName, clk.View(botColor), active == botColor, st)
	if a.machine.Thinking() && active == topColor {
		topClock += "  " + st.Accent.Render("thinking…")
	}
	if a.machine.Thinking() && active == botColor {
		botClock += "  " + st.Accent.Render("thinking…")
	}
	boardBlock := topClock + "\n" + board + "\n" + botClock

	a.refreshHistory(side)
	panel := a.panel(st, side)
	var main string
	if side {
		main = lipgloss.JoinHorizontal(lipgloss.Top, boardBlock, "  ", panel)
	} else {
		main = boardBlock + "\n" + panel
	}

	status := a.session.StatusLine(a.machine.Thinking(), a.machine.Premove != nil)
	if a.toast != "" && time.Now().Before(a.toastUntil) && a.toast != status {
		status += "   " + a.toast
	}
	header := st.Title.Render("Termchess") + st.Muted.Render("  ·  "+st.Name+"  ·  "+squareName(cw, ch))
	if a.graphics != gfx.Off && a.form == nil {
		header += st.Muted.Render("  ·  " + a.graphics.String())
	}
	if a.focusHist {
		header += st.Accent.Render("  ·  history")
	}

	var extra string
	if a.machine.Phase == interact.PhasePromotion || a.machine.Phase == interact.PhasePremovePromo {
		extra = "\n" + promotionLine(a.machine, st)
	}

	a.syncKeys()
	a.help.ShowAll = a.fullHelp
	a.help.SetWidth(max(20, a.width-2))
	a.help.Styles.ShortKey = st.HelpKey
	a.help.Styles.ShortDesc = st.HelpDesc
	a.help.Styles.ShortSeparator = st.Muted
	a.help.Styles.FullKey = st.HelpKey
	a.help.Styles.FullDesc = st.HelpDesc
	a.help.Styles.FullSeparator = st.Muted
	helpView := a.help.View(a.keys)

	return header + "\n" + main + extra + "\n" + st.Status.Render(status) + "\n" + helpView
}

func (a *App) refreshHistory(side bool) {
	text := game.SANHistory(a.session.Game())
	if text != a.histText {
		a.histText = text
		a.history.SetContent(text)
		if n := a.session.MoveCount(); n != a.histN {
			a.history.GotoBottom()
			a.histN = n
		}
	}
	w := 30
	if !side {
		w = max(20, a.width-4)
	}
	h := 10
	if !side {
		h = 6
	}
	a.history.SetWidth(w)
	a.history.SetHeight(h)
}

func (a *App) panel(st theme.Styles, side bool) string {
	w := 30
	if !side {
		w = max(20, a.width-4)
	}
	whiteCap, blackCap, diff := game.Captured(a.session.Game())
	whiteAhead, blackAhead := 0, 0
	if diff > 0 {
		whiteAhead = diff
	} else if diff < 0 {
		blackAhead = -diff
	}
	body := st.Accent.Render("Moves") + "\n" + a.history.View() + "\n\n" +
		st.Accent.Render("Captured") + "\n" +
		st.Text.Render("White  ") + renderCaptured(whiteCap, whiteAhead, st) + "\n" +
		st.Text.Render("Black  ") + renderCaptured(blackCap, blackAhead, st)
	return st.Panel.Width(w + 2).Render(body)
}

func renderCaptured(ps []chess.Piece, ahead int, st theme.Styles) string {
	// A hyphen, not an em dash. xfce draws U+2014 as a heavy block.
	body := st.Muted.Render("-")
	if len(ps) > 0 {
		var b strings.Builder
		for _, p := range ps {
			if st.Mono || st.WhitePiece == nil {
				b.WriteString(p.String())
				continue
			}
			b.WriteString(pieceChip(p, st).Render(solidGlyph(p)))
		}
		body = b.String()
	}
	if ahead > 0 {
		return body + fmt.Sprintf("  +%d", ahead)
	}
	return body
}

// graphicsSetup emits Kitty transmits and virtual placements that this
// session has not sent yet. It is empty on every later redraw. The board
// cells stay the block pieces either way.
func (a *App) graphicsSetup() string {
	if a.graphics != gfx.Kitty || !a.kittyBoard() {
		return ""
	}
	cw, ch, _, tooSmall := layout(a.width, a.height)
	if tooSmall {
		return ""
	}
	frame, sent := gfx.Frame(gfx.Kitty, a.pieceSpots(cw, ch, a.session.BoardFlipped()), a.gfxSent)
	a.gfxSent = sent
	return frame
}

// graphicsSuffix places iTerm and Sixel images. Sequences are zero-width.
// Kitty does not use this path.
func (a *App) graphicsSuffix() string {
	if a.graphics == gfx.Off || a.graphics == gfx.Kitty {
		return ""
	}
	if !a.boardImages() {
		return ""
	}
	cw, ch, _, tooSmall := layout(a.width, a.height)
	if tooSmall {
		return ""
	}
	frame, sent := gfx.Frame(a.graphics, a.pieceSpots(cw, ch, a.session.BoardFlipped()), a.gfxSent)
	a.gfxSent = sent
	return frame
}

func (a *App) boardImages() bool {
	return a.graphics != gfx.Off && a.screen == screenPlay && a.form == nil && a.session != nil && a.machine != nil
}

// kittyBoard stays on while a confirm dialog is open. The board is still on
// screen, and the placeholders have to stay in it or the pieces vanish.
func (a *App) kittyBoard() bool {
	return a.graphics == gfx.Kitty && a.screen == screenPlay && a.session != nil && a.machine != nil
}

func (a *App) pieceSpots(cellW, cellH int, flipped bool) []gfx.Spot {
	var spots []gfx.Spot
	for sq := chess.A1; sq <= chess.H8; sq++ {
		p := a.session.Piece(sq)
		if p == chess.NoPiece {
			continue
		}
		png, err := pieces.EmbeddedPNG(p.Color(), p.Type())
		if err != nil || len(png) == 0 {
			continue
		}
		id := pieces.ID(p.Color(), p.Type())
		if id == 0 {
			continue
		}
		row, col := coord.Visual(sq, flipped)
		spots = append(spots, gfx.Spot{
			Row:  4 + row*cellH,
			Col:  4 + col*cellW,
			Cols: cellW,
			Rows: cellH,
			ID:   id,
			PNG:  png,
		})
	}
	return spots
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
