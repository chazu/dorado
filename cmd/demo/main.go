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

	// Workspace — editable text pane
	workspace := display.NewWindow(80, 60, 500, 380, "Workspace")
	workspace.SetEditor(`"Welcome to Dorado — the ST-80 IDE for Maggie."

| x |
x := 42 factorial.
Transcript show: x printString.

"Unicode: αβγδ → ∞ ≠ ≈ © ♠♣♥♦ ★"

"Try typing, selecting text, and editing!"`)
	wm.AddWindow(workspace)

	// System Browser — static content for now
	browser := display.NewWindow(300, 200, 600, 430, "System Browser")
	drawBrowserContent(browser.Content)
	wm.AddWindow(browser)

	// Transcript — editable
	transcript := display.NewWindow(700, 80, 400, 300, "Transcript")
	transcript.SetEditor(`Dorado started.
Loading Workspace...
Loading System Browser...
Ready.`)
	wm.AddWindow(transcript)

	// World menu (right-click on desktop)
	wm.WorldMenuFunc = func(x, y int) []display.MenuItem {
		return []display.MenuItem{
			{Label: "New Workspace", Action: func() {
				w := display.NewWindow(x, y, 400, 300, "Workspace")
				w.SetEditor("")
				wm.AddWindow(w)
			}},
			display.Separator(),
			{Label: "About Dorado", Disabled: true},
		}
	}

	// Window context menu (right-click in content)
	wm.WindowMenuFunc = func(w *display.Window, x, y int) []display.MenuItem {
		items := []display.MenuItem{
			{Label: "Cut", Action: func() {
				if w.Editor != nil {
					w.Editor.Cut()
					w.MarkDirty()
				}
			}},
			{Label: "Copy", Action: func() {
				if w.Editor != nil {
					w.Editor.Copy()
				}
			}},
			{Label: "Paste", Action: func() {
				if w.Editor != nil {
					w.Editor.Paste()
					w.MarkDirty()
				}
			}},
			display.Separator(),
			{Label: "Select All", Action: func() {
				if w.Editor != nil {
					w.Editor.HandleEvent(display.Event{
						Type: display.EventKeyDown,
						Key:  int(65), // 'A' key — will be handled with cmd/ctrl
					})
					w.MarkDirty()
				}
			}},
		}
		return items
	}

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
	display.DrawString(f, 8, y, `    "Return a string representation of the receiver."`, black)
	y += lh
	display.DrawString(f, 8, y, "    ^ self class name", black)
}
