// Package ui is the Bubble Tea application.
//
// The root model is a small stack of screens: menu, setup, and the board.
// Confirm, save, and load are Huh forms layered on the current screen.
// Board input is the state machine in internal/interact. Nothing in Update
// blocks: engine searches, directory listings, and file writes are commands.
package ui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/timer"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/harmonica"
	"github.com/notnil/chess"
	"termchess/internal/clock"
	"termchess/internal/config"
	"termchess/internal/engine"
	"termchess/internal/game"
	"termchess/internal/gfx"
	"termchess/internal/interact"
	"termchess/internal/store"
	"termchess/internal/theme"
)

type screen int

const (
	screenMenu screen = iota
	screenSetup
	screenPlay
)

type formKind int

const (
	formNone formKind = iota
	formMenu
	formSetup
	formLoad
	formSave
	formConfirm
)

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmQuit
	confirmNew
	confirmResign
	confirmDrawClaim
	confirmDrawOffer
	confirmDrawAccept
	confirmLoad
)

// Options configures the program. Tests inject a mock engine and a temp directory.
type Options struct {
	StockfishPath string
	EngineOK      bool
	Engine        engine.Engine
	Config        config.Config
	ConfigPath    string
	DataDir       string
	Logger        *log.Logger
	ColorMode     ColorMode
	// Colorless is set when the resolved profile cannot draw square colours,
	// including --color none and an auto profile that landed on ASCII.
	Colorless bool
	// Graphics is the resolved image protocol. The zero value draws no images.
	Graphics gfx.Mode
	// Blocks is the text-art glyph set. The zero value is quadrants.
	Blocks BlockDetail
	// Pieces is classic, silhouette, or set1..set5. The zero value is classic.
	Pieces PieceStyle
	// Profile is the colour depth board cells should emit. The zero value
	// is 24-bit. main sets this from --color and the terminal environment.
	Profile colorprofile.Profile
}

// App is the root model.
type App struct {
	screen screen
	width  int
	height int
	dark   bool

	keys     KeyMap
	help     help.Model
	fullHelp bool

	cfg     config.Config
	cfgPath string
	dataDir string
	themeID theme.ID

	form        *huh.Form
	formKind    formKind
	confirmKind confirmKind
	setupMode   game.Mode

	choice         string
	autoFlipChoice string
	tcID           string
	minutes        string
	incSec         string
	colorChoice    string
	eloChoice      string
	filename       string
	loadChoice     string
	confirmed      bool

	session *game.Session
	machine *interact.Machine
	// cursorSide is the side whose square the cursor returns to after the
	// other side moves. In a two-player game that is the side that moved
	// first. Against the computer it is the human, handled separately.
	cursorSide    chess.Color
	cursorSideSet bool
	left          [3]chess.Square
	leftOK        [3]bool
	history       viewport.Model
	histText      string
	histN         int
	focusHist     bool

	toast      string
	toastUntil time.Time

	thinkGen   int
	eng        engine.Engine
	enginePath string
	engineOK   bool

	log       *log.Logger
	colorMode ColorMode
	colorless bool
	profile   colorprofile.Profile
	graphics  gfx.Mode
	blocks    BlockDetail
	pieces    PieceStyle
	// gfxSent is the set of Kitty image ids already uploaded this session.
	gfxSent map[int]struct{}
	// pace is the minimum gap between showing two moves. Zero applies at once.
	pace      time.Duration
	lastApply time.Time
	pending   []pacedMove
	spring    harmonica.Spring
	cPos      float64
	cVel      float64
	cTgt      float64
	wasChk    bool
}

// clampGraphics keeps only an explicit protocol. Any other value, including
// a zero mode, draws text. The caller decides the protocol from --graphics;
// this refuses a stray value.
func clampGraphics(m gfx.Mode) gfx.Mode {
	switch m {
	case gfx.Kitty, gfx.ITerm, gfx.Sixel:
		return m
	default:
		return gfx.Off
	}
}

// New builds the app on the start menu.
func New(opt Options) *App {
	if opt.Logger == nil {
		opt.Logger = log.New(io.Discard)
	}
	if opt.ConfigPath == "" {
		opt.ConfigPath = config.ConfigPath()
	}
	if opt.DataDir == "" {
		opt.DataDir = config.GamesDir()
	}
	cfg := opt.Config
	if cfg.Theme == "" {
		cfg = config.Default()
	}
	a := &App{
		width:      80,
		height:     32,
		dark:       true,
		keys:       newKeyMap(),
		help:       help.New(),
		cfg:        cfg,
		cfgPath:    opt.ConfigPath,
		dataDir:    opt.DataDir,
		themeID:    theme.Parse(cfg.Theme),
		eng:        opt.Engine,
		enginePath: opt.StockfishPath,
		engineOK:   opt.EngineOK || opt.Engine != nil || opt.StockfishPath != "",
		log:        opt.Logger,
		colorMode:  opt.ColorMode,
		colorless:  opt.Colorless || opt.ColorMode == ColorNone,
		profile:    opt.Profile,
		graphics:   clampGraphics(opt.Graphics),
		blocks:     opt.Blocks,
		pieces:     opt.Pieces,
		pace:       time.Duration(cfg.MovePaceMs) * time.Millisecond,
		spring:     harmonica.NewSpring(harmonica.FPS(10), 6.5, 0.42),
		history:    viewport.New(viewport.WithWidth(28), viewport.WithHeight(12)),
	}
	a.openMenu()
	return a
}

func (a *App) Init() tea.Cmd {
	if a.form != nil {
		return a.form.Init()
	}
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		if a.form != nil {
			return a.passForm(msg)
		}
		return a, nil
	case tea.BackgroundColorMsg:
		a.dark = msg.IsDark()
		if a.form != nil {
			return a.passForm(msg)
		}
		return a, nil
	case clockTickMsg:
		return a.onTick()
	case paceMsg:
		return a.onPace(msg.at)
	case timer.StartStopMsg:
		if a.session != nil {
			a.session.Clock().Update(msg)
		}
		return a, nil
	case engineMsg:
		return a.onEngine(msg)
	case listMsg:
		return a.onList(msg)
	case fileMsg:
		return a.onFile(msg)
	case wroteMsg:
		return a.onWrote(msg)
	case configSavedMsg:
		if msg.err != nil {
			a.setToast("Could not save settings")
			a.log.Error("config", "err", msg.err)
		}
		return a, nil
	case tea.KeyPressMsg:
		return a.onKey(msg)
	default:
		if a.form != nil {
			return a.passForm(msg)
		}
	}
	return a, nil
}

func (a *App) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.form != nil {
		switch msg.String() {
		case "esc":
			return a.onEsc()
		case "ctrl+c":
			return a.requestQuit()
		case "q":
			if a.formKind != formSave && a.formKind != formSetup {
				return a.requestQuit()
			}
		}
		return a.passForm(msg)
	}
	switch {
	case key.Matches(msg, a.keys.Quit):
		return a.requestQuit()
	case a.screen == screenPlay:
		return a.onPlayKey(msg)
	case key.Matches(msg, a.keys.Help):
		a.fullHelp = !a.fullHelp
		return a, nil
	}
	return a, nil
}

func (a *App) requestQuit() (tea.Model, tea.Cmd) {
	if a.screen == screenPlay && a.session != nil && !a.session.Over() {
		return a.ask(confirmQuit, "Quit this game?")
	}
	return a, tea.Quit
}

func (a *App) onEsc() (tea.Model, tea.Cmd) {
	switch a.formKind {
	case formConfirm, formSave:
		a.clearForm()
		return a, nil
	case formLoad:
		a.clearForm()
		if a.screen != screenPlay {
			return a.openMenu()
		}
		return a, nil
	case formSetup:
		return a.openMenu()
	case formMenu:
		return a, tea.Quit
	default:
		if a.machine != nil {
			a.machine.Cancel()
		}
		return a, nil
	}
}

func (a *App) passForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.form == nil {
		return a, nil
	}
	updated, cmd := a.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		a.form = f
	}
	if a.form.State == huh.StateCompleted {
		return a.finishForm()
	}
	if a.form.State == huh.StateAborted {
		if a.formKind == formMenu {
			return a, tea.Quit
		}
		kind := a.formKind
		a.clearForm()
		if kind != formSave && a.screen != screenPlay {
			return a.openMenu()
		}
		return a, nil
	}
	return a, cmd
}

func (a *App) finishForm() (tea.Model, tea.Cmd) {
	kind := a.formKind
	a.form = nil
	a.formKind = formNone
	switch kind {
	case formMenu:
		switch a.choice {
		case "computer":
			if !a.engineOK {
				a.setToast("Stockfish was not found — two-player is still available")
				return a.openMenu()
			}
			return a.openSetup(game.ModeComputer)
		case "load":
			return a, a.listCmd()
		default:
			return a.openSetup(game.ModeLocal)
		}
	case formSetup:
		return a.beginFromForm()
	case formSave:
		return a, a.writeCmd()
	case formLoad:
		if a.loadChoice == "" || a.loadChoice == "back" {
			if a.screen == screenPlay {
				return a, nil
			}
			return a.openMenu()
		}
		return a, a.readCmd(a.loadChoice)
	case formConfirm:
		ck := a.confirmKind
		yes := a.confirmed
		a.confirmKind = confirmNone
		if !yes {
			return a, nil
		}
		return a.applyConfirm(ck)
	default:
		return a, nil
	}
}

func (a *App) applyConfirm(kind confirmKind) (tea.Model, tea.Cmd) {
	switch kind {
	case confirmQuit:
		return a, tea.Quit
	case confirmNew:
		return a.openMenu()
	case confirmResign:
		if a.session == nil || a.session.Over() {
			return a, nil
		}
		color := a.session.Turn()
		if a.session.Mode() == game.ModeComputer {
			color = a.session.Human()
		}
		cmd := a.session.Resign(color)
		a.machine.SetOver()
		a.thinkGen++
		a.log.Info("resign", "color", color.Name())
		return a, cmd
	case confirmDrawClaim:
		if err := a.session.ClaimDraw(); err != nil {
			a.setToast(err.Error())
			return a, nil
		}
		a.machine.SetOver()
		a.thinkGen++
		return a, a.session.StopClock()
	case confirmDrawOffer:
		a.session.OfferDraw(a.session.Turn())
		a.setToast("Draw offered — opponent presses d to answer")
		return a, nil
	case confirmDrawAccept:
		if err := a.session.AcceptDraw(); err != nil {
			a.setToast(err.Error())
			return a, nil
		}
		a.machine.SetOver()
		a.thinkGen++
		return a, a.session.StopClock()
	case confirmLoad:
		return a, a.listCmd()
	default:
		return a, nil
	}
}

func (a *App) openMenu() (tea.Model, tea.Cmd) {
	a.screen = screenMenu
	a.session = nil
	a.machine = nil
	a.focusHist = false
	a.form = a.menuForm()
	a.formKind = formMenu
	a.confirmKind = confirmNone
	return a, a.form.Init()
}

func (a *App) openSetup(mode game.Mode) (tea.Model, tea.Cmd) {
	a.screen = screenSetup
	a.setupMode = mode
	a.form = a.setupForm(mode)
	a.formKind = formSetup
	return a, a.form.Init()
}

func (a *App) ask(kind confirmKind, title string) (tea.Model, tea.Cmd) {
	a.confirmKind = kind
	a.form = a.confirmForm(title)
	a.formKind = formConfirm
	return a, a.form.Init()
}

func (a *App) clearForm() {
	a.form = nil
	a.formKind = formNone
	a.confirmKind = confirmNone
}

func (a *App) beginFromForm() (tea.Model, tea.Cmd) {
	tc := a.timeControl()
	mode := a.setupMode
	human := chess.NoColor
	white, black := "White", "Black"
	elo := 0
	auto := a.autoFlipChoice == "yes"
	if mode == game.ModeComputer {
		if err := a.ensureEngine(); err != nil {
			a.setToast(err.Error())
			a.engineOK = false
			return a.openMenu()
		}
		human = a.humanColor()
		elo = engine.Nearest(atoi(a.eloChoice))
		name := engine.OpponentName(elo)
		if human == chess.Black {
			white, black = name, "You"
		} else {
			white, black = "You", name
		}
		auto = false
	}
	s, err := game.New(game.Options{
		Mode:      mode,
		Human:     human,
		WhiteName: white,
		BlackName: black,
		TC:        tc,
		AutoFlip:  auto,
		Elo:       elo,
	})
	if err != nil {
		a.setToast(err.Error())
		return a.openMenu()
	}
	a.cfg.Theme = string(a.themeID)
	a.cfg.Last.Mode = mode.String()
	a.cfg.Last.AutoFlip = auto
	a.cfg.Last.HumanColor = a.colorChoice
	a.cfg.Last.Elo = elo
	a.cfg.Last.TimeControl = a.tcID
	a.cfg.Last.CustomMinutes = atoi(a.minutes)
	a.cfg.Last.CustomIncrement = atoi(a.incSec)
	a.log.Info("new game", "mode", mode.String(), "tc", tc.PGNString(), "human", human.String(), "elo", elo)
	model, cmd := a.useSession(s)
	return model, tea.Batch(cmd, a.saveCfg())
}

// useSession installs s as the live game. Tests call it directly.
func (a *App) useSession(s *game.Session) (tea.Model, tea.Cmd) {
	a.session = s
	a.screen = screenPlay
	a.clearForm()
	a.focusHist = false
	a.histText = ""
	a.histN = -1
	cur := chess.E2
	if s.BoardFlipped() {
		cur = chess.E7
	}
	if from, to, ok := s.LastMove(); ok {
		_ = from
		cur = to
	}
	a.machine = interact.New(cur, s.Human())
	if s.Over() {
		a.machine.SetOver()
		return a, nil
	}
	var cmds []tea.Cmd
	cmds = append(cmds, s.ArmClock(), a.tick())
	if s.Mode() == game.ModeComputer && s.Turn() != s.Human() {
		cmds = append(cmds, a.think())
	}
	return a, tea.Batch(cmds...)
}

func (a *App) ensureEngine() error {
	if a.eng != nil {
		return nil
	}
	if a.enginePath == "" {
		return fmt.Errorf("stockfish is not available")
	}
	e, err := engine.NewStockfish(a.enginePath)
	if err != nil {
		return err
	}
	a.eng = e
	a.engineOK = true
	return nil
}

// Shutdown closes the engine process. Call it after the program exits.
func (a *App) Shutdown() {
	if a.eng != nil {
		_ = a.eng.Close()
	}
}

func (a *App) setToast(s string) {
	a.toast = s
	a.toastUntil = time.Now().Add(1600 * time.Millisecond)
}

func (a *App) styles() theme.Styles { return theme.For(a.themeID, a.dark) }

func atoi(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

type clockTickMsg struct{}

type paceMsg struct{ at time.Time }

type pacedMove struct {
	mv        *chess.Move
	when      time.Time
	premove   bool
	committed bool
}

func (a *App) tick() tea.Cmd {
	return tea.Tick(clock.Interval, func(time.Time) tea.Msg { return clockTickMsg{} })
}

type engineMsg struct {
	gen  int
	move *chess.Move
	err  error
}

type listMsg struct {
	entries []store.Entry
	err     error
}

type fileMsg struct {
	name string
	data string
	err  error
}

type wroteMsg struct {
	path string
	err  error
}

func (a *App) think() tea.Cmd {
	if a.session == nil || a.session.Over() {
		return nil
	}
	if err := a.ensureEngine(); err != nil {
		a.setToast(err.Error())
		return nil
	}
	a.machine.BeginEngine()
	a.thinkGen++
	gen := a.thinkGen
	pos := a.session.Game().Position()
	elo := a.session.Elo()
	clk := a.session.Clock()
	side := a.session.Turn()
	// The engine shortens this further when the rating's own movetime is lower.
	budget := engine.ThinkBudget(engine.SearchCeiling, clk.Remaining(side), clk.Increment(), clk.Unlimited())
	eng := a.eng
	a.log.Debug("think", "budget", budget.String(), "elo", elo, "gen", gen)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), budget+3*time.Second)
		defer cancel()
		mv, err := eng.BestMove(ctx, engine.Request{Position: pos, Elo: elo, Budget: budget})
		return engineMsg{gen: gen, move: mv, err: err}
	}
}

func (a *App) listCmd() tea.Cmd {
	dir := a.dataDir
	return func() tea.Msg {
		entries, err := store.List(dir)
		return listMsg{entries: entries, err: err}
	}
}

func (a *App) readCmd(name string) tea.Cmd {
	dir := a.dataDir
	return func() tea.Msg {
		data, err := store.Read(dir, name)
		return fileMsg{name: name, data: data, err: err}
	}
}

func (a *App) writeCmd() tea.Cmd {
	if a.session == nil {
		return nil
	}
	dir := a.dataDir
	name := a.filename
	pgn := a.session.PGN()
	return func() tea.Msg {
		path, err := store.Save(dir, name, pgn)
		return wroteMsg{path: path, err: err}
	}
}

func (a *App) saveCfg() tea.Cmd {
	path := a.cfgPath
	cfg := a.cfg
	return func() tea.Msg {
		err := config.Save(path, cfg)
		return configSavedMsg{err}
	}
}

type configSavedMsg struct{ err error }

func (a *App) onList(msg listMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.setToast(msg.err.Error())
		if a.screen != screenPlay {
			return a.openMenu()
		}
		return a, nil
	}
	a.form = a.loadForm(msg.entries)
	a.formKind = formLoad
	return a, a.form.Init()
}

func (a *App) onFile(msg fileMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.setToast(msg.err.Error())
		return a, nil
	}
	s, err := game.LoadPGN(msg.data)
	if err != nil {
		a.setToast(err.Error())
		return a, nil
	}
	if s.Mode() == game.ModeComputer {
		if err := a.ensureEngine(); err != nil {
			s.DemoteToLocal()
			a.setToast("Stockfish missing — loaded as two-player")
		}
	}
	a.log.Info("loaded", "file", msg.name, "plies", s.MoveCount())
	return a.useSession(s)
}

func (a *App) onWrote(msg wroteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.setToast(msg.err.Error())
		a.log.Error("save", "err", msg.err)
		return a, nil
	}
	if a.session != nil {
		a.session.MarkSaved()
	}
	a.setToast("Saved " + msg.path)
	a.log.Info("saved", "path", msg.path)
	return a, nil
}
