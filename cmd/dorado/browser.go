package main

import (
	"sort"
	"strings"

	"github.com/chazu/dorado/go/display"
	"github.com/chazu/maggie/vm"
)

// Browser is the System Browser — a four-pane class/method navigator.
type Browser struct {
	window *display.Window

	// Data
	categories []string
	classes    []string
	protocols  []string
	methods    []string

	// Selection state (indices, -1 = none)
	selCategory int
	selClass    int
	selProtocol int
	selMethod   int

	// Pane layout (computed on render)
	paneW int
	paneH int

	// Code editor for the bottom pane
	codeEditor *display.TextEditor
	codeForm   *display.Form
}

func openBrowser() {
	contentW, contentH := 800, 550

	w := display.NewWindow(60, 40, contentW, contentH, "System Browser")
	b := &Browser{
		window:      w,
		selCategory: -1,
		selClass:    -1,
		selProtocol: -1,
		selMethod:   -1,
	}

	// Load categories (namespaces + "Kernel")
	b.loadCategories()
	if len(b.categories) > 0 {
		b.selCategory = 0
		b.loadClasses()
	}

	// Create code editor form for bottom pane
	codeH := contentH - 130 // top panes take ~130px
	b.codeForm = display.NewForm(contentW, codeH)
	b.codeEditor = display.NewTextEditor(b.codeForm, "")

	// Render into content form
	b.render()

	// Attach custom click handler via the window manager
	// For now, store browser reference in window for lookup
	w.Content = display.NewForm(contentW, contentH)
	b.render()

	// Wire click handler
	w.OnContentClick = func(lx, ly int) {
		b.handleClick(lx, ly)
	}
	// Wire keyboard to code editor
	w.OnKeyEvent = func(e display.Event) bool {
		if b.codeEditor != nil {
			consumed := b.codeEditor.HandleEvent(e)
			if consumed {
				b.render()
			}
			return consumed
		}
		return false
	}

	app.wm.AddWindow(w)
}

func (b *Browser) loadCategories() {
	catSet := map[string]bool{}
	allClasses := app.vm.Classes.All()
	for _, cls := range allClasses {
		ns := cls.Namespace
		if ns == "" {
			ns = "Kernel"
		}
		catSet[ns] = true
	}
	b.categories = make([]string, 0, len(catSet))
	for cat := range catSet {
		b.categories = append(b.categories, cat)
	}
	sort.Strings(b.categories)
}

func (b *Browser) loadClasses() {
	b.classes = nil
	if b.selCategory < 0 || b.selCategory >= len(b.categories) {
		return
	}
	cat := b.categories[b.selCategory]
	allClasses := app.vm.Classes.All()
	seen := map[string]bool{}
	for _, cls := range allClasses {
		ns := cls.Namespace
		if ns == "" {
			ns = "Kernel"
		}
		if ns == cat && !seen[cls.Name] {
			b.classes = append(b.classes, cls.Name)
			seen[cls.Name] = true
		}
	}
	sort.Strings(b.classes)
	b.selClass = -1
	b.protocols = nil
	b.methods = nil
	b.selProtocol = -1
	b.selMethod = -1
}

func (b *Browser) loadProtocols() {
	b.protocols = nil
	if b.selClass < 0 || b.selClass >= len(b.classes) {
		return
	}
	cls := b.lookupSelectedClass()
	if cls == nil {
		return
	}

	// Collect protocol names from method categories
	protoSet := map[string]bool{}
	methods := cls.VTable.LocalMethods()
	for selID := range methods {
		name := app.vm.Selectors.Name(selID)
		proto := categorizeMethod(name)
		protoSet[proto] = true
	}
	for proto := range protoSet {
		b.protocols = append(b.protocols, proto)
	}
	sort.Strings(b.protocols)
	b.selProtocol = -1
	b.methods = nil
	b.selMethod = -1
}

func (b *Browser) loadMethods() {
	b.methods = nil
	if b.selClass < 0 {
		return
	}
	cls := b.lookupSelectedClass()
	if cls == nil {
		return
	}

	methods := cls.VTable.LocalMethods()
	selProto := ""
	if b.selProtocol >= 0 && b.selProtocol < len(b.protocols) {
		selProto = b.protocols[b.selProtocol]
	}

	for selID := range methods {
		name := app.vm.Selectors.Name(selID)
		if selProto == "" || categorizeMethod(name) == selProto {
			b.methods = append(b.methods, name)
		}
	}
	sort.Strings(b.methods)
	b.selMethod = -1
}

func (b *Browser) lookupSelectedClass() *vm.Class {
	if b.selClass < 0 || b.selClass >= len(b.classes) {
		return nil
	}
	name := b.classes[b.selClass]
	cat := ""
	if b.selCategory >= 0 && b.selCategory < len(b.categories) {
		cat = b.categories[b.selCategory]
	}

	// Try qualified name first
	if cat != "" && cat != "Kernel" {
		cls := app.vm.Classes.Lookup(cat + "::" + name)
		if cls != nil {
			return cls
		}
	}
	return app.vm.Classes.Lookup(name)
}

func (b *Browser) loadMethodSource() {
	if b.selMethod < 0 || b.selMethod >= len(b.methods) {
		b.codeEditor.SetText("")
		return
	}
	cls := b.lookupSelectedClass()
	if cls == nil {
		return
	}
	methodName := b.methods[b.selMethod]
	selID := app.vm.Selectors.Intern(methodName)
	method := cls.VTable.LocalMethods()[selID]
	if method == nil {
		b.codeEditor.SetText(methodName + " [\n    \"method source not available\"\n]")
		return
	}

	// Try to get source from CompiledMethod
	if cm, ok := method.(*vm.CompiledMethod); ok && cm.Source != "" {
		b.codeEditor.SetText(cm.Source)
	} else {
		doc := vm.MethodDocString(method)
		if doc != "" {
			b.codeEditor.SetText(methodName + " [\n    \"" + doc + "\"\n    <primitive>\n]")
		} else {
			b.codeEditor.SetText(methodName + " [\n    <primitive>\n]")
		}
	}
}

// categorizeMethod returns a simple protocol category based on the method name.
func categorizeMethod(name string) string {
	switch {
	case strings.HasPrefix(name, "is") || strings.HasPrefix(name, "not"):
		return "testing"
	case strings.HasPrefix(name, "print") || name == "displayString":
		return "printing"
	case strings.HasPrefix(name, "as"):
		return "converting"
	case strings.HasPrefix(name, "do:") || strings.HasPrefix(name, "collect:") ||
		strings.HasPrefix(name, "select:") || strings.HasPrefix(name, "reject:") ||
		strings.HasPrefix(name, "detect:") || strings.HasPrefix(name, "inject:"):
		return "enumerating"
	case strings.HasSuffix(name, ":") && !strings.Contains(name[:len(name)-1], ":"):
		return "accessing"
	default:
		return "other"
	}
}

// render draws the browser content into the window's content Form.
func (b *Browser) render() {
	f := b.window.Content
	w := f.Width()
	h := f.Height()
	font := display.DefaultFont()
	lh := font.LineHeight() + 2

	white := display.ColorRGB(255, 255, 255)
	black := display.ColorRGB(0, 0, 0)
	gray := display.ColorRGB(180, 180, 180)
	selBG := display.ColorRGB(40, 40, 120)
	selFG := display.ColorRGB(255, 255, 255)

	f.Fill(white)

	// Four panes at top, each w/4 wide, 120px tall
	paneH := 120
	pw := w / 4

	// Draw pane separators
	for i := 1; i < 4; i++ {
		for y := 0; y < paneH; y++ {
			f.SetPixelAt(pw*i, y, gray)
		}
	}
	for x := 0; x < w; x++ {
		f.SetPixelAt(x, paneH, gray)
	}

	// Draw pane contents
	drawListPane(f, 0, 0, pw, paneH, b.categories, b.selCategory, lh, black, selBG, selFG, font)
	drawListPane(f, pw, 0, pw, paneH, b.classes, b.selClass, lh, black, selBG, selFG, font)
	drawListPane(f, pw*2, 0, pw, paneH, b.protocols, b.selProtocol, lh, black, selBG, selFG, font)
	drawListPane(f, pw*3, 0, pw, paneH, b.methods, b.selMethod, lh, black, selBG, selFG, font)

	// Code pane below
	codeY := paneH + 1
	codeH := h - codeY
	if b.codeForm == nil || b.codeForm.Height() != codeH || b.codeForm.Width() != w {
		b.codeForm = display.NewForm(w, codeH)
		b.codeEditor = display.NewTextEditor(b.codeForm, b.codeEditor.Text())
	}
	b.codeEditor.Render()
	display.CopyBits(f, 0, codeY, b.codeForm)

	b.window.MarkDirty()
}

func drawListPane(f *display.Form, x, y, w, h int, items []string, sel int, lh int, fg, selBG, selFG uint32, font *display.Font) {
	pad := 4
	for i, item := range items {
		iy := y + pad + i*lh
		if iy+lh > y+h {
			break
		}
		if i == sel {
			f.FillRectWH(selBG, x+1, iy, w-2, lh)
			display.DrawStringFont(f, x+pad, iy, item, selFG, font)
		} else {
			display.DrawStringFont(f, x+pad, iy, item, fg, font)
		}
	}
}

// handleBrowserClick processes a click in a browser window's content area.
func (b *Browser) handleClick(localX, localY int) {
	font := display.DefaultFont()
	lh := font.LineHeight() + 2
	paneH := 120
	pw := b.window.Content.Width() / 4

	if localY < paneH {
		// Click in one of the four panes
		pane := localX / pw
		idx := (localY - 4) / lh

		switch pane {
		case 0: // Categories
			if idx >= 0 && idx < len(b.categories) {
				b.selCategory = idx
				b.loadClasses()
				b.codeEditor.SetText("")
			}
		case 1: // Classes
			if idx >= 0 && idx < len(b.classes) {
				b.selClass = idx
				b.loadProtocols()
				b.loadMethods()
				// Show class definition
				cls := b.lookupSelectedClass()
				if cls != nil {
					b.codeEditor.SetText(classDefinitionText(cls))
				}
			}
		case 2: // Protocols
			if idx >= 0 && idx < len(b.protocols) {
				b.selProtocol = idx
				b.loadMethods()
				b.codeEditor.SetText("")
			}
		case 3: // Methods
			if idx >= 0 && idx < len(b.methods) {
				b.selMethod = idx
				b.loadMethodSource()
			}
		}
		b.render()
	} else {
		// Click in code pane
		codeY := paneH + 1
		b.codeEditor.HandleClickLocal(localX, localY-codeY, false)
		b.render()
	}
}

func classDefinitionText(cls *vm.Class) string {
	var sb strings.Builder
	sb.WriteString(cls.Name)
	sb.WriteString(" subclass: ")
	if cls.Superclass != nil {
		sb.WriteString(cls.Superclass.Name)
	} else {
		sb.WriteString("nil")
	}
	sb.WriteString("\n")
	ivars := cls.InstVars
	if len(ivars) > 0 {
		sb.WriteString("  instanceVars: ")
		sb.WriteString(strings.Join(ivars, " "))
		sb.WriteString("\n")
	}
	return sb.String()
}
