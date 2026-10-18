package pieces

import "embed"

// Files holds the SVG sources, the contact sheet, and the pre-rendered PNGs.
//
//go:embed *.svg preview.png png/*.png
var Files embed.FS
