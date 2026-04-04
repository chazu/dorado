// Command dorado is the Dorado ST-80 IDE for Maggie.
// It embeds the Maggie VM and the display layer, wiring them together
// so that Maggie code can drive the graphical environment.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/chazu/dorado/go/display"
	"github.com/chazu/maggie/compiler"
	"github.com/chazu/maggie/pipeline"
	"github.com/chazu/maggie/vm"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	screenW = 1280
	screenH = 960
)

// app holds the global application state.
var app struct {
	vm      *vm.VM
	screen  *display.Form
	wm      *display.WindowManager
	backend *display.EbitengineBackend
}

func main() {
	// Bootstrap the Maggie VM
	vmInst := vm.NewVM()

	imagePath := findMaggieImage()
	if imagePath != "" {
		if err := vmInst.LoadImage(imagePath); err != nil {
			log.Fatalf("Failed to load Maggie image: %v", err)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Warning: no Maggie image found, running without base classes")
	}

	vmInst.UseGoCompiler(compiler.Compile)
	vmInst.SetFileInFunc(func(v *vm.VM, source, sourcePath, nsOverride string, verbose bool) (int, error) {
		p := &pipeline.Pipeline{VM: v}
		return p.CompileSourceFile(source, sourcePath, nsOverride)
	})
	vmInst.SetFileInBatchFunc(func(v *vm.VM, dirPath string, verbose bool) (int, error) {
		p := &pipeline.Pipeline{VM: v}
		return p.CompilePath(dirPath + "/...")
	})

	// Create display
	screen := display.NewForm(screenW, screenH)
	backend := display.NewEbitengineBackend(screen)
	wm := display.NewWindowManager(screen)

	app.vm = vmInst
	app.screen = screen
	app.wm = wm
	app.backend = backend

	// Register display primitives
	registerDisplayPrimitives(vmInst, wm)

	// Load Dorado Maggie source files
	if srcDir := findSourceDir(); srcDir != "" {
		p := &pipeline.Pipeline{VM: vmInst}
		methods, err := p.CompilePath(srcDir + "/...")
		if err != nil {
			log.Fatalf("Failed to compile Dorado sources: %v", err)
		}
		if methods > 0 {
			fmt.Printf("Dorado: compiled %d methods\n", methods)
		}
	}

	// Run Main.start if it exists
	runMaggieMain(vmInst)

	// Create initial windows if Main.start didn't create any
	if len(wm.Windows()) == 0 {
		createDefaultLayout(wm)
	}

	// Wire up menus
	wm.WorldMenuFunc = worldMenu
	wm.WindowMenuFunc = windowMenu

	colorBG := display.ColorRGB(168, 168, 168)

	backend.OnUpdate = func() {
		for _, e := range backend.PollEvents() {
			// Handle Cmd+D (do it) and Cmd+P (print it) globally
			if e.Type == display.EventKeyDown && !wm.HasMenu() {
				if handleGlobalShortcut(e) {
					continue
				}
			}
			wm.HandleEvent(e)
		}

		// Refresh debugger if active
		if debugger != nil && debugger.active {
			refreshDebugger()
		}

		wm.Composite(colorBG)
	}

	if err := backend.Run(); err != nil {
		log.Fatal(err)
	}
}

func runMaggieMain(vmInst *vm.VM) {
	mainClass := vmInst.Classes.Lookup("Dorado::Main")
	if mainClass == nil {
		mainClass = vmInst.Classes.Lookup("Main")
	}
	if mainClass == nil {
		return
	}
	selectorID := vmInst.Selectors.Intern("start")
	if mainClass.ClassVTable.Lookup(selectorID) == nil {
		return
	}
	qualifiedName := mainClass.FullName()
	classValue := vmInst.Symbols.SymbolValue(qualifiedName)
	func() {
		defer func() {
			if r := recover(); r != nil {
				transcriptWrite(fmt.Sprintf("Error in Main.start: %v", r))
			}
		}()
		vmInst.Send(classValue, "start", nil)
	}()
}

func createDefaultLayout(wm *display.WindowManager) {
	// Transcript
	transcript := display.NewWindow(700, 80, 500, 300, "Transcript")
	transcript.SetEditor("Dorado started.\nReady.\n")
	wm.AddWindow(transcript)

	// Workspace
	ws := display.NewWindow(40, 40, 600, 500, "Workspace")
	ws.SetEditor(`"Welcome to Dorado — the ST-80 IDE for Maggie."
"Select an expression and press Cmd+D to evaluate (Do it),"
"or Cmd+P to evaluate and print the result (Print it)."

3 + 4.

42 factorial.

#(1 2 3 4 5) select: [:x | x even].

'Hello, ' , 'Dorado!'.
`)
	wm.AddWindow(ws)
}

// --- Menus ---

func worldMenu(x, y int) []display.MenuItem {
	return []display.MenuItem{
		{Label: "New Workspace", Action: func() {
			w := display.NewWindow(x, y, 500, 380, "Workspace")
			w.SetEditor("")
			app.wm.AddWindow(w)
		}},
		{Label: "System Browser", Action: func() {
			openBrowser()
		}},
		{Label: "New Transcript", Action: func() {
			w := display.NewWindow(x, y, 500, 300, "Transcript")
			w.SetEditor("")
			app.wm.AddWindow(w)
		}},
		display.Separator(),
		{Label: "Debugger", Action: func() {
			openDebugger()
		}},
		display.Separator(),
		{Label: "About Dorado", Action: func() {
			transcriptWrite("Dorado — ST-80 IDE for Maggie")
		}},
	}
}

func windowMenu(w *display.Window, x, y int) []display.MenuItem {
	// Check if this is an inspector window
	if insp := getInspectorForWindow(w); insp != nil {
		return inspectorMenu(insp)
	}

	items := []display.MenuItem{
		{Label: "Do it  (⌘D)", Action: func() { doIt(w) }},
		{Label: "Print it  (⌘P)", Action: func() { printIt(w) }},
		{Label: "Inspect it  (⌘I)", Action: func() { inspectIt(w) }},
		display.Separator(),
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
				// Manually select all
				buf := w.Editor.Buffer
				lastLine := buf.LineCount() - 1
				lastCol := len([]rune(buf.Line(lastLine)))
				w.Editor.HandleClickLocal(0, 0, false)
				w.Editor.HandleClickLocal(w.Content.Width(), w.Content.Height(), true)
				_ = lastCol // ensure we trigger full selection
				w.MarkDirty()
			}
		}},
	}
	return items
}

// --- Do it / Print it ---

func doIt(w *display.Window) {
	defer func() {
		if r := recover(); r != nil {
			transcriptWrite(fmt.Sprintf("Do it error (panic): %v", r))
		}
	}()

	if w == nil || w.Editor == nil {
		return
	}
	source := getEvalSource(w.Editor)
	if source == "" {
		return
	}

	_, _, err := evalExpression(app.vm, source)
	if err != nil {
		transcriptWrite("Error: " + err.Error())
	}
	w.MarkDirty()
}

func inspectIt(w *display.Window) {
	defer func() {
		if r := recover(); r != nil {
			transcriptWrite(fmt.Sprintf("Inspect error (panic): %v", r))
		}
	}()

	if w == nil || w.Editor == nil {
		transcriptWrite("Inspect: no editor in focused window")
		return
	}
	source := getEvalSource(w.Editor)
	if source == "" {
		transcriptWrite("Inspect: no text selected or current line empty")
		return
	}

	transcriptWrite("Inspecting: " + source)
	result, _, err := evalExpression(app.vm, source)
	if err != nil {
		transcriptWrite("Error: " + err.Error())
		return
	}

	openInspector(app.vm, result)
}

func printIt(w *display.Window) {
	defer func() {
		if r := recover(); r != nil {
			transcriptWrite(fmt.Sprintf("Print it error (panic): %v", r))
		}
	}()

	if w == nil || w.Editor == nil {
		return
	}
	source := getEvalSource(w.Editor)
	if source == "" {
		return
	}

	_, printStr, err := evalExpression(app.vm, source)
	if err != nil {
		transcriptWrite("Error: " + err.Error())
		return
	}

	// Insert result after selection (or cursor)
	te := w.Editor
	cursor := te.Cursor()
	te.SetCursor(cursor)
	te.Buffer.Insert(cursor, " "+printStr)
	w.MarkDirty()
}

// getEvalSource returns the selected text, or the current line if no selection.
func getEvalSource(te *display.TextEditor) string {
	sel := te.SelectedText()
	if sel != "" {
		return sel
	}
	// No selection: use the current line
	cursor := te.Cursor()
	return te.Buffer.Line(cursor.Line)
}

// --- Global keyboard shortcuts ---

func handleGlobalShortcut(e display.Event) bool {
	k := ebiten.Key(e.Key)
	cmd := ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight) ||
		ebiten.IsKeyPressed(ebiten.KeyControl)
	if !cmd {
		return false
	}

	w := app.wm.Focused()
	if w == nil {
		return false
	}

	// Check if focused window is an inspector
	if insp := getInspectorForWindow(w); insp != nil {
		switch k {
		case ebiten.KeyD:
			insp.doIt()
			return true
		case ebiten.KeyP:
			insp.printIt()
			return true
		case ebiten.KeyI:
			insp.inspectSelected()
			return true
		}
		return false
	}

	if w.Editor == nil {
		return false
	}

	switch k {
	case ebiten.KeyD:
		doIt(w)
		return true
	case ebiten.KeyP:
		printIt(w)
		return true
	case ebiten.KeyI:
		inspectIt(w)
		return true
	}
	return false
}

// --- Transcript ---

func transcriptWrite(text string) {
	wm := app.wm
	var transcript *display.Window
	for _, w := range wm.Windows() {
		if w.Title == "Transcript" && !w.Closed {
			transcript = w
			break
		}
	}
	if transcript == nil {
		transcript = display.NewWindow(700, 80, 500, 300, "Transcript")
		transcript.SetEditor("")
		wm.AddWindow(transcript)
	}
	if transcript.Editor != nil {
		existing := transcript.Editor.Text()
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		transcript.Editor.SetText(existing + text)
		transcript.MarkDirty()
	}
}

// --- Paths ---

func findSourceDir() string {
	if info, err := os.Stat("src"); err == nil && info.IsDir() {
		return "src"
	}
	return ""
}

func findMaggieImage() string {
	paths := []string{
		"maggie.image",
		os.ExpandEnv("$HOME/dev/go/maggie/cmd/mag/maggie.image"),
	}
	if magPath, err := os.Executable(); err == nil {
		dir := magPath[:strings.LastIndex(magPath, "/")]
		paths = append(paths, dir+"/maggie.image")
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// --- Display primitives for Maggie ---

func registerDisplayPrimitives(vmInst *vm.VM, wm *display.WindowManager) {
	displayClass := vmInst.Classes.Lookup("DoradoDisplay")
	if displayClass == nil {
		displayClass = vm.NewClass("DoradoDisplay", vmInst.ObjectClass)
		vmInst.Classes.Register(displayClass)
	}
	vmInst.SetGlobal("DoradoDisplay", vmInst.ClassValue(displayClass))

	displayClass.AddClassMethod1(vmInst.Selectors, "newWindow:", func(vmPtr interface{}, recv vm.Value, titleVal vm.Value) vm.Value {
		v := vmPtr.(*vm.VM)
		title := v.Registry().GetStringContent(titleVal)
		w := display.NewWindow(100, 100, 500, 380, title)
		w.SetEditor("")
		wm.AddWindow(w)
		return vm.FromSmallInt(int64(len(wm.Windows()) - 1))
	})

	displayClass.AddClassMethod0(vmInst.Selectors, "windowCount", func(_ interface{}, _ vm.Value) vm.Value {
		return vm.FromSmallInt(int64(len(wm.Windows())))
	})

	displayClass.AddClassMethod0(vmInst.Selectors, "screenWidth", func(_ interface{}, _ vm.Value) vm.Value {
		return vm.FromSmallInt(int64(app.screen.Width()))
	})
	displayClass.AddClassMethod0(vmInst.Selectors, "screenHeight", func(_ interface{}, _ vm.Value) vm.Value {
		return vm.FromSmallInt(int64(app.screen.Height()))
	})

	displayClass.AddClassMethod1(vmInst.Selectors, "transcript:", func(vmPtr interface{}, recv vm.Value, textVal vm.Value) vm.Value {
		v := vmPtr.(*vm.VM)
		text := v.Registry().GetStringContent(textVal)
		transcriptWrite(text)
		return vm.Nil
	})
}
