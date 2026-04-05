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

	w.OnContentClick = func(lx, ly int) {
		insp.handleClick(lx, ly)
	}

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

func (insp *Inspector) buildEntries() {
	insp.entries = nil
	val := insp.object
	v := insp.vmInst

	// Try to get instance variables via the object's slots
	if val.IsObject() {
		obj := vm.ObjectFromValue(val)
		if obj != nil {
			class := v.GetClassFromValue(val)
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
			// If no named ivars but has slots, show indexed slots
			if len(insp.entries) == 0 {
				for i := 0; i < obj.NumSlots() && i < 50; i++ {
					insp.entries = append(insp.entries, inspectorEntry{
						Name:  fmt.Sprintf("[%d]", i),
						Value: obj.GetSlot(i),
					})
				}
			}
		}
	}

	// Use Maggie introspection to get more information
	// Try sending instVarNames and instVarAt: for richer inspection
	if len(insp.entries) == 0 {
		insp.addIntrospectedEntries(val)
	}

	// Always add useful derived properties based on the type
	className := valueClassName(v, val)
	switch className {
	case "SmallInteger", "BigInteger":
		insp.addComputedEntry("decimal", val)
		insp.addSendEntry(val, "printString", "printString")
		insp.addSendEntry(val, "even", "even")
		insp.addSendEntry(val, "odd", "odd")
		insp.addSendEntry(val, "class", "class")
	case "Float":
		insp.addSendEntry(val, "printString", "printString")
		insp.addSendEntry(val, "class", "class")
	case "String":
		insp.addSendEntry(val, "size", "size")
		insp.addSendEntry(val, "class", "class")
		// Show the string contents
		if vm.IsStringValue(val) {
			str := v.Registry().GetStringContent(val)
			insp.entries = append(insp.entries, inspectorEntry{
				Name: "contents", Value: val,
			})
			// Show individual characters for short strings
			if len([]rune(str)) <= 20 {
				for i, ch := range str {
					insp.entries = append(insp.entries, inspectorEntry{
						Name:  fmt.Sprintf("[%d]", i),
						Value: v.Registry().NewStringValue(string(ch)),
					})
				}
			}
		}
	case "Symbol":
		insp.addSendEntry(val, "class", "class")
	case "Array":
		insp.addSendEntry(val, "size", "size")
		insp.addSendEntry(val, "class", "class")
		insp.addArrayElements(val)
	case "ArrayList":
		insp.addSendEntry(val, "size", "size")
		insp.addSendEntry(val, "class", "class")
		insp.addArrayListElements(val)
	case "Dictionary":
		insp.addSendEntry(val, "size", "size")
		insp.addSendEntry(val, "class", "class")
		insp.addDictionaryEntries(val)
	case "Set":
		insp.addSendEntry(val, "size", "size")
		insp.addSendEntry(val, "class", "class")
	case "True", "False":
		insp.addSendEntry(val, "class", "class")
	case "UndefinedObject":
		// nil — nothing much to show
	default:
		// For unknown types, try common messages
		if len(insp.entries) == 0 {
			insp.addSendEntry(val, "class", "class")
			insp.addSendEntry(val, "printString", "printString")
		}
	}
}

func (insp *Inspector) addIntrospectedEntries(val vm.Value) {
	defer func() { recover() }()

	// Try instVarSize to see if there are slots to inspect
	sizeResult := insp.vmInst.Send(val, "instVarSize", nil)
	if sizeResult.IsSmallInt() {
		n := int(sizeResult.SmallInt())
		for i := 0; i < n && i < 50; i++ {
			slotResult := insp.vmInst.Send(val, "instVarAt:", []vm.Value{vm.FromSmallInt(int64(i))})
			insp.entries = append(insp.entries, inspectorEntry{
				Name:  fmt.Sprintf("slot[%d]", i),
				Value: slotResult,
			})
		}
	}
}

func (insp *Inspector) addSendEntry(val vm.Value, selector, label string) {
	defer func() { recover() }()
	result := insp.vmInst.Send(val, selector, nil)
	insp.entries = append(insp.entries, inspectorEntry{
		Name:  label,
		Value: result,
	})
}

func (insp *Inspector) addComputedEntry(label string, val vm.Value) {
	insp.entries = append(insp.entries, inspectorEntry{
		Name:  label,
		Value: val,
	})
}

func (insp *Inspector) addArrayElements(val vm.Value) {
	defer func() { recover() }()
	sizeVal := insp.vmInst.Send(val, "size", nil)
	if !sizeVal.IsSmallInt() {
		return
	}
	n := int(sizeVal.SmallInt())
	if n > 50 {
		n = 50
	}
	for i := 0; i < n; i++ {
		elem := insp.vmInst.Send(val, "at:", []vm.Value{vm.FromSmallInt(int64(i))})
		insp.entries = append(insp.entries, inspectorEntry{
			Name:  fmt.Sprintf("[%d]", i),
			Value: elem,
		})
	}
}

func (insp *Inspector) addArrayListElements(val vm.Value) {
	defer func() { recover() }()
	sizeVal := insp.vmInst.Send(val, "size", nil)
	if !sizeVal.IsSmallInt() {
		return
	}
	n := int(sizeVal.SmallInt())
	if n > 50 {
		n = 50
	}
	for i := 0; i < n; i++ {
		elem := insp.vmInst.Send(val, "at:", []vm.Value{vm.FromSmallInt(int64(i))})
		insp.entries = append(insp.entries, inspectorEntry{
			Name:  fmt.Sprintf("[%d]", i),
			Value: elem,
		})
	}
}

func (insp *Inspector) addDictionaryEntries(val vm.Value) {
	defer func() { recover() }()
	keysVal := insp.vmInst.Send(val, "keys", nil)
	sizeVal := insp.vmInst.Send(keysVal, "size", nil)
	if !sizeVal.IsSmallInt() {
		return
	}
	n := int(sizeVal.SmallInt())
	if n > 50 {
		n = 50
	}
	for i := 0; i < n; i++ {
		key := insp.vmInst.Send(keysVal, "at:", []vm.Value{vm.FromSmallInt(int64(i))})
		keyStr := valuePrintString(insp.vmInst, key)
		value := insp.vmInst.Send(val, "at:", []vm.Value{key})
		insp.entries = append(insp.entries, inspectorEntry{
			Name:  keyStr,
			Value: value,
		})
	}
}

func (insp *Inspector) selectedValue() vm.Value {
	if insp.selIndex < 0 || insp.selIndex >= len(insp.entries) {
		return insp.object
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

	display.DrawHLine(f, 0, y, listW, gray)
	y += 2

	// Instance variables / entries
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

	// Vertical separator
	display.DrawVLine(f, listW, 0, insp.listPaneH, gray)

	// --- Right pane: selected value ---
	selVal := insp.selectedValue()
	className := valueClassName(insp.vmInst, selVal)
	printStr := valuePrintString(insp.vmInst, selVal)

	vy := 4
	display.DrawString(f, valueX+8, vy, className, gray)
	vy += lh

	lines := wrapText(printStr, w-listW-20, font)
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

	display.DrawString(f, 4, sepY+2, "evaluate (in context of self):", gray)

	// --- Eval pane ---
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

func (insp *Inspector) handleClick(lx, ly int) {
	font := display.DefaultFont()
	lh := font.LineHeight() + 2
	listStartY := 4

	if ly < insp.listPaneH && lx < insp.listPaneW {
		// Check "self" row
		if ly >= listStartY && ly < listStartY+lh {
			insp.selIndex = -1
			insp.render()
			return
		}
		y := listStartY + lh + 2 // skip separator

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
