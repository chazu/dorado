package main

import (
	"fmt"
	"strings"

	"github.com/chazu/dorado/go/display"
	"github.com/chazu/maggie/vm"
)

// inspectorEntry represents a row in the inspector's variable list.
type inspectorEntry struct {
	Name  string
	Value vm.Value
}

// Inspector is a two-pane object inspector with an evaluation pane.
type Inspector struct {
	window   *display.Window
	vmInst   *vm.VM
	object   vm.Value // the inspected object
	entries  []inspectorEntry
	selIndex int // selected entry (-1 = self)

	// Layout
	listPaneW int
	listPaneH int

	// Eval pane
	evalForm   *display.Form
	evalEditor *display.TextEditor
}

// inspectorRegistry maps windows to their Inspector instances.
var inspectorRegistry = map[*display.Window]*Inspector{}

func getInspectorForWindow(w *display.Window) *Inspector {
	return inspectorRegistry[w]
}

func inspectorMenu(insp *Inspector) []display.MenuItem {
	return []display.MenuItem{
		{Label: "Do it  (⌘D)", Action: func() { insp.doIt() }},
		{Label: "Print it  (⌘P)", Action: func() { insp.printIt() }},
		{Label: "Inspect it  (⌘I)", Action: func() { insp.inspectSelected() }},
		display.Separator(),
		{Label: "Cut", Action: func() {
			insp.evalEditor.Cut()
			insp.render()
		}},
		{Label: "Copy", Action: func() {
			insp.evalEditor.Copy()
		}},
		{Label: "Paste", Action: func() {
			insp.evalEditor.Paste()
			insp.render()
		}},
	}
}

func openInspector(vmInst *vm.VM, val vm.Value) {
	contentW, contentH := 520, 380
	title := "Inspector: " + shortDescription(vmInst, val)
	w := display.NewWindow(200, 150, contentW, contentH, title)

	insp := &Inspector{
		window:    w,
		vmInst:    vmInst,
		object:    val,
		selIndex:  -1, // self selected by default
		listPaneW: 180,
		listPaneH: 260,
	}

	insp.buildEntries()

	// Eval pane at bottom
	evalH := contentH - insp.listPaneH - 4
	insp.evalForm = display.NewForm(contentW, evalH)
	insp.evalEditor = display.NewTextEditor(insp.evalForm, "")

	w.Content = display.NewForm(contentW, contentH)

	w.OnContentClick = func(lx, ly int) {
		insp.handleClick(lx, ly)
	}

	w.OnKeyEvent = func(e display.Event) bool {
		// Eval pane gets keyboard input
		evalY := insp.listPaneH + 4
		consumed := insp.evalEditor.HandleEvent(e)
		if consumed {
			insp.render()
		}
		_ = evalY
		return consumed
	}

	insp.render()
	inspectorRegistry[w] = insp
	app.wm.AddWindow(w)
}

func (insp *Inspector) buildEntries() {
	insp.entries = nil
	val := insp.object

	if val.IsObject() {
		obj := vm.ObjectFromValue(val)
		if obj != nil {
			class := insp.vmInst.GetClassFromValue(val)
			if class != nil {
				ivars := class.AllInstVarNames()
				for i, name := range ivars {
					if i < obj.NumSlots() {
						insp.entries = append(insp.entries, inspectorEntry{
							Name:  name,
							Value: obj.GetSlot(i),
						})
					}
				}
			}
		}
	}

	// Add synthetic entries for non-object types
	if val.IsSmallInt() {
		insp.entries = append(insp.entries,
			inspectorEntry{Name: "value", Value: val},
		)
	} else if vm.IsStringValue(val) {
		insp.entries = append(insp.entries,
			inspectorEntry{Name: "contents", Value: val},
		)
	} else if val.IsSymbol() {
		insp.entries = append(insp.entries,
			inspectorEntry{Name: "name", Value: val},
		)
	}
}

func (insp *Inspector) selectedValue() vm.Value {
	if insp.selIndex < 0 || insp.selIndex >= len(insp.entries) {
		return insp.object // self
	}
	return insp.entries[insp.selIndex].Value
}

func (insp *Inspector) render() {
	f := insp.window.Content
	w := f.Width()
	font := display.DefaultFont()
	lh := font.LineHeight() + 2

	white := display.ColorRGB(255, 255, 255)
	black := display.ColorRGB(0, 0, 0)
	gray := display.ColorRGB(180, 180, 180)
	selBG := display.ColorRGB(40, 40, 120)
	selFG := display.ColorRGB(255, 255, 255)

	f.Fill(white)

	listW := insp.listPaneW
	valueX := listW + 1
	valueW := w - listW - 1

	// --- Left pane: variable names ---
	y := 4

	// "self" entry
	if insp.selIndex == -1 {
		f.FillRectWH(selBG, 0, y, listW, lh)
		display.DrawString(f, 8, y, "self", selFG)
	} else {
		display.DrawString(f, 8, y, "self", black)
	}
	y += lh

	// Separator after self
	display.DrawHLine(f, 0, y, listW, gray)
	y += 2

	// Instance variables
	for i, entry := range insp.entries {
		if y+lh > insp.listPaneH {
			break
		}
		if i == insp.selIndex {
			f.FillRectWH(selBG, 0, y, listW, lh)
			display.DrawString(f, 8, y, entry.Name, selFG)
		} else {
			display.DrawString(f, 8, y, entry.Name, black)
		}
		y += lh
	}

	// Vertical separator between panes
	display.DrawVLine(f, listW, 0, insp.listPaneH, gray)

	// --- Right pane: selected value ---
	selVal := insp.selectedValue()
	className := valueClassName(insp.vmInst, selVal)
	printStr := valuePrintString(insp.vmInst, selVal)

	vy := 4
	display.DrawString(f, valueX+8, vy, className, gray)
	vy += lh

	// Word-wrap the printString into the value pane
	lines := wrapText(printStr, valueW-16, font)
	for _, line := range lines {
		if vy+lh > insp.listPaneH {
			break
		}
		display.DrawString(f, valueX+8, vy, line, black)
		vy += lh
	}

	// --- Horizontal separator ---
	sepY := insp.listPaneH
	display.DrawHLine(f, 0, sepY, w, gray)

	// Label
	display.DrawString(f, 4, sepY+2, "evaluate (in context of self):", gray)

	// --- Eval pane ---
	evalY := sepY + lh + 4
	evalH := f.Height() - evalY
	if insp.evalForm.Height() != evalH || insp.evalForm.Width() != w {
		oldText := insp.evalEditor.Text()
		insp.evalForm = display.NewForm(w, evalH)
		insp.evalEditor = display.NewTextEditor(insp.evalForm, oldText)
	}
	insp.evalEditor.Render()
	display.CopyBits(f, 0, evalY, insp.evalForm)

	insp.window.MarkDirty()
}

func (insp *Inspector) handleClick(lx, ly int) {
	font := display.DefaultFont()
	lh := font.LineHeight() + 2

	if ly < insp.listPaneH && lx < insp.listPaneW {
		// Click in variable list
		y := 4
		// Check "self" row
		if ly >= y && ly < y+lh {
			insp.selIndex = -1
			insp.render()
			return
		}
		y += lh + 2 // skip separator

		for i := range insp.entries {
			if ly >= y && ly < y+lh {
				insp.selIndex = i
				insp.render()
				return
			}
			y += lh
		}
	} else if ly >= insp.listPaneH {
		// Click in eval pane
		evalY := insp.listPaneH + lh + 4
		insp.evalEditor.HandleClickLocal(lx, ly-evalY, false)
		insp.render()
	}
}

// inspectorDoIt evaluates an expression in the context of the inspected object.
func (insp *Inspector) doIt() {
	source := getInspectorEvalSource(insp)
	if source == "" {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			transcriptWrite(fmt.Sprintf("Inspector eval error: %v", r))
		}
	}()

	// Wrap the expression so 'self' refers to the inspected object
	// We compile and execute, passing the object as receiver
	method, err := app.vm.CompileExpression(source)
	if err != nil {
		transcriptWrite("Compile error: " + err.Error())
		return
	}

	_, execErr := app.vm.ExecuteSafe(method, insp.object, nil)
	if execErr != nil {
		transcriptWrite("Error: " + execErr.Error())
		return
	}

	// Refresh entries in case ivars changed
	insp.buildEntries()
	insp.render()
}

func (insp *Inspector) printIt() {
	source := getInspectorEvalSource(insp)
	if source == "" {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			transcriptWrite(fmt.Sprintf("Inspector eval error: %v", r))
		}
	}()

	method, err := app.vm.CompileExpression(source)
	if err != nil {
		transcriptWrite("Compile error: " + err.Error())
		return
	}

	result, execErr := app.vm.ExecuteSafe(method, insp.object, nil)
	if execErr != nil {
		transcriptWrite("Error: " + execErr.Error())
		return
	}

	ps := valuePrintString(app.vm, result)
	cursor := insp.evalEditor.Cursor()
	insp.evalEditor.SetCursor(cursor)
	insp.evalEditor.Buffer.Insert(cursor, " "+ps)
	insp.render()
}

func (insp *Inspector) inspectSelected() {
	val := insp.selectedValue()
	openInspector(insp.vmInst, val)
}

func getInspectorEvalSource(insp *Inspector) string {
	sel := insp.evalEditor.SelectedText()
	if sel != "" {
		return sel
	}
	cursor := insp.evalEditor.Cursor()
	return insp.evalEditor.Buffer.Line(cursor.Line)
}

// --- Utilities ---

func wrapText(text string, maxW int, font *display.Font) []string {
	if maxW <= 0 {
		return []string{text}
	}
	var lines []string
	for _, raw := range strings.Split(text, "\n") {
		if font.MeasureString(raw) <= maxW {
			lines = append(lines, raw)
			continue
		}
		// Simple word wrap
		words := strings.Fields(raw)
		cur := ""
		for _, word := range words {
			test := cur
			if test != "" {
				test += " "
			}
			test += word
			if font.MeasureString(test) > maxW && cur != "" {
				lines = append(lines, cur)
				cur = word
			} else {
				cur = test
			}
		}
		if cur != "" {
			lines = append(lines, cur)
		}
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

func valueClassName(vmInst *vm.VM, val vm.Value) string {
	defer func() { recover() }()
	result := vmInst.Send(val, "class", nil)
	nameResult := vmInst.Send(result, "name", nil)
	if nameResult.IsSymbol() {
		return vmInst.Symbols.Name(nameResult.SymbolID())
	}
	if vm.IsStringValue(nameResult) {
		return vmInst.Registry().GetStringContent(nameResult)
	}
	return "?"
}

func shortDescription(vmInst *vm.VM, val vm.Value) string {
	ps := valuePrintString(vmInst, val)
	if len(ps) > 40 {
		return ps[:40] + "..."
	}
	return ps
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
