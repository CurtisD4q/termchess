// Generates the SVG sources and the embedded PNG sizes from the drawing code.
package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strconv"

	"github.com/notnil/chess"
	"termchess/assets/pieces"
)

func main() {
	dir := "assets/pieces"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	pngDir := filepath.Join(dir, "png")
	if err := os.MkdirAll(pngDir, 0o755); err != nil {
		panic(err)
	}
	for _, side := range []chess.Color{chess.White, chess.Black} {
		for _, pt := range pieces.Types() {
			name := pieces.Name(side, pt)
			if err := os.WriteFile(filepath.Join(dir, name+".svg"), []byte(pieces.SVG(side, pt)), 0o644); err != nil {
				panic(err)
			}
			for _, size := range []int{32, 64, 128} {
				buf, err := pieces.PNG(side, pt, size)
				if err != nil {
					panic(err)
				}
				path := filepath.Join(pngDir, name+"-"+strconv.Itoa(size)+".png")
				if err := os.WriteFile(path, buf, 0o644); err != nil {
					panic(err)
				}
			}
		}
	}
	f, err := os.Create(filepath.Join(dir, "preview.png"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, pieces.Preview()); err != nil {
		panic(err)
	}
}
