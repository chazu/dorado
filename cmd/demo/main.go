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

var colorBG = display.ColorRGB(168, 168, 168) // ST-80 medium gray

func main() {
	font := display.DefaultFont()
	fmt.Printf("Cozette: %d glyphs, %dpx line height\n", len(font.Glyphs), font.LineHeight())

	screen := display.NewForm(screenW, screenH)
	backend := display.NewEbitengineBackend(screen)
	wm := display.NewWindowManager(screen)

	// Create some windows
	workspace := display.NewWindow(80, 60, 500, 380, "Workspace")
	drawWorkspaceContent(workspace.Content)
	wm.AddWindow(workspace)

	browser := display.NewWindow(300, 200, 600, 430, "System Browser")
	drawBrowserContent(browser.Content)
	wm.AddWindow(browser)

	transcript := display.NewWindow(700, 80, 400, 300, "Transcript")
	drawTranscriptContent(transcript.Content)
	wm.AddWindow(transcript)

	backend.OnUpdate = func() {
		for _, e := range backend.PollEvents() {
			wm.HandleEvent(e)
		}
		wm.Composite(colorBG)
	}

	if err := backend.Run(); err != nil {
		log.Fatal(err)
	}
}

func drawWorkspaceContent(f *display.Form) {
	black := display.ColorRGB(0, 0, 0)
	lh := display.DefaultFont().LineHeight() + 3
	y := 8
	display.DrawString(f, 8, y, "\"Welcome to Dorado — the ST-80 IDE for Maggie.\"", black)
	y += lh * 2
	display.DrawString(f, 8, y, "| x |", black)
	y += lh
	display.DrawString(f, 8, y, "x := 42 factorial.", black)
	y += lh
	display.DrawString(f, 8, y, "Transcript show: x printString.", black)
	y += lh * 2
	display.DrawString(f, 8, y, "\"Unicode: αβγδ → ∞ ≠ ≈ © ♠♣♥♦ ★\"", black)
}

func drawBrowserContent(f *display.Form) {
	black := display.ColorRGB(0, 0, 0)
	gray := display.ColorRGB(168, 168, 168)
	lh := display.DefaultFont().LineHeight() + 3

	// Simulate the four-pane header
	pw := f.Width() / 4
	for i := 1; i < 4; i++ {
		x := pw * i
		for y := 0; y < 120; y++ {
			f.SetPixelAt(x, y, gray)
		}
	}
	for x := 0; x < f.Width(); x++ {
		f.SetPixelAt(x, 120, gray)
	}

	// Category pane
	y := 8
	for _, cat := range []string{"Kernel-Objects", "Kernel-Classes", "Kernel-Methods", "Collections-Abstract", "Collections-Sequenceable"} {
		display.DrawString(f, 8, y, cat, black)
		y += lh
	}

	// Class pane
	y = 8
	for _, cls := range []string{"Object", "Boolean", "True", "False", "UndefinedObject"} {
		display.DrawString(f, pw+8, y, cls, black)
		y += lh
	}

	// Protocol pane
	y = 8
	for _, proto := range []string{"accessing", "comparing", "copying", "printing", "testing"} {
		display.DrawString(f, pw*2+8, y, proto, black)
		y += lh
	}

	// Method pane
	y = 8
	for _, meth := range []string{"yourself", "printString", "respondsTo:", "isKindOf:", "error:"} {
		display.DrawString(f, pw*3+8, y, meth, black)
		y += lh
	}

	// Code area
	y = 130
	display.DrawString(f, 8, y, "printString", black)
	y += lh
	display.DrawString(f, 8, y, "    \"Return a string representation of the receiver.\"", black)
	y += lh
	display.DrawString(f, 8, y, "    ^ self class name", black)
}

func drawTranscriptContent(f *display.Form) {
	black := display.ColorRGB(0, 0, 0)
	lh := display.DefaultFont().LineHeight() + 3
	y := 8
	display.DrawString(f, 8, y, "Dorado started.", black)
	y += lh
	display.DrawString(f, 8, y, "Loading Workspace...", black)
	y += lh
	display.DrawString(f, 8, y, "Loading System Browser...", black)
	y += lh
	display.DrawString(f, 8, y, "Ready.", black)
}
