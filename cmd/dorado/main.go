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
)

const (
	screenW = 1280
	screenH = 960
)

func main() {
	// Bootstrap the Maggie VM
	vmInst := vm.NewVM()

	// Load the base Maggie image (provides String, Array, etc.)
	imagePath := findMaggieImage()
	if imagePath != "" {
		if err := vmInst.LoadImage(imagePath); err != nil {
			log.Fatalf("Failed to load Maggie image: %v", err)
		}
		fmt.Printf("Loaded Maggie image from %s\n", imagePath)
	} else {
		fmt.Println("Warning: no Maggie image found, running without base classes")
	}

	vmInst.UseGoCompiler(compiler.Compile)

	// Wire up FileIn so Maggie code can load source files
	vmInst.SetFileInFunc(func(v *vm.VM, source string, sourcePath string, nsOverride string, verbose bool) (int, error) {
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

	// Register display primitives BEFORE compiling sources
	// so that Maggie code can reference DoradoDisplay at compile time
	registerDisplayPrimitives(vmInst, screen, wm)

	// Load Dorado Maggie source files
	srcDir := findSourceDir()
	if srcDir != "" {
		p := &pipeline.Pipeline{VM: vmInst}
		methods, err := p.CompilePath(srcDir + "/...")
		if err != nil {
			log.Fatalf("Failed to compile Dorado sources: %v", err)
		}
		fmt.Printf("Dorado: compiled %d methods\n", methods)
	}

	// Try to call Dorado::Main.start if it exists
	mainClass := vmInst.Classes.Lookup("Dorado::Main")
	if mainClass == nil {
		mainClass = vmInst.Classes.Lookup("Main")
	}
	if mainClass != nil {
		selectorID := vmInst.Selectors.Intern("start")
		if mainClass.ClassVTable.Lookup(selectorID) != nil {
			qualifiedName := mainClass.FullName()
			classValue := vmInst.Symbols.SymbolValue(qualifiedName)
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Fprintf(os.Stderr, "Maggie error in Main.start: %v\n", r)
						// Try to extract error message
						if exc, ok := r.(interface{ Error() string }); ok {
							fmt.Fprintf(os.Stderr, "  Detail: %s\n", exc.Error())
						}
					}
				}()
				vmInst.Send(classValue, "start", nil)
			}()
		}
	}

	colorBG := display.ColorRGB(168, 168, 168)

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

// findSourceDir looks for the Dorado Maggie source directory.
func findSourceDir() string {
	if info, err := os.Stat("src"); err == nil && info.IsDir() {
		return "src"
	}
	return ""
}

// findMaggieImage locates the Maggie base image.
func findMaggieImage() string {
	// Check standard locations
	paths := []string{
		"maggie.image",
		os.ExpandEnv("$HOME/dev/go/maggie/cmd/mag/maggie.image"),
	}
	// Also check next to the mag binary
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

// registerDisplayPrimitives registers Go functions that Maggie code can call
// to interact with the display system.
func registerDisplayPrimitives(vmInst *vm.VM, screen *display.Form, wm *display.WindowManager) {
	// Create a DoradoDisplay class for display operations
	displayClass := vmInst.Classes.Lookup("DoradoDisplay")
	if displayClass == nil {
		displayClass = vm.NewClass("DoradoDisplay", vmInst.ObjectClass)
		vmInst.Classes.Register(displayClass)
	}
	vmInst.SetGlobal("DoradoDisplay", vmInst.ClassValue(displayClass))

	// --- Window creation and management ---

	// DoradoDisplay newWindow: title x: x y: y width: w  → returns a window ID
	// (>4 params, so we split into two steps)

	// DoradoDisplay newWindow: title → creates window at default position, returns window index
	displayClass.AddClassMethod1(vmInst.Selectors, "newWindow:", func(vmPtr interface{}, recv vm.Value, titleVal vm.Value) vm.Value {
		v := vmPtr.(*vm.VM)
		title := v.Registry().GetStringContent(titleVal)
		w := display.NewWindow(100, 100, 500, 380, title)
		w.SetEditor("")
		wm.AddWindow(w)
		return vm.FromSmallInt(int64(len(wm.Windows()) - 1))
	})

	// DoradoDisplay windowCount → number of open windows
	displayClass.AddClassMethod0(vmInst.Selectors, "windowCount", func(vmPtr interface{}, recv vm.Value) vm.Value {
		return vm.FromSmallInt(int64(len(wm.Windows())))
	})

	// DoradoDisplay screenWidth / screenHeight
	displayClass.AddClassMethod0(vmInst.Selectors, "screenWidth", func(_ interface{}, _ vm.Value) vm.Value {
		return vm.FromSmallInt(int64(screen.Width()))
	})
	displayClass.AddClassMethod0(vmInst.Selectors, "screenHeight", func(_ interface{}, _ vm.Value) vm.Value {
		return vm.FromSmallInt(int64(screen.Height()))
	})

	// DoradoDisplay transcript: text → write text to a Transcript window
	displayClass.AddClassMethod1(vmInst.Selectors, "transcript:", func(vmPtr interface{}, recv vm.Value, textVal vm.Value) vm.Value {
		v := vmPtr.(*vm.VM)
		text := v.Registry().GetStringContent(textVal)
		// Find or create Transcript window
		var transcript *display.Window
		for _, w := range wm.Windows() {
			if w.Title == "Transcript" && !w.Closed {
				transcript = w
				break
			}
		}
		if transcript == nil {
			transcript = display.NewWindow(700, 80, 400, 300, "Transcript")
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
		return vm.Nil
	})
}
