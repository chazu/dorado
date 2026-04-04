package main

import (
	"fmt"
	"log"

	"github.com/chazu/dorado/go/display"
)

const (
	screenW = 1280
	screenH = 960
)

var (
	colorBG       = display.ColorRGB(168, 168, 168) // ST-80 medium gray
	colorWhite    = display.ColorRGB(255, 255, 255)
	colorBlack    = display.ColorRGB(0, 0, 0)
	colorDarkGray = display.ColorRGB(80, 80, 80)
)

func main() {
	// Pre-load the font so any parse error is caught early
	font := display.DefaultFont()
	fmt.Printf("Cozette: %d glyphs, %dpx line height\n", len(font.Glyphs), font.LineHeight())

	screen := display.NewForm(screenW, screenH)
	backend := display.NewEbitengineBackend(screen)

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

	// Draw two overlapping "windows"
	drawWindow(screen, 80, 60, 500, 400, "Workspace")
	drawWindow(screen, 300, 200, 600, 450, "System Browser")

	// Demo text in the Workspace content area
	display.DrawString(screen, 92, 96, "Welcome to Dorado — the ST-80 IDE for Maggie.", colorBlack)
	display.DrawString(screen, 92, 112, "Form + BitBlt + Cozette font rendering.", colorBlack)
	display.DrawString(screen, 92, 128, "Unicode: αβγδ → ∞ ≠ ≈ © ♠♣♥♦ ★", colorBlack)
	display.DrawString(screen, 92, 160, "| x |", colorBlack)
	display.DrawString(screen, 92, 176, "x := 42 factorial.", colorBlack)
	display.DrawString(screen, 92, 192, "Transcript show: x printString.", colorBlack)

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
	titleH := 20

	// White content area
	screen.FillRectWH(colorWhite, x, y, w, h)

	// Dark gray title bar
	screen.FillRectWH(colorDarkGray, x, y, w, titleH)

	// Close box (small square in top-left of title bar)
	closeSize := 12
	closePad := (titleH - closeSize) / 2
	closeX := x + closePad
	closeY := y + closePad
	screen.FillRectWH(colorWhite, closeX, closeY, closeSize, closeSize)
	drawHLine(screen, closeX, closeY, closeSize)
	drawHLine(screen, closeX, closeY+closeSize-1, closeSize)
	drawVLine(screen, closeX, closeY, closeSize)
	drawVLine(screen, closeX+closeSize-1, closeY, closeSize)

	// Title text — positioned after close box with gap
	textX := closeX + closeSize + 8
	textY := y + (titleH-display.DefaultFont().LineHeight())/2
	display.DrawString(screen, textX, textY, title, colorWhite)

	// 1px black border
	drawHLine(screen, x, y, w)
	drawHLine(screen, x, y+h-1, w)
	drawVLine(screen, x, y, h)
	drawVLine(screen, x+w-1, y, h)

	// Title bar separator
	drawHLine(screen, x, y+titleH, w)
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

// drawCursorFollower draws a small inverted square at the mouse position.
func drawCursorFollower(screen *display.Form, mx, my int) {
	size := 8
	op := &display.BitBltOp{
		Dst:    screen,
		DstX:   mx - size/2,
		DstY:   my - size/2,
		Width:  size,
		Height: size,
		Rule:   display.RuleInvertDest,
	}
	op.Execute()
}
