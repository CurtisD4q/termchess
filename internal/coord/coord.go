// Package coord maps chess squares to the screen and moves a cursor.
//
// The cursor is stored as a board square. Arrow keys and hjkl are
// screen-relative: up always moves toward the top of the terminal, including
// when the board is drawn with black at the bottom. Movement stops at the
// edge of the board.
package coord

import "github.com/notnil/chess"

// Dir is a screen-relative cursor step.
type Dir int

const (
	Up Dir = iota
	Down
	Left
	Right
)

// Visual returns the on-screen row and column of sq.
// Row 0 is the top of the board. When flipped is false, white is at the bottom.
func Visual(sq chess.Square, flipped bool) (row, col int) {
	file := int(sq.File())
	rank := int(sq.Rank())
	if !flipped {
		return 7 - rank, file
	}
	return rank, 7 - file
}

// Square converts a visual row and column back into a board square.
func Square(row, col int, flipped bool) chess.Square {
	var file, rank int
	if !flipped {
		file = col
		rank = 7 - row
	} else {
		file = 7 - col
		rank = row
	}
	return chess.NewSquare(chess.File(file), chess.Rank(rank))
}

// Move steps the cursor one square. The boolean is false when the cursor is
// already on that edge and stays put.
func Move(sq chess.Square, dir Dir, flipped bool) (chess.Square, bool) {
	row, col := Visual(sq, flipped)
	switch dir {
	case Up:
		row--
	case Down:
		row++
	case Left:
		col--
	case Right:
		col++
	}
	if row < 0 || row > 7 || col < 0 || col > 7 {
		return sq, false
	}
	return Square(row, col, flipped), true
}
