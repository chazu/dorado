package main

import (
	"log"

	"github.com/chazu/dorado/go/display"
)

const (
	screenW = 1280
	screenH = 960
)

var (
	colorBG      = display.ColorRGB(168, 168, 168) // ST-80 medium gray
	colorWhite   = display.ColorRGB(255, 255, 255)
	colorBlack   = display.ColorRGB(0, 0, 0)
	colorDarkGray = display.ColorRGB(80, 80, 80)
)

func main() {
	screen := display.NewForm(screenW, screenH)
	backend := display.NewEbitengineBackend(screen)

	// Draw the initial scene
	drawScene(screen)

	// Track mouse position for the cursor follower
	cursorX, cursorY := 0, 0

	backend.OnUpdate = func() {
		events := backend.PollEvents()
		for _, e := range events {
			if e.Type == display.EventMouseMove {
				cursorX = e.X
				cursorY = e.Y
			}
		}

		// Redraw scene + cursor follower each frame
		drawScene(screen)
		drawCursorFollower(screen, cursorX, cursorY)
	}

	if err := backend.Run(); err != nil {
		log.Fatal(err)
	}
}

func drawScene(screen *display.Form) {
	// Gray desktop background
	screen.Fill(colorBG)

	// Draw two overlapping "windows" to demonstrate z-order and BitBlt
	drawWindow(screen, 80, 60, 500, 400, "Workspace")
	drawWindow(screen, 300, 200, 600, 450, "System Browser")

	// XOR checkerboard pattern in the corner to demonstrate rule 6
	checker := display.NewForm(128, 128)
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			if (x/8+y/8)%2 == 0 {
				checker.SetPixelAt(x, y, colorBlack)
			} else {
				checker.SetPixelAt(x, y, colorWhite)
			}
		}
	}
	op := &display.BitBltOp{
		Dst:    screen,
		Src:    checker,
		DstX:   screenW - 160,
		DstY:   screenH - 160,
		Width:  128,
		Height: 128,
		Rule:   display.RuleXor,
	}
	op.Execute()
}

func drawWindow(screen *display.Form, x, y, w, h int, title string) {
	titleH := 24

	// White content area
	screen.FillRectWH(colorWhite, x, y, w, h)

	// Dark gray title bar
	screen.FillRectWH(colorDarkGray, x, y, w, titleH)

	// Render title text as simple block letters (no font yet)
	// Start after the close box (x+4 + 16px box + 6px gap = x+26)
	tx := x + 26
	ty := y + 5
	for _, ch := range title {
		drawChar(screen, tx, ty, ch)
		tx += 12 // 10px char width + 2px kerning gap
	}

	// 1px black border
	drawHLine(screen, x, y, w)
	drawHLine(screen, x, y+h-1, w)
	drawVLine(screen, x, y, h)
	drawVLine(screen, x+w-1, y, h)

	// Title bar separator
	drawHLine(screen, x, y+titleH, w)

	// Close box (small square in top-left)
	screen.FillRectWH(colorWhite, x+4, y+4, 16, 16)
	drawHLine(screen, x+4, y+4, 16)
	drawHLine(screen, x+4, y+19, 16)
	drawVLine(screen, x+4, y+4, 16)
	drawVLine(screen, x+19, y+4, 16)
}

func drawHLine(f *display.Form, x, y, w int) {
	for i := 0; i < w; i++ {
		f.SetPixelAt(x+i, y, colorBlack)
	}
}

func drawVLine(f *display.Form, x, y, h int) {
	for i := 0; i < h; i++ {
		f.SetPixelAt(x, y+i, colorBlack)
	}
}

// drawChar renders a very crude 8x16 block character for the demo.
// This is placeholder until we integrate the Cozette bitmap font.
func drawChar(f *display.Form, x, y int, ch rune) {
	// Minimal 5x7 bitmaps for uppercase + space, enough for window titles
	glyphs := map[rune][]string{
		'S': {"_###_", "#___#", "#____", "_###_", "____#", "#___#", "_###_"},
		'y': {"#___#", "#___#", "_#_#_", "__#__", "_#___", "#____", "#____"},
		's': {"_####", "#____", "_###_", "____#", "____#", "####_", "_____"},
		't': {"#####", "__#__", "__#__", "__#__", "__#__", "__#__", "__#__"},
		'e': {"#####", "#____", "#####", "#____", "#____", "#####", "_____"},
		'm': {"#___#", "##_##", "#_#_#", "#___#", "#___#", "#___#", "_____"},
		' ': {"_____", "_____", "_____", "_____", "_____", "_____", "_____"},
		'B': {"####_", "#___#", "####_", "#___#", "#___#", "####_", "_____"},
		'r': {"#_##_", "##__#", "#____", "#____", "#____", "#____", "_____"},
		'o': {"_###_", "#___#", "#___#", "#___#", "#___#", "_###_", "_____"},
		'w': {"#___#", "#___#", "#_#_#", "#_#_#", "##_##", "#___#", "_____"},
		'W': {"#___#", "#___#", "#_#_#", "#_#_#", "##_##", "_#_#_", "_____"},
		'k': {"#__#_", "#_#__", "##___", "#_#__", "#__#_", "#___#", "_____"},
		'p': {"####_", "#___#", "#___#", "####_", "#____", "#____", "_____"},
		'a': {"_###_", "#___#", "#___#", "#####", "#___#", "#___#", "_____"},
		'c': {"_###_", "#___#", "#____", "#____", "#___#", "_###_", "_____"},
		'i': {"__#__", "_____", "__#__", "__#__", "__#__", "__#__", "_____"},
	}
	glyph, ok := glyphs[ch]
	if !ok {
		return
	}
	for row, line := range glyph {
		for col, c := range line {
			if c == '#' {
				f.SetPixelAt(x+col*2, y+row*2, colorWhite)
				f.SetPixelAt(x+col*2+1, y+row*2, colorWhite)
				f.SetPixelAt(x+col*2, y+row*2+1, colorWhite)
				f.SetPixelAt(x+col*2+1, y+row*2+1, colorWhite)
			}
		}
	}
}

// drawCursorFollower draws a small black square at the mouse position
// to prove that event polling works.
func drawCursorFollower(screen *display.Form, mx, my int) {
	size := 8
	op := &display.BitBltOp{
		Dst:    screen,
		DstX:   mx - size/2,
		DstY:   my - size/2,
		Width:  size,
		Height: size,
		Rule:   display.RuleInvertDest, // XOR-style invert so it's visible on any background
	}
	op.Execute()
}
