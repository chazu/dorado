package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chazu/dorado/go/display"
	"github.com/chazu/maggie/vm"
)

// FileList is a file browser for navigating the filesystem and filing in code.
type FileList struct {
	window *display.Window

	// State
	currentDir string
	entries    []fileEntry
	selIndex   int

	// Layout
	listPaneH int

	// Content viewer
	contentForm   *display.Form
	contentEditor *display.TextEditor
}

type fileEntry struct {
	Name  string
	IsDir bool
	Path  string
}

var fileListRegistry = map[*display.Window]*FileList{}

func openFileList() {
	dir, _ := os.Getwd()
	openFileListAt(dir)
}

func openFileListAt(dir string) {
	contentW, contentH := 600, 450
	w := display.NewWindow(150, 100, contentW, contentH, "File List: "+filepath.Base(dir))

	fl := &FileList{
		window:     w,
		currentDir: dir,
		selIndex:   -1,
		listPaneH:  180,
	}

	fl.loadEntries()

	contentH2 := contentH - fl.listPaneH - 4
	fl.contentForm = display.NewForm(contentW, contentH2)
	fl.contentEditor = display.NewTextEditor(fl.contentForm, "")
	fl.contentEditor.SyntaxHighlight = true

	w.Content = display.NewForm(contentW, contentH)

	w.OnContentClick = func(lx, ly int) {
		fl.handleClick(lx, ly)
	}

	w.OnKeyEvent = func(e display.Event) bool {
		consumed := fl.contentEditor.HandleEvent(e)
		if consumed {
			fl.render()
		}
		return consumed
	}

	fl.render()
	fileListRegistry[w] = fl
	app.wm.AddWindow(w)
}

func (fl *FileList) loadEntries() {
	fl.entries = nil
	fl.selIndex = -1

	// Parent directory
	fl.entries = append(fl.entries, fileEntry{Name: "..", IsDir: true, Path: filepath.Dir(fl.currentDir)})

	dirEntries, err := os.ReadDir(fl.currentDir)
	if err != nil {
		return
	}

	// Directories first, then files, sorted
	var dirs, files []fileEntry
	for _, e := range dirEntries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(fl.currentDir, e.Name())
		if e.IsDir() {
			dirs = append(dirs, fileEntry{Name: e.Name() + "/", IsDir: true, Path: path})
		} else {
			files = append(files, fileEntry{Name: e.Name(), IsDir: false, Path: path})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	fl.entries = append(fl.entries, dirs...)
	fl.entries = append(fl.entries, files...)
}

func (fl *FileList) render() {
	f := fl.window.Content
	w := f.Width()
	font := display.DefaultFont()
	lh := font.LineHeight() + 2

	white := display.ColorRGB(255, 255, 255)
	black := display.ColorRGB(0, 0, 0)
	gray := display.ColorRGB(180, 180, 180)
	blue := display.ColorRGB(0, 60, 180)
	selBG := display.ColorRGB(40, 40, 120)
	selFG := display.ColorRGB(255, 255, 255)

	f.Fill(white)

	// Directory path
	display.DrawString(f, 4, 2, fl.currentDir, gray)
	y := lh + 4

	// File list
	display.DrawHLine(f, 0, y-1, w, gray)
	for i, entry := range fl.entries {
		iy := y + i*lh
		if iy+lh > fl.listPaneH {
			break
		}
		fg := black
		if entry.IsDir {
			fg = blue
		}
		if i == fl.selIndex {
			f.FillRectWH(selBG, 0, iy, w, lh)
			display.DrawString(f, 8, iy, entry.Name, selFG)
		} else {
			display.DrawString(f, 8, iy, entry.Name, fg)
		}
	}

	// Separator
	display.DrawHLine(f, 0, fl.listPaneH, w, gray)

	// Button bar
	btnY := fl.listPaneH + 2
	btnH := 20
	btnBG := display.ColorRGB(200, 200, 200)

	bx := 4
	for _, label := range []string{"File In", "File Out", "Refresh"} {
		btnW := font.MeasureString(label) + 12
		f.FillRectWH(btnBG, bx, btnY, btnW, btnH)
		display.DrawRect(f, bx, btnY, btnW, btnH, gray)
		display.DrawString(f, bx+6, btnY+3, label, black)
		bx += btnW + 4
	}

	// Content editor
	contentY := btnY + btnH + 2
	contentH := f.Height() - contentY
	if fl.contentForm.Height() != contentH || fl.contentForm.Width() != w {
		oldText := fl.contentEditor.Text()
		fl.contentForm = display.NewForm(w, contentH)
		fl.contentEditor = display.NewTextEditor(fl.contentForm, oldText)
		fl.contentEditor.SyntaxHighlight = true
	}
	fl.contentEditor.Render()
	display.CopyBits(f, 0, contentY, fl.contentForm)

	fl.window.MarkDirty()
}

func (fl *FileList) handleClick(lx, ly int) {
	font := display.DefaultFont()
	lh := font.LineHeight() + 2
	listStartY := lh + 4

	// File list click
	if ly >= listStartY && ly < fl.listPaneH {
		idx := (ly - listStartY) / lh
		if idx >= 0 && idx < len(fl.entries) {
			entry := fl.entries[idx]
			if entry.IsDir {
				// Navigate into directory
				fl.currentDir = entry.Path
				fl.window.Title = "File List: " + filepath.Base(fl.currentDir)
				fl.loadEntries()
				fl.contentEditor.SetText("")
			} else {
				fl.selIndex = idx
				fl.loadFileContent(entry.Path)
			}
		}
		fl.render()
		return
	}

	// Button bar
	btnY := fl.listPaneH + 2
	btnH := 20
	if ly >= btnY && ly < btnY+btnH {
		fl.handleButtonClick(lx)
		return
	}

	// Content editor
	contentY := btnY + btnH + 2
	fl.contentEditor.HandleClickLocal(lx, ly-contentY, false)
	fl.render()
}

func (fl *FileList) handleButtonClick(lx int) {
	font := display.DefaultFont()
	bx := 4
	for i, label := range []string{"File In", "File Out", "Refresh"} {
		btnW := font.MeasureString(label) + 12
		if lx >= bx && lx < bx+btnW {
			switch i {
			case 0: // File In
				fl.fileIn()
			case 1: // File Out
				transcriptWrite("File Out not yet implemented")
			case 2: // Refresh
				fl.loadEntries()
				fl.render()
			}
			return
		}
		bx += btnW + 4
	}
}

func (fl *FileList) loadFileContent(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fl.contentEditor.SetText(fmt.Sprintf("Error reading %s: %v", path, err))
		return
	}
	fl.contentEditor.SetText(string(data))
}

func (fl *FileList) fileIn() {
	if fl.selIndex < 0 || fl.selIndex >= len(fl.entries) {
		transcriptWrite("No file selected")
		return
	}
	entry := fl.entries[fl.selIndex]
	if entry.IsDir {
		transcriptWrite("Cannot file in a directory")
		return
	}

	if !strings.HasSuffix(entry.Name, ".mag") {
		transcriptWrite("Can only file in .mag files")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			transcriptWrite(fmt.Sprintf("File in error: %v", r))
		}
	}()

	// Use the VM's FileIn mechanism
	data, err := os.ReadFile(entry.Path)
	if err != nil {
		transcriptWrite(fmt.Sprintf("Error reading %s: %v", entry.Path, err))
		return
	}

	method, compileErr := app.vm.CompileExpression(fmt.Sprintf("Compiler fileIn: '%s'", entry.Path))
	if compileErr != nil {
		// Fallback: compile the source directly
		transcriptWrite(fmt.Sprintf("Filing in %s (%d bytes)...", entry.Name, len(data)))
		// Use the pipeline directly — not ideal but works
		transcriptWrite("Direct file-in not yet supported; use Compiler fileIn: in a Workspace")
		return
	}

	_, execErr := app.vm.ExecuteSafe(method, vm.Nil, nil)
	if execErr != nil {
		transcriptWrite(fmt.Sprintf("File in error: %v", execErr))
		return
	}

	transcriptWrite(fmt.Sprintf("Filed in %s", entry.Name))
}
