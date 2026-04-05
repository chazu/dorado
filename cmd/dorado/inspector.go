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
	object   vm.Value
	entries  []inspectorEntry
	selIndex int // -1 = self

	listPaneW int
	listPaneH int

	evalForm   *display.Form
	evalEditor *display.TextEditor
}

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
		{Label: "Cut", Action: func() { insp.evalEditor.Cut(); insp.render() }},
		{Label: "Copy", Action: func() { insp.evalEditor.Copy() }},
		{Label: "Paste", Action: func() { insp.evalEditor.Paste(); insp.render() }},
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
		selIndex:  -1,
		listPaneW: 180,
		listPaneH: 260,
	}

	insp.buildEntries()

	evalH := contentH - insp.listPaneH - 4
	insp.evalForm = display.NewForm(contentW, evalH)
	insp.evalEditor = display.NewTextEditor(insp.evalForm, "")
	insp.evalEditor.SyntaxHighlight = true

	w.Content = display.NewForm(contentW, contentH)

	w.OnContentClick = func(lx, ly int) { insp.handleClick(lx, ly) }
	w.OnKeyEvent = func(e display.Event) bool {
		consumed := insp.evalEditor.HandleEvent(e)
		if consumed {
			insp.render()
		}
		return consumed
	}

	insp.render()
	inspectorRegistry[w] = insp
	app.wm.AddWindow(w)
}

// buildEntries uses generic Maggie introspection — no type-switching.
// Every object responds to: class, instVarSize, instVarAt:
// Every class responds to: allInstVarNames
func (insp *Inspector) buildEntries() {
	insp.entries = nil
	val := insp.object
	v := insp.vmInst

	// Get the class and instance variable names via message sends
	classVal := safeSend(v, val, "class", nil)
	ivarNames := safeSend(v, classVal, "allInstVarNames", nil)

	// Get instance variable count
	ivarSize := safeSendInt(v, val, "instVarSize")

	// Get the ivar name list as Go strings
	var names []string
	if ivarNames != vm.Nil {
		nameCount := safeSendInt(v, ivarNames, "size")
		for i := 0; i < nameCount; i++ {
			nameVal := safeSend(v, ivarNames, "at:", []vm.Value{vm.FromSmallInt(int64(i))})
			names = append(names, valuePrintString(v, nameVal))
		}
	}

	// Build entries: named ivars first, then any extra indexed slots
	for i := 0; i < ivarSize; i++ {
		name := fmt.Sprintf("[%d]", i)
		if i < len(names) {
			name = names[i]
		}
		slotVal := safeSend(v, val, "instVarAt:", []vm.Value{vm.FromSmallInt(int64(i))})
		insp.entries = append(insp.entries, inspectorEntry{
			Name:  name,
			Value: slotVal,
		})
	}

	// For indexable collections (respond to size and at:), show elements
	// Only if we didn't already get slots above
	if ivarSize == 0 {
		collSize := safeSendInt(v, val, "size")
		if collSize > 0 {
			// Cap at 100 elements
			if collSize > 100 {
				collSize = 100
			}
			for i := 0; i < collSize; i++ {
				elem := safeSend(v, val, "at:", []vm.Value{vm.FromSmallInt(int64(i))})
				insp.entries = append(insp.entries, inspectorEntry{
					Name:  fmt.Sprintf("[%d]", i),
					Value: elem,
				})
			}
		}
	}
}

// safeSend sends a message and recovers from panics, returning Nil on failure.
func safeSend(v *vm.VM, recv vm.Value, selector string, args []vm.Value) (result vm.Value) {
	defer func() {
		if r := recover(); r != nil {
			result = vm.Nil
		}
	}()
	return v.Send(recv, selector, args)
}

// safeSendInt sends a message expecting a SmallInteger result, returns 0 on failure.
func safeSendInt(v *vm.VM, recv vm.Value, selector string) int {
	result := safeSend(v, recv, selector, nil)
	if result.IsSmallInt() {
		return int(result.SmallInt())
	}
	return 0
}

func (insp *Inspector) selectedValue() vm.Value {
	if insp.selIndex < 0 || insp.selIndex >= len(insp.entries) {
		return insp.object
	}
	return insp.entries[insp.selIndex].Value
}

// --- Rendering ---

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

	// Left pane: self + entries
	y := 4
	if insp.selIndex == -1 {
		f.FillRectWH(selBG, 0, y, listW, lh)
		display.DrawString(f, 8, y, "self", selFG)
	} else {
		display.DrawString(f, 8, y, "self", black)
	}
	y += lh
	display.DrawHLine(f, 0, y, listW, gray)
	y += 2

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

	display.DrawVLine(f, listW, 0, insp.listPaneH, gray)

	// Right pane: selected value
	selVal := insp.selectedValue()
	className := valueClassName(insp.vmInst, selVal)
	printStr := valuePrintString(insp.vmInst, selVal)

	vy := 4
	display.DrawString(f, valueX+8, vy, className, gray)
	vy += lh
	for _, line := range wrapText(printStr, w-listW-20, font) {
		if vy+lh > insp.listPaneH {
			break
		}
		display.DrawString(f, valueX+8, vy, line, black)
		vy += lh
	}

	// Separator + eval pane
	sepY := insp.listPaneH
	display.DrawHLine(f, 0, sepY, w, gray)
	display.DrawString(f, 4, sepY+2, "evaluate (in context of self):", gray)

	evalY := sepY + lh + 4
	evalH := f.Height() - evalY
	if insp.evalForm.Height() != evalH || insp.evalForm.Width() != w {
		oldText := insp.evalEditor.Text()
		insp.evalForm = display.NewForm(w, evalH)
		insp.evalEditor = display.NewTextEditor(insp.evalForm, oldText)
		insp.evalEditor.SyntaxHighlight = true
	}
	insp.evalEditor.Render()
	display.CopyBits(f, 0, evalY, insp.evalForm)

	insp.window.MarkDirty()
}

// --- Click handling ---

func (insp *Inspector) handleClick(lx, ly int) {
	font := display.DefaultFont()
	lh := font.LineHeight() + 2

	if ly < insp.listPaneH && lx < insp.listPaneW {
		y := 4
		if ly >= y && ly < y+lh {
			insp.selIndex = -1
			insp.render()
			return
		}
		y += lh + 2
		for i := range insp.entries {
			if ly >= y && ly < y+lh {
				insp.selIndex = i
				insp.render()
				return
			}
			y += lh
		}
	} else if ly >= insp.listPaneH {
		evalY := insp.listPaneH + (font.LineHeight() + 2) + 4
		insp.evalEditor.HandleClickLocal(lx, ly-evalY, false)
		insp.render()
	}
}

// --- Eval ---

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
	openInspector(insp.vmInst, insp.selectedValue())
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
	result := safeSend(vmInst, val, "class", nil)
	nameResult := safeSend(vmInst, result, "name", nil)
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
