package main

import (
	"fmt"
	"strings"

	"github.com/chazu/dorado/go/display"
	"github.com/chazu/maggie/vm"
)

// Debugger manages the debugger UI.
type Debugger struct {
	window *display.Window
	active bool

	// UI state
	frames    []vm.StackFrame
	variables []vm.Variable
	selFrame  int // selected stack frame index

	// Layout
	stackPaneH int
	varPaneH   int
	codePaneH  int
}

var debugger *Debugger

// openDebugger creates or focuses the debugger window.
func openDebugger() {
	if debugger != nil && debugger.window != nil && !debugger.window.Closed {
		app.wm.BringToFront(debugger.window)
		return
	}

	w := display.NewWindow(100, 100, 700, 500, "Debugger")
	debugger = &Debugger{
		window:     w,
		selFrame:   -1,
		stackPaneH: 150,
		varPaneH:   150,
	}

	w.OnContentClick = func(lx, ly int) {
		debugger.handleClick(lx, ly)
	}

	// Activate the VM debugger
	app.vm.Debugger.Activate()
	debugger.active = true

	debugger.render()
	app.wm.AddWindow(w)

	transcriptWrite("Debugger activated.")
}

// closeDebugger deactivates the debugger.
func closeDebugger() {
	if debugger == nil {
		return
	}
	if app.vm.Debugger != nil {
		app.vm.Debugger.Deactivate()
		// Resume if paused
		if app.vm.Debugger.IsPaused() {
			app.vm.Debugger.Resume()
		}
	}
	debugger.active = false
	debugger = nil
	transcriptWrite("Debugger deactivated.")
}

// refreshDebugger updates the debugger display with current VM state.
func refreshDebugger() {
	if debugger == nil || debugger.window.Closed {
		return
	}
	d := debugger

	if app.vm.Debugger.IsPaused() {
		d.frames = app.vm.DebugCallStack()
		if d.selFrame < 0 && len(d.frames) > 0 {
			d.selFrame = 0
		}
		if d.selFrame >= 0 && d.selFrame < len(d.frames) {
			d.variables = app.vm.DebugVariables(d.frames[d.selFrame].ID)
		}
	} else {
		d.frames = nil
		d.variables = nil
		d.selFrame = -1
	}

	d.render()
}

func (d *Debugger) render() {
	f := d.window.Content
	w := f.Width()
	font := display.DefaultFont()
	lh := font.LineHeight() + 2

	white := display.ColorRGB(255, 255, 255)
	black := display.ColorRGB(0, 0, 0)
	gray := display.ColorRGB(180, 180, 180)
	darkGray := display.ColorRGB(100, 100, 100)
	selBG := display.ColorRGB(40, 40, 120)
	selFG := display.ColorRGB(255, 255, 255)
	btnBG := display.ColorRGB(200, 200, 200)

	f.Fill(white)

	paused := app.vm.Debugger != nil && app.vm.Debugger.IsPaused()

	// --- Button bar at top ---
	btnY := 4
	btnH := 22
	btnW := 80
	btnGap := 4
	bx := 4

	buttons := []struct {
		label   string
		enabled bool
		action  func()
	}{
		{"Resume", paused, func() { app.vm.Debugger.Resume() }},
		{"Step Over", paused, func() {
			if d.selFrame >= 0 && d.selFrame < len(d.frames) {
				f := d.frames[d.selFrame]
				app.vm.Debugger.StepOver(f.ID, f.Line)
			}
		}},
		{"Step Into", paused, func() { app.vm.Debugger.StepInto() }},
		{"Step Out", paused, func() {
			if d.selFrame >= 0 && d.selFrame < len(d.frames) {
				app.vm.Debugger.StepOut(d.frames[d.selFrame].ID)
			}
		}},
		{"Pause", !paused, func() { app.vm.Debugger.Pause() }},
	}

	for _, btn := range buttons {
		bg := btnBG
		fg := black
		if !btn.enabled {
			fg = gray
		}
		f.FillRectWH(bg, bx, btnY, btnW, btnH)
		display.DrawRect(f, bx, btnY, btnW, btnH, darkGray)
		textX := bx + (btnW-font.MeasureString(btn.label))/2
		textY := btnY + (btnH-font.LineHeight())/2
		display.DrawString(f, textX, textY, btn.label, fg)
		bx += btnW + btnGap
	}

	// --- Stack frames pane ---
	stackY := btnY + btnH + 8
	display.DrawString(f, 4, stackY, "Call Stack:", darkGray)
	stackY += lh

	display.DrawHLine(f, 0, stackY-1, w, gray)

	for i, frame := range d.frames {
		iy := stackY + i*lh
		if iy+lh > stackY+d.stackPaneH {
			break
		}
		label := fmt.Sprintf("%s >> %s  (line %d)", frame.Class, frame.Method, frame.Line)
		if frame.IsBlock {
			label = "  [] in " + label
		}
		if i == d.selFrame {
			f.FillRectWH(selBG, 0, iy, w, lh)
			display.DrawString(f, 8, iy, label, selFG)
		} else {
			display.DrawString(f, 8, iy, label, black)
		}
	}

	if len(d.frames) == 0 {
		status := "Running..."
		if !d.active {
			status = "Debugger inactive"
		}
		display.DrawString(f, 8, stackY, status, gray)
	}

	// --- Variables pane ---
	varY := stackY + d.stackPaneH + 4
	display.DrawHLine(f, 0, varY-1, w, gray)
	display.DrawString(f, 4, varY, "Variables:", darkGray)
	varY += lh

	for i, v := range d.variables {
		iy := varY + i*lh
		if iy+lh > varY+d.varPaneH {
			break
		}
		line := fmt.Sprintf("%-20s %s  (%s)", v.Name, v.Value, v.Type)
		display.DrawString(f, 8, iy, line, black)
	}

	if len(d.variables) == 0 && len(d.frames) > 0 {
		display.DrawString(f, 8, varY, "(no variables)", gray)
	}

	// --- Source pane ---
	srcY := varY + d.varPaneH + 4
	display.DrawHLine(f, 0, srcY-1, w, gray)
	display.DrawString(f, 4, srcY, "Source:", darkGray)
	srcY += lh

	if d.selFrame >= 0 && d.selFrame < len(d.frames) {
		frame := d.frames[d.selFrame]
		// Try to find method source
		cls := app.vm.Classes.Lookup(frame.Class)
		if cls != nil {
			selID := app.vm.Selectors.Intern(frame.Method)
			method := cls.VTable.LocalMethods()[selID]
			if method != nil {
				if cm, ok := method.(*vm.CompiledMethod); ok && cm.Source != "" {
					lines := strings.Split(cm.Source, "\n")
					for i, line := range lines {
						ly := srcY + i*lh
						if ly+lh > f.Height() {
							break
						}
						lineNum := i + 1
						// Highlight current line
						if lineNum == frame.Line {
							f.FillRectWH(display.ColorRGB(255, 255, 200), 0, ly, w, lh)
							display.DrawString(f, 4, ly, "→", display.ColorRGB(200, 0, 0))
						}
						display.DrawString(f, 20, ly, fmt.Sprintf("%3d  %s", lineNum, line), black)
					}
				} else {
					display.DrawString(f, 8, srcY, "(source not available)", gray)
				}
			}
		}
	}

	d.window.MarkDirty()
}

func (d *Debugger) handleClick(lx, ly int) {
	font := display.DefaultFont()
	lh := font.LineHeight() + 2
	btnH := 22

	// Check button bar
	btnY := 4
	if ly >= btnY && ly < btnY+btnH {
		btnW := 80
		btnGap := 4
		idx := (lx - 4) / (btnW + btnGap)
		paused := app.vm.Debugger != nil && app.vm.Debugger.IsPaused()

		switch idx {
		case 0: // Resume
			if paused {
				app.vm.Debugger.Resume()
			}
		case 1: // Step Over
			if paused && d.selFrame >= 0 && d.selFrame < len(d.frames) {
				f := d.frames[d.selFrame]
				app.vm.Debugger.StepOver(f.ID, f.Line)
			}
		case 2: // Step Into
			if paused {
				app.vm.Debugger.StepInto()
			}
		case 3: // Step Out
			if paused && d.selFrame >= 0 && d.selFrame < len(d.frames) {
				app.vm.Debugger.StepOut(d.frames[d.selFrame].ID)
			}
		case 4: // Pause
			if !paused {
				app.vm.Debugger.Pause()
			}
		}
		// Refresh after a short delay to let the VM respond
		refreshDebugger()
		return
	}

	// Check stack frame clicks
	stackY := btnY + btnH + 8 + lh
	if ly >= stackY && ly < stackY+d.stackPaneH {
		idx := (ly - stackY) / lh
		if idx >= 0 && idx < len(d.frames) {
			d.selFrame = idx
			d.variables = app.vm.DebugVariables(d.frames[idx].ID)
			d.render()
		}
	}
}

