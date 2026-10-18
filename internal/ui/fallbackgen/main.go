// Generates the quadrant text-art contact sheet and the large starting board.
package main

import (
	"image"
	"image/png"
	"os"

	"termchess/internal/ui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "sets" {
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		if err := ui.WritePieceSetPreviews(dir); err != nil {
			panic(err)
		}
		return
	}
	path := "pieces-fallback-preview.png"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	writePNG(path, ui.FallbackPreview())
	if len(os.Args) > 2 {
		writePNG(os.Args[2], ui.BoardPreview())
	}
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}
