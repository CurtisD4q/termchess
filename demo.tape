# Record a short pass-and-play game.
#   go install github.com/charmbracelet/vhs@latest
#   vhs demo.tape
Output demo.gif
Set Shell bash
Set FontSize 18
Set Width 1200
Set Height 800
Set TypingSpeed 40ms
Set Padding 16

Type "go run ./cmd/termchess"
Enter
Sleep 1s
# Start menu: Local two-player is already selected.
Enter
Sleep 500ms
# Setup: flip = No, time = Bullet is not the default. Accept the defaults.
Enter
Sleep 200ms
Enter
Sleep 200ms
Enter
Sleep 200ms
Enter
Sleep 200ms
Enter
Sleep 800ms
# e2, then e4.
Enter
Sleep 200ms
Type "k"
Sleep 120ms
Type "k"
Sleep 120ms
Enter
Sleep 600ms
Type "?"
Sleep 1s
Type "?"
Sleep 400ms
Type "q"
Sleep 400ms
Type "y"
Sleep 200ms
Enter
Sleep 500ms
