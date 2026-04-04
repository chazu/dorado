package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chazu/dorado/go/display"
	"github.com/chazu/maggie/vm"
	"github.com/hajimehoshi/ebiten/v2"
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

	// Class-side toggle
	classSide bool

	// Pane layout
	paneW int
	paneH int

	// Code editor for the bottom pane
	codeEditor *display.TextEditor
	codeForm   *display.Form

	// Dirty tracking
	dirty bool // code pane has unsaved edits
}

// browserRegistry maps windows to Browser instances.
var browserRegistry = map[*display.Window]*Browser{}

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

	b.loadCategories()
	if len(b.categories) > 0 {
		b.selCategory = 0
		b.loadClasses()
	}

	codeH := contentH - 150 // top panes + button bar
	b.codeForm = display.NewForm(contentW, codeH)
	b.codeEditor = display.NewTextEditor(b.codeForm, "")
	b.codeEditor.OnChange = func(_ string) {
		b.dirty = true
	}

	w.Content = display.NewForm(contentW, contentH)

	w.OnContentClick = func(lx, ly int) {
		b.handleClick(lx, ly)
	}

	w.OnKeyEvent = func(e display.Event) bool {
		if e.Type == display.EventKeyDown {
			k := ebiten.Key(e.Key)
			cmd := ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight) ||
				ebiten.IsKeyPressed(ebiten.KeyControl)

			if cmd && k == ebiten.KeyS {
				b.accept()
				return true
			}
		}
		// Forward to code editor
		consumed := b.codeEditor.HandleEvent(e)
		if consumed {
			b.render()
		}
		return consumed
	}

	b.render()
	browserRegistry[w] = b
	app.wm.AddWindow(w)
}

func getBrowserForWindow(w *display.Window) *Browser {
	return browserRegistry[w]
}

func browserMenu(br *Browser) []display.MenuItem {
	return []display.MenuItem{
		{Label: "Accept (⌘S)", Action: func() { br.accept() }},
		display.Separator(),
		{Label: "Senders of...", Action: func() { br.findSenders() }},
		{Label: "Implementors of...", Action: func() { br.findImplementors() }},
		display.Separator(),
		{Label: "Cut", Action: func() {
			br.codeEditor.Cut()
			br.render()
		}},
		{Label: "Copy", Action: func() {
			br.codeEditor.Copy()
		}},
		{Label: "Paste", Action: func() {
			br.codeEditor.Paste()
			br.render()
		}},
	}
}

// --- Accept (compile + save to disk) ---

func (b *Browser) accept() {
	defer func() {
		if r := recover(); r != nil {
			transcriptWrite(fmt.Sprintf("Accept error: %v", r))
		}
	}()

	cls := b.lookupSelectedClass()
	if cls == nil {
		transcriptWrite("Accept: no class selected")
		return
	}

	source := b.codeEditor.Text()
	if strings.TrimSpace(source) == "" {
		return
	}

	// Compile and install the method live
	var selectorName string
	var err error

	if b.classSide {
		selectorName, err = b.compileAndInstallClassMethod(cls, source)
	} else {
		selectorName, err = b.compileAndInstallMethod(cls, source)
	}

	if err != nil {
		transcriptWrite("Compile error: " + err.Error())
		return
	}

	// Persist to disk
	filePath := b.findSourceFile(cls)
	if filePath != "" {
		diskErr := vm.UpdateMethodInFile(filePath, selectorName, source, b.classSide)
		if diskErr != nil {
			// Method might be new — try appending
			diskErr = b.appendMethodToFile(filePath, source)
			if diskErr != nil {
				transcriptWrite(fmt.Sprintf("Warning: compiled OK but failed to save to %s: %v", filePath, diskErr))
			} else {
				transcriptWrite(fmt.Sprintf("Saved (new method) to %s", filePath))
			}
		} else {
			transcriptWrite(fmt.Sprintf("Saved to %s", filePath))
		}
	} else {
		transcriptWrite("Compiled OK (no source file found for " + cls.Name + ")")
	}

	// Refresh browser state
	b.dirty = false
	b.loadProtocols()
	b.loadMethods()

	// Select the newly compiled method
	for i, m := range b.methods {
		if m == selectorName {
			b.selMethod = i
			break
		}
	}
	b.render()
}

func (b *Browser) compileAndInstallMethod(cls *vm.Class, source string) (string, error) {
	source = stripMethodKeyword(source)
	method, err := app.vm.Compile(source, cls)
	if err != nil {
		return "", err
	}
	if method == nil {
		return "", fmt.Errorf("compilation returned nil")
	}
	method.SetClass(cls)
	selectorID := app.vm.Selectors.Intern(method.Name())
	cls.VTable.AddMethod(selectorID, method)
	return method.Name(), nil
}

func (b *Browser) compileAndInstallClassMethod(cls *vm.Class, source string) (string, error) {
	source = stripClassMethodKeyword(source)
	method, err := app.vm.Compile(source, cls)
	if err != nil {
		return "", err
	}
	if method == nil {
		return "", fmt.Errorf("compilation returned nil")
	}
	method.SetClass(cls)
	selectorID := app.vm.Selectors.Intern(method.Name())
	cls.ClassVTable.AddMethod(selectorID, method)
	return method.Name(), nil
}

// stripMethodKeyword removes "method: " prefix if present.
func stripMethodKeyword(source string) string {
	trimmed := strings.TrimSpace(source)
	if strings.HasPrefix(trimmed, "method:") {
		return strings.TrimPrefix(trimmed, "method:")
	}
	return source
}

// stripClassMethodKeyword removes "classMethod: " prefix if present.
func stripClassMethodKeyword(source string) string {
	trimmed := strings.TrimSpace(source)
	if strings.HasPrefix(trimmed, "classMethod:") {
		return strings.TrimPrefix(trimmed, "classMethod:")
	}
	return source
}

// findSourceFile locates the .mag file for a class by scanning source directories.
func (b *Browser) findSourceFile(cls *vm.Class) string {
	// Check src/ directory recursively for ClassName.mag
	candidates := []string{
		cls.Name + ".mag",
	}
	// Also try namespace-qualified paths
	if cls.Namespace != "" {
		candidates = append(candidates, cls.Namespace+"_"+cls.Name+".mag")
	}

	var found string
	filepath.Walk("src", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		for _, candidate := range candidates {
			if base == candidate {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}

// appendMethodToFile adds a new method to the end of a .mag file.
func (b *Browser) appendMethodToFile(filePath string, source string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(data)

	// Ensure file ends with newline
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	// Indent the method source
	source = strings.TrimSpace(source)
	lines := strings.Split(source, "\n")
	var sb strings.Builder
	sb.WriteString("\n")
	for _, line := range lines {
		sb.WriteString("  ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return os.WriteFile(filePath, []byte(content+sb.String()), 0644)
}

// --- Data loading ---

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

	protoSet := map[string]bool{}
	var methods map[int]vm.Method
	if b.classSide {
		methods = cls.ClassVTable.LocalMethods()
	} else {
		methods = cls.VTable.LocalMethods()
	}
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

	var methods map[int]vm.Method
	if b.classSide {
		methods = cls.ClassVTable.LocalMethods()
	} else {
		methods = cls.VTable.LocalMethods()
	}

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

	var method vm.Method
	if b.classSide {
		method = cls.ClassVTable.LocalMethods()[selID]
	} else {
		method = cls.VTable.LocalMethods()[selID]
	}

	if method == nil {
		b.codeEditor.SetText(methodName + " [\n    \"method source not available\"\n]")
		return
	}

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
	b.dirty = false
}

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

// --- Rendering ---

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
	btnBG := display.ColorRGB(200, 200, 200)
	btnActiveBG := display.ColorRGB(120, 120, 180)

	f.Fill(white)

	// --- Four panes at top ---
	paneH := 120
	pw := w / 4

	for i := 1; i < 4; i++ {
		display.DrawVLine(f, pw*i, 0, paneH, gray)
	}
	display.DrawHLine(f, 0, paneH, w, gray)

	drawListPane(f, 0, 0, pw, paneH, b.categories, b.selCategory, lh, black, selBG, selFG, font)
	drawListPane(f, pw, 0, pw, paneH, b.classes, b.selClass, lh, black, selBG, selFG, font)
	drawListPane(f, pw*2, 0, pw, paneH, b.protocols, b.selProtocol, lh, black, selBG, selFG, font)
	drawListPane(f, pw*3, 0, pw, paneH, b.methods, b.selMethod, lh, black, selBG, selFG, font)

	// --- Button bar ---
	btnY := paneH + 2
	btnH := 20
	bx := 4

	// Instance/Class toggle
	instBG := btnBG
	clsBG := btnBG
	instFG := black
	clsFG := black
	if !b.classSide {
		instBG = btnActiveBG
		instFG = white
	} else {
		clsBG = btnActiveBG
		clsFG = white
	}

	instW := font.MeasureString("instance") + 12
	f.FillRectWH(instBG, bx, btnY, instW, btnH)
	display.DrawRect(f, bx, btnY, instW, btnH, gray)
	display.DrawString(f, bx+6, btnY+3, "instance", instFG)
	bx += instW + 2

	clsW := font.MeasureString("class") + 12
	f.FillRectWH(clsBG, bx, btnY, clsW, btnH)
	display.DrawRect(f, bx, btnY, clsW, btnH, gray)
	display.DrawString(f, bx+6, btnY+3, "class", clsFG)
	bx += clsW + 8

	// Accept button
	acceptLabel := "Accept (⌘S)"
	if b.dirty {
		acceptLabel = "● Accept (⌘S)"
	}
	acceptW := font.MeasureString(acceptLabel) + 12
	f.FillRectWH(btnBG, bx, btnY, acceptW, btnH)
	display.DrawRect(f, bx, btnY, acceptW, btnH, gray)
	display.DrawString(f, bx+6, btnY+3, acceptLabel, black)

	// --- Code pane ---
	codeY := btnY + btnH + 2
	codeH := h - codeY
	if b.codeForm == nil || b.codeForm.Height() != codeH || b.codeForm.Width() != w {
		oldText := ""
		if b.codeEditor != nil {
			oldText = b.codeEditor.Text()
		}
		b.codeForm = display.NewForm(w, codeH)
		b.codeEditor = display.NewTextEditor(b.codeForm, oldText)
		b.codeEditor.OnChange = func(_ string) { b.dirty = true }
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

// --- Click handling ---

func (b *Browser) handleClick(localX, localY int) {
	font := display.DefaultFont()
	lh := font.LineHeight() + 2
	paneH := 120
	pw := b.window.Content.Width() / 4

	if localY < paneH {
		// Click in four panes
		pane := localX / pw
		idx := (localY - 4) / lh

		switch pane {
		case 0: // Categories
			if idx >= 0 && idx < len(b.categories) {
				b.selCategory = idx
				b.loadClasses()
				b.codeEditor.SetText("")
				b.dirty = false
			}
		case 1: // Classes
			if idx >= 0 && idx < len(b.classes) {
				b.selClass = idx
				b.loadProtocols()
				b.loadMethods()
				cls := b.lookupSelectedClass()
				if cls != nil {
					b.codeEditor.SetText(classDefinitionText(cls))
					b.dirty = false
				}
			}
		case 2: // Protocols
			if idx >= 0 && idx < len(b.protocols) {
				b.selProtocol = idx
				b.loadMethods()
				b.codeEditor.SetText("")
				b.dirty = false
			}
		case 3: // Methods
			if idx >= 0 && idx < len(b.methods) {
				b.selMethod = idx
				b.loadMethodSource()
			}
		}
		b.render()
		return
	}

	// Button bar
	btnY := paneH + 2
	btnH := 20
	if localY >= btnY && localY < btnY+btnH {
		b.handleButtonClick(localX)
		return
	}

	// Code pane
	codeY := btnY + btnH + 2
	b.codeEditor.HandleClickLocal(localX, localY-codeY, false)
	b.render()
}

func (b *Browser) handleButtonClick(lx int) {
	font := display.DefaultFont()
	bx := 4
	instW := font.MeasureString("instance") + 12
	clsW := font.MeasureString("class") + 12

	if lx >= bx && lx < bx+instW {
		// Instance side
		if b.classSide {
			b.classSide = false
			b.loadProtocols()
			b.loadMethods()
			b.codeEditor.SetText("")
			b.dirty = false
		}
		b.render()
		return
	}
	bx += instW + 2

	if lx >= bx && lx < bx+clsW {
		// Class side
		if !b.classSide {
			b.classSide = true
			b.loadProtocols()
			b.loadMethods()
			b.codeEditor.SetText("")
			b.dirty = false
		}
		b.render()
		return
	}
	bx += clsW + 8

	// Accept button
	acceptLabel := "Accept (⌘S)"
	if b.dirty {
		acceptLabel = "● Accept (⌘S)"
	}
	acceptW := font.MeasureString(acceptLabel) + 12
	if lx >= bx && lx < bx+acceptW {
		b.accept()
		return
	}
}

// --- Senders / Implementors ---

func (b *Browser) findSenders() {
	selector := b.selectedMethodName()
	if selector == "" {
		transcriptWrite("No method selected")
		return
	}

	var results []string
	selID := app.vm.Selectors.Intern(selector)
	for _, cls := range app.vm.Classes.All() {
		for _, method := range cls.VTable.LocalMethods() {
			if cm, ok := method.(*vm.CompiledMethod); ok {
				if methodSendsSelector(cm, selID) {
					results = append(results, fmt.Sprintf("%s >> %s", cls.Name, cm.Name()))
				}
			}
		}
		for _, method := range cls.ClassVTable.LocalMethods() {
			if cm, ok := method.(*vm.CompiledMethod); ok {
				if methodSendsSelector(cm, selID) {
					results = append(results, fmt.Sprintf("%s class >> %s", cls.Name, cm.Name()))
				}
			}
		}
	}
	sort.Strings(results)
	openResultList("Senders of #"+selector, results)
}

func (b *Browser) findImplementors() {
	selector := b.selectedMethodName()
	if selector == "" {
		transcriptWrite("No method selected")
		return
	}

	var results []string
	selID := app.vm.Selectors.Intern(selector)
	for _, cls := range app.vm.Classes.All() {
		if cls.VTable.LocalMethods()[selID] != nil {
			results = append(results, cls.Name)
		}
		if cls.ClassVTable.LocalMethods()[selID] != nil {
			results = append(results, cls.Name+" class")
		}
	}
	sort.Strings(results)
	openResultList("Implementors of #"+selector, results)
}

func (b *Browser) selectedMethodName() string {
	if b.selMethod >= 0 && b.selMethod < len(b.methods) {
		return b.methods[b.selMethod]
	}
	return ""
}

// methodSendsSelector checks if a compiled method's literal table references a selector.
func methodSendsSelector(cm *vm.CompiledMethod, selID int) bool {
	// Check if the selector appears in the bytecode by scanning for send opcodes
	// that reference this selector ID. The selector ID is stored as a 2-byte operand.
	// For simplicity, scan the source text for the selector name.
	if cm.Source != "" {
		name := app.vm.Selectors.Name(selID)
		return strings.Contains(cm.Source, name)
	}
	return false
}

// openResultList creates a window showing a list of search results.
func openResultList(title string, results []string) {
	content := title + "\n" + strings.Repeat("─", 40) + "\n"
	if len(results) == 0 {
		content += "(none found)\n"
	} else {
		content += fmt.Sprintf("(%d found)\n\n", len(results))
		for _, r := range results {
			content += r + "\n"
		}
	}

	w := display.NewWindow(250, 100, 450, 350, title)
	w.SetEditor(content)
	app.wm.AddWindow(w)
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
