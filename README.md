# Termchess

A keyboard-only chess game for the terminal. The cursor moves with the arrow keys or `hjkl`. Enter or Space selects a piece and plays a legal move. There is no typed move input.

Rules come from [github.com/notnil/chess](https://github.com/notnil/chess): legality, check, checkmate, stalemate, castling, en passant, promotion, threefold repetition, the fifty-move rule, and insufficient material. The computer opponent is [Hypersion](https://github.com/RenCopp/Hypersion) over UCI. If the binary is missing, two-player mode and saved games still work.

## Install

Go 1.26 or newer is required (Bubble Tea v2).

```sh
git clone https://github.com/CurtisD4q/termchess termchess
cd termchess
make build
./bin/termchess
```

`make run` is the same thing via `go run ./cmd/termchess`.

Colour follows the terminal. `--color` is `auto` (default), `truecolor`, `256`, `16`, or `none`. `auto` uses true colour when `COLORTERM` is `truecolor` or `24bit`, and 256 colours for `TERM=xterm-256color` (what xfce4-terminal reports). Square colours and the cursor are backgrounds, so a colorless profile draws the cursor as brackets instead, for example `[♟]`. If the board stays black, start with `--color 256`. `NO_COLOR` forces the colorless fallback.

The board always draws the block pieces. `--pieces` is `classic` (default), `silhouette`, or an alternate classic set `set1` through `set5`. The alternate sets use the same 8×4 grid, colours, and board; `classic` stays the default. `--graphics` is off unless you pass `kitty`, `iterm`, or `sixel`. Omitting the flag leaves images off. `--graphics off` forces text. Nothing else selects a protocol: not `TERM`, not `KITTY_WINDOW_ID`, `KITTY_PID`, or `KITTY_PUBLIC_KEY`, not a terminal query, and not `graphics` in `config.json`. There is no `auto` value. The text pieces stay in every square either way. `--debug-dump` prints one frame of the starting position and exits.

### Hypersion

The computer is [Hypersion](https://github.com/RenCopp/Hypersion), a GPL-3.0 UCI engine. Termchess starts it as a separate program and does not link it, so Hypersion's license does not cover this project. It looks for `hypersion` or `Hypersion` on `PATH`, then `/usr/local/bin`, `/usr/bin`, Homebrew, and `C:\Program Files\Hypersion\Hypersion.exe`. Pass an explicit binary with `--engine`.

Hypersion needs two NNUE files next to the binary. They are not in the Hypersion git repository. They are the Stockfish 18 networks, also GPL-3.0:

- `nn-c288c895ea92.nnue` (big, about 104 MB)
- `nn-37f18f62d772.nnue` (small, about 3 MB)

```sh
git clone https://github.com/RenCopp/Hypersion.git
cd Hypersion
make build
curl -L -O https://tests.stockfishchess.org/api/nn/nn-c288c895ea92.nnue
curl -L -O https://tests.stockfishchess.org/api/nn/nn-37f18f62d772.nnue
```

`make build` needs `g++` 12 or newer and GNU make. The default target is AVX2. `make build ARCH=x86-64-avxvnni` is the faster build on recent Intel and AMD CPUs. The binary is `./Hypersion`. Put both `.nnue` files in that same directory. Termchess passes those paths as `EvalFile` and `EvalFileSmall`, so you do not have to start the game from that directory.

Without the networks Hypersion still runs, on its classical evaluator, and plays much weaker than the rating you picked.

```sh
termchess --engine /path/to/Hypersion
```

On Windows the same two files sit beside `Hypersion.exe`. Download them with:

```powershell
Invoke-WebRequest -Uri "https://tests.stockfishchess.org/api/nn/nn-c288c895ea92.nnue" -OutFile "nn-c288c895ea92.nnue"
Invoke-WebRequest -Uri "https://tests.stockfishchess.org/api/nn/nn-37f18f62d772.nnue" -OutFile "nn-37f18f62d772.nnue"
```

## Play

The start menu is a form:

1. **Local two-player** — pass-and-play, with an option to flip the board each turn.
2. **Play vs computer** — choose White, Black, or random, and an engine rating from 500 to 2500. The list has 52 steps. Arrow keys or `j`/`k` move one step, `Ctrl+U` / `Ctrl+D` page, and `/` filters (type `1650` to jump). The clock and the status line show `Hypersion (1650)`. That rating is saved with the rest of the setup and written into the PGN (`Elo`, plus `WhiteElo` or `BlackElo`).
3. **Load a saved game** — resume a PGN from the games folder.

Every new game asks for a time control: none, bullet 1+0, blitz 3+2 or 5+0, rapid 10+0 or 15+10, or custom minutes plus increment. The active clock is highlighted and ticks. A flag is a loss, or a draw when the opponent has no mating material.

Moves are shown on a separate, fixed cadence so a human move and a Hypersion reply land with the same rhythm. The gap is `movePaceMs` in `config.json` (default 400, `0` shows a move the moment it is ready, maximum 5000). The first move of a game is drawn on the next beat. After that, a move waits until the gap has elapsed since the previous one was drawn, and a queued pre-move waits one more beat instead of appearing in the same frame. A player who has already thought longer than the gap sees no extra lag. The chess clock is not this cadence: it keeps ticking every 100ms during the hold, and the increment is applied when the move is actually played.

The cursor stops at the edge of the board. Movement is screen-relative, so after a flip, Up still moves toward the top of the screen. After you confirm a move it sits on that move's destination. Hypersion's reply leaves it on the square where you left it, and so do a pre-move when it plays and undo. In a two-player game the side that moves first keeps the cursor: after the other side plays, it returns to the square that first side left, rather than staying on the reply. It is only brought back onto the board if it has left the board.

While Hypersion is thinking the board is locked, and the status line shows `thinking…`. You may queue one pre-move. It is played if it is still legal, otherwise it is cleared.

Promotion opens an overlay (`Q R B N`). Left and Right change the piece, Enter confirms, Esc cancels the move and keeps the selection.

### Controls

| Key | Action |
| --- | --- |
| `↑↓←→` or `hjkl` | Move the cursor (or scroll the move list when it is focused). Your move leaves it on the destination; an opponent reply, a pre-move, and undo do not take it to another square |
| `Enter` / `Space` | Select a piece, or play the move on a highlighted square |
| `Esc` | Clear the selection, or cancel promotion |
| `u` | Undo one ply locally, or a full move pair against the computer |
| `f` | Flip the board |
| `t` | Cycle theme |
| `s` | Save PGN |
| `o` | Open a saved game |
| `n` | New game |
| `r` | Resign. The hint bar shows this during a live game. It asks `Resign? y/n`. In a local game the side to move resigns; against the computer, you resign. Nothing happens once the game is over |
| `d` | Offer a draw, accept one, or claim threefold / fifty-move |
| `Tab` | Focus the board or the move history |
| `?` | Toggle the full help list |
| `q` / `Ctrl+C` | Quit |

New game, resign, quit mid-game, and loading over an unsaved game ask for confirmation. `y` accepts, `n` or `Esc` declines. A resignation ends the game `0-1` or `1-0`, the status line names who resigned, and the saved PGN records `Result` and `Termination` (for example `White resigns`). The same end-of-game actions as any other finished game are then available.

The short help bar is always visible during a game. `?` expands it.

### Board

Every square is 8×4 cells, a step down from the old 9×4 large size. There is no square-size setting. An old `squareSize` value in `config.json` is ignored. A window that cannot hold the board says `please enlarge the terminal`.

Squares draw the classic block pictures by default. Every square is the same 8×4 tile. Light squares are `#B08A64` and dark squares are `#6B4F3A`. File letters a–h sit on the top and bottom of a dark `#2A2018` border, and rank numbers 1–8 sit on both sides of that border. A piece is solid quadrant blocks (`█`, `▀`, `▄`, and the corner blocks) in one colour, with no outline: white is `#FFFFFF` and black is `#000000`, painted as the foreground on the square. The cursor square is solid `#F2F27A`, and the piece stays on top of it. The king has a cross, the queen a five-point crown, the rook a crenellated top wider than its body with a matching base, the bishop a small cross, the knight a horse head, and the pawn a rounded square on a thick neck. The same glyphs are in the fonts xfce4-terminal and Kitty ship, so the pieces stay visible in both. `--pieces silhouette` switches to the other hand-tuned set at the same size. `--pieces set1` through `set5` switch among five other classic block sets on this same grid. `--blocks` applies to silhouette mode. `auto` uses quadrants on Kitty and on a plain xterm such as xfce4-terminal, and still paints with those ordinary block elements. An empty target is a centred `·`. A capture is a centred `×`. Highlights change only the tile background.

The drawings are original (MIT, `assets/pieces/LICENSE`). A contact sheet is `assets/pieces/preview.png`.

The high-contrast theme and `--color none` draw classic pieces as the same block picture, white in `▒` and black in the solid blocks, and the cursor as brackets around that row. Silhouette mode draws white as the dark rim and black as the filled shape. A 16-colour terminal uses the same pictures; the palette is downsampled.

```
   a  b  c  d  e  f  g  h
8  ♜  ♞  ♝  ♛  ♚  ♝  ♞  ♜  8
7  ♟  ♟  ♟  ♟  ♟  ♟  ♟  ♟  7
6                          6
5                          5
4              ·           4
3              ·           3
2  ♙  ♙  ♙  ♙ [♙] ♙  ♙  ♙  2
1  ♖  ♘  ♗  ♕  ♔  ♗  ♘  ♖  1
   a  b  c  d  e  f  g  h
```

High-contrast theme, White's e-pawn selected. In a real terminal each cell is padded to the same display width, so wide chess glyphs stay aligned. This plain-text copy looks slightly off where the brackets add columns.

Themes: classic wood, green tournament, blue, and high contrast. `t` cycles them. Colours are drawn in the palette selected by `--color`, so a 256-colour terminal receives 256-colour sequences and a true-colour terminal receives 24-bit sequences.

### Save and load

`s` asks for a file name (default `termchess-YYYYMMDD-HHMMSS.pgn`) and writes it under the OS data directory:

| OS | Games folder |
| --- | --- |
| Linux | `~/.local/share/termchess/games` (`XDG_DATA_HOME` if set) |
| macOS | `~/Library/Application Support/termchess/games` |
| Windows | `%LOCALAPPDATA%\termchess\games` |

Headers include Event, Date, White, Black, Result, and TimeControl, plus the clocks and the mode so a loaded game can resume. Settings (theme, graphics protocol, move pace, last setup) live in the user config directory as `termchess/config.json`. An old `squareSize` key is ignored. A `graphics` value in that file does not turn images on. Pass `--graphics kitty`, `--graphics iterm`, or `--graphics sixel`. `--graphics off` forces text. `movePaceMs` is the gap described above. Debug logs go to `termchess.log` in the data directory, never to the screen while the game is running.

A demo recording is `demo.tape` (Charm VHS). This environment has no VHS or ffmpeg, so `demo.gif` is not checked in. Record one with:

```sh
go install github.com/charmbracelet/vhs@latest
vhs demo.tape
```

## Engine rating

The picker is exactly these 52 ratings:

| Range | Step |
| --- | --- |
| 500, 600, …, 1000 | 100 |
| 1050, 1100, …, 1700 | 50 |
| 1725, 1750, …, 2500 | 25 |

On the first search the engine's `uci` handshake is read. If it advertises `UCI_LimitStrength` and `UCI_Elo`, the minimum and maximum come from that option. Hypersion reports 500–3300, so every rating on this ladder is inside the range. Nothing in the mapping hardcodes those numbers.

- A rating inside the engine's range sets `UCI_LimitStrength` and `UCI_Elo` to that rating. When the reported range covers 500 through 2500, that is the whole ladder: no Skill Level, depth, or node cap. Think time grows from 400ms toward 2s as the rating rises, then the clock trims it.
- A rating above the engine maximum still uses `UCI_Elo`, clamped to the maximum. Think time keeps growing with the requested rating.
- A rating below the minimum is not sent as `UCI_Elo`. The engine would clamp every one of those to the floor and they would play the same. Skill Level stays at 0, because a higher skill searched deeply can play above the floor. Depth (1–6), nodes (8–800), and movetime (40–500ms) rise with the rating. The chance of a random legal move falls from 85% at the bottom of the band to 0% at the step just under the floor. Hypersion's minimum is 500, so this band is empty for it.
- If the engine has no `UCI_Elo` range, the whole ladder uses that shape and Skill Level also rises from 0 to 20, with a longer search at the top.

Each step is strictly stronger than the one below it. The code for this lives in `internal/engine/elo.go`.

## Layout

```
cmd/termchess          program entry, --engine, --color, log file
internal/coord         screen-relative cursor, stops at the edge
internal/game          rules wrapper, PGN, undo, draws, flag fall
internal/interact      board state machine
internal/engine        Hypersion UCI adapter and a scripted mock
internal/clock         two Bubbles timers, increment, flag
internal/theme         four palettes
internal/config        persisted settings
internal/store         PGN files in the data directory
internal/ui            Bubble Tea model, Huh forms, board rendering
```

### Board state machine

```
idle → piece selected → awaiting promotion → move committed
                ↑ esc
engine-thinking → pre-move selected → pre-move promotion → back to thinking
game over
```

Enter on your piece selects it and shows legal moves. Enter on another of your pieces switches the selection. Enter on a highlighted square commits the move. Enter on anything else flashes and keeps the selection. Esc clears the current layer.

### Root model

One Bubble Tea model (`internal/ui.App`) owns the screen:

- **menu** — Huh select: two-player, computer, load
- **setup** — time control, flip or colour and Elo
- **play** — board, clocks, move viewport, captured material (a dim hyphen when that side has taken nothing), status, help
- **overlays** — promotion picker, Huh confirm, save prompt, load list

Game-over is the result line on the status row. The footer is the only place that offers new game, save, and quit. Engine searches, directory listings, and file writes are `tea.Cmd` values, so `Update` never blocks. The chess clock is a `tea.Tick` every 100ms that feeds the active Bubbles timer. Move pacing is a second tick: human moves, engine replies, and pre-moves are committed from one queue on the `movePaceMs` cadence.

## Tests

```sh
make test
```

Covered: cursor movement including a flipped board and the cursor staying put after an opponent reply, a pre-move, and undo, selection and legal targets, promotion, pre-moves, clock increments and flag fall, undo, PGN round-trip, the Elo ladder and its strength mapping, config persistence, and key-message walks through the menu, a move, undo, help, quit confirmation, and the engine mock.

## Libraries

- [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) — `charm.land/bubbletea/v2`
- [Lip Gloss v2](https://github.com/charmbracelet/lipgloss) — styling
- [Bubbles v2](https://github.com/charmbracelet/bubbles) — viewport, timer, help, key
- [Huh v2](https://github.com/charmbracelet/huh) — forms
- [Log](https://github.com/charmbracelet/log) — file logging
- [Harmonica](https://github.com/charmbracelet/harmonica) — a short flash when the king is in check
