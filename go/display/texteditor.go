package display

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// undoEntry records a state for undo/redo.
type undoEntry struct {
	text   string
	cursor CursorPos
}

// TextEditor is an editable text pane with Emacs-style keybindings.
//
// Keybindings:
//
//	C-space    Set mark (start selection)
//	C-g        Cancel mark / deselect
//	C-a        Beginning of line
//	C-e        End of line
//	C-p        Previous line
//	C-n        Next line
//	C-f        Forward character
//	C-b        Backward character
//	C-d        Delete character forward
//	C-h        Delete character backward (backspace)
//	C-k        Kill to end of line (into kill ring)
//	C-y        Yank (paste from kill ring)
//	C-w        Kill region (cut selection into kill ring)
//	M-w        Copy region (copy selection into kill ring)
//	C-/        Undo
//	C-z        Undo
//	C-shift-z  Redo
//	C-x C-s    (handled by window — Accept/Save)
//	Cmd+C/X/V  Also supported for macOS clipboard compat
type TextEditor struct {
	Buffer *TextBuffer
	Form   *Form

	cursor   CursorPos
	mark     CursorPos // Emacs mark
	markSet  bool      // whether mark is active

	scrollY   int        // first visible line
	scrollbar *Scrollbar // vertical scrollbar

	// Cursor blink
	blinkTick int
	blinkOn   bool

	// Kill ring (Emacs-style)
	killRing     []string
	killRingIdx  int
	lastWasKill  bool // for appending consecutive kills

	// Undo/redo
	undoStack []undoEntry
	redoStack []undoEntry
	undoGroup bool // suppress saving during undo/redo

	// Mouse drag state
	dragging bool

	// Appearance
	PadX           int
	PadY           int
	TabWidth       int
	SyntaxHighlight bool // enable Maggie syntax coloring

	// LSP integration
	LSP        *LSPClient // optional LSP client for completion/hover
	DocumentURI string    // URI for LSP notifications
	docVersion  int       // incremented on each change

	// Completion popup
	completions    []LSPCompletionItem
	completionIdx  int
	showCompletion bool

	// Callbacks
	OnChange func(text string)
}

// NewTextEditor creates a text editor that renders into the given Form.
func NewTextEditor(f *Form, text string) *TextEditor {
	sb := NewScrollbar(f.Width()-scrollbarWidth, 0, f.Height())
	te := &TextEditor{
		Buffer:    NewTextBuffer(text),
		Form:      f,
		scrollbar: sb,
		blinkOn:   true,
		PadX:      4,
		PadY:      4,
		TabWidth:  4,
	}
	te.saveUndo()
	return te
}

// textAreaWidth returns the width available for text (excluding scrollbar).
func (te *TextEditor) textAreaWidth() int {
	return te.Form.Width() - scrollbarWidth
}

// Cursor returns the current cursor position.
func (te *TextEditor) Cursor() CursorPos { return te.cursor }

// SetCursor moves the cursor and deactivates the mark.
func (te *TextEditor) SetCursor(pos CursorPos) {
	te.cursor = te.Buffer.Clamp(pos)
	te.markSet = false
}

// SelectedText returns the text between mark and cursor, or "".
func (te *TextEditor) SelectedText() string {
	if !te.markSet {
		return ""
	}
	return te.Buffer.Selection(te.mark, te.cursor)
}

// Text returns the full buffer text.
func (te *TextEditor) Text() string { return te.Buffer.Text() }

// SetText replaces the buffer content and resets state.
func (te *TextEditor) SetText(text string) {
	te.Buffer.SetText(text)
	te.cursor = CursorPos{0, 0}
	te.markSet = false
	te.scrollY = 0
	te.undoStack = nil
	te.redoStack = nil
	te.saveUndo()
}

// --- Undo / Redo ---

func (te *TextEditor) saveUndo() {
	if te.undoGroup {
		return
	}
	entry := undoEntry{text: te.Buffer.Text(), cursor: te.cursor}
	// Don't save duplicate states
	if len(te.undoStack) > 0 && te.undoStack[len(te.undoStack)-1].text == entry.text {
		return
	}
	te.undoStack = append(te.undoStack, entry)
	// Limit undo history
	if len(te.undoStack) > 200 {
		te.undoStack = te.undoStack[len(te.undoStack)-200:]
	}
	te.redoStack = nil
}

func (te *TextEditor) undo() {
	if len(te.undoStack) <= 1 {
		return
	}
	// Push current state to redo
	te.redoStack = append(te.redoStack, te.undoStack[len(te.undoStack)-1])
	te.undoStack = te.undoStack[:len(te.undoStack)-1]

	entry := te.undoStack[len(te.undoStack)-1]
	te.undoGroup = true
	te.Buffer.SetText(entry.text)
	te.cursor = te.Buffer.Clamp(entry.cursor)
	te.undoGroup = false
	te.markSet = false
	te.ensureCursorVisible()
	te.fireChange()
}

func (te *TextEditor) redo() {
	if len(te.redoStack) == 0 {
		return
	}
	entry := te.redoStack[len(te.redoStack)-1]
	te.redoStack = te.redoStack[:len(te.redoStack)-1]
	te.undoStack = append(te.undoStack, entry)

	te.undoGroup = true
	te.Buffer.SetText(entry.text)
	te.cursor = te.Buffer.Clamp(entry.cursor)
	te.undoGroup = false
	te.markSet = false
	te.ensureCursorVisible()
	te.fireChange()
}

// --- Kill ring ---

func (te *TextEditor) killPush(text string) {
	if te.lastWasKill && len(te.killRing) > 0 {
		// Append to last kill
		te.killRing[len(te.killRing)-1] += text
	} else {
		te.killRing = append(te.killRing, text)
		if len(te.killRing) > 30 {
			te.killRing = te.killRing[1:]
		}
	}
	te.killRingIdx = len(te.killRing) - 1
	// Also set the clipboard for OS interop
	ClipboardSet(te.killRing[te.killRingIdx])
}

func (te *TextEditor) yank() string {
	if len(te.killRing) == 0 {
		// Fall back to clipboard
		return ClipboardGet()
	}
	return te.killRing[te.killRingIdx]
}

// --- Visibility ---

func (te *TextEditor) visibleLines() int {
	lh := DefaultFont().LineHeight()
	return (te.Form.Height() - te.PadY*2) / lh
}

func (te *TextEditor) ensureCursorVisible() {
	vis := te.visibleLines()
	if vis <= 0 {
		vis = 1
	}
	if te.cursor.Line < te.scrollY {
		te.scrollY = te.cursor.Line
	}
	if te.cursor.Line >= te.scrollY+vis {
		te.scrollY = te.cursor.Line - vis + 1
	}
}

// --- Event handling ---

// HandleEvent processes an input event. Returns true if consumed.
func (te *TextEditor) HandleEvent(e Event) bool {
	switch e.Type {
	case EventKeyChar:
		// Don't insert when Ctrl is held (those are Emacs bindings)
		if ebiten.IsKeyPressed(ebiten.KeyControl) {
			return false
		}
		// Don't insert when Cmd is held (those are shortcuts)
		if ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight) {
			return false
		}
		te.insertChar(e.Char)
		return true
	case EventKeyDown:
		return te.handleKey(e.Key)
	case EventMouseDown:
		if e.Button == ButtonLeft {
			return true
		}
	}
	return false
}

func (te *TextEditor) insertChar(ch rune) {
	if ch == '\r' {
		ch = '\n'
	}
	if te.markSet {
		te.deleteRegion()
	}
	s := string(ch)
	if ch == '\t' {
		s = strings.Repeat(" ", te.TabWidth)
	}
	te.cursor = te.Buffer.Insert(te.cursor, s)
	te.markSet = false
	te.lastWasKill = false
	te.ensureCursorVisible()
	te.saveUndo()
	te.fireChange()
}

func (te *TextEditor) deleteRegion() {
	if !te.markSet {
		return
	}
	te.cursor = te.Buffer.Delete(te.mark, te.cursor)
	te.markSet = false
}

func (te *TextEditor) handleKey(key int) bool {
	k := ebiten.Key(key)
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl)
	cmd := ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight)
	shift := ebiten.IsKeyPressed(ebiten.KeyShift)

	// --- Emacs Ctrl bindings ---
	if ctrl {
		switch k {
		case ebiten.KeySpace:
			// C-space: set mark
			te.mark = te.cursor
			te.markSet = true
			return true

		case ebiten.KeyG:
			// C-g: cancel mark
			te.markSet = false
			return true

		case ebiten.KeyA:
			// C-a: beginning of line
			te.cursor.Col = 0
			te.lastWasKill = false
			te.ensureCursorVisible()
			return true

		case ebiten.KeyE:
			// C-e: end of line
			te.cursor.Col = len([]rune(te.Buffer.Line(te.cursor.Line)))
			te.lastWasKill = false
			te.ensureCursorVisible()
			return true

		case ebiten.KeyP:
			// C-p: previous line
			if te.cursor.Line > 0 {
				te.cursor.Line--
				te.cursor = te.Buffer.Clamp(te.cursor)
			}
			te.lastWasKill = false
			te.ensureCursorVisible()
			return true

		case ebiten.KeyN:
			// C-n: next line
			if te.cursor.Line < te.Buffer.LineCount()-1 {
				te.cursor.Line++
				te.cursor = te.Buffer.Clamp(te.cursor)
			}
			te.lastWasKill = false
			te.ensureCursorVisible()
			return true

		case ebiten.KeyF:
			// C-f: forward character
			te.moveForward()
			te.lastWasKill = false
			return true

		case ebiten.KeyB:
			// C-b: backward character
			te.moveBackward()
			te.lastWasKill = false
			return true

		case ebiten.KeyD:
			// C-d: delete character forward
			te.deleteForward()
			te.lastWasKill = false
			return true

		case ebiten.KeyH:
			// C-h: delete character backward (backspace)
			te.deleteBackward()
			te.lastWasKill = false
			return true

		case ebiten.KeyK:
			// C-k: kill to end of line
			te.killLine()
			return true

		case ebiten.KeyY:
			// C-y: yank
			text := te.yank()
			if text != "" {
				if te.markSet {
					te.deleteRegion()
				}
				te.cursor = te.Buffer.Insert(te.cursor, text)
				te.markSet = false
				te.ensureCursorVisible()
				te.saveUndo()
				te.fireChange()
			}
			te.lastWasKill = false
			return true

		case ebiten.KeyW:
			// C-w: kill region (cut)
			if te.markSet {
				text := te.Buffer.Selection(te.mark, te.cursor)
				te.killPush(text)
				te.deleteRegion()
				te.ensureCursorVisible()
				te.saveUndo()
				te.fireChange()
			}
			te.lastWasKill = false
			return true

		case ebiten.KeySlash:
			// C-/: undo
			te.undo()
			te.lastWasKill = false
			return true

		case ebiten.KeyZ:
			// C-z: undo, C-shift-z: redo
			if shift {
				te.redo()
			} else {
				te.undo()
			}
			te.lastWasKill = false
			return true
		}
	}

	// --- Alt/Meta bindings ---
	if ebiten.IsKeyPressed(ebiten.KeyAlt) {
		switch k {
		case ebiten.KeyW:
			// M-w: copy region
			if te.markSet {
				text := te.Buffer.Selection(te.mark, te.cursor)
				te.killPush(text)
			}
			te.lastWasKill = false
			return true

		case ebiten.KeyF:
			// M-f: forward word
			te.moveForwardWord()
			te.lastWasKill = false
			return true

		case ebiten.KeyB:
			// M-b: backward word
			te.moveBackwardWord()
			te.lastWasKill = false
			return true
		}
	}

	// --- Cmd bindings (macOS compat) ---
	if cmd {
		switch k {
		case ebiten.KeyC:
			te.Copy()
			return true
		case ebiten.KeyX:
			te.Cut()
			return true
		case ebiten.KeyV:
			te.Paste()
			return true
		case ebiten.KeyA:
			// Cmd+A: select all (set mark at beginning, cursor at end)
			te.mark = CursorPos{0, 0}
			lastLine := te.Buffer.LineCount() - 1
			te.cursor = CursorPos{lastLine, len([]rune(te.Buffer.Line(lastLine)))}
			te.markSet = true
			return true
		case ebiten.KeyZ:
			if shift {
				te.redo()
			} else {
				te.undo()
			}
			return true
		}
	}

	// --- Completion popup navigation ---
	if te.showCompletion {
		switch k {
		case ebiten.KeyDown:
			if te.completionIdx < len(te.completions)-1 {
				te.completionIdx++
			}
			return true
		case ebiten.KeyUp:
			if te.completionIdx > 0 {
				te.completionIdx--
			}
			return true
		case ebiten.KeyEnter, ebiten.KeyNumpadEnter, ebiten.KeyTab:
			te.acceptCompletion()
			return true
		case ebiten.KeyEscape:
			te.showCompletion = false
			return true
		}
		// Any other key dismisses completion and falls through
		te.showCompletion = false
	}

	// --- Standard keys ---
	switch k {
	case ebiten.KeyBackspace:
		te.deleteBackward()
		te.lastWasKill = false
		return true

	case ebiten.KeyDelete:
		te.deleteForward()
		te.lastWasKill = false
		return true

	case ebiten.KeyEnter, ebiten.KeyNumpadEnter:
		te.insertChar('\n')
		return true

	case ebiten.KeyLeft:
		te.moveBackward()
		te.lastWasKill = false
		return true

	case ebiten.KeyRight:
		te.moveForward()
		te.lastWasKill = false
		return true

	case ebiten.KeyUp:
		if te.cursor.Line > 0 {
			te.cursor.Line--
			te.cursor = te.Buffer.Clamp(te.cursor)
		}
		te.lastWasKill = false
		te.ensureCursorVisible()
		return true

	case ebiten.KeyDown:
		if te.cursor.Line < te.Buffer.LineCount()-1 {
			te.cursor.Line++
			te.cursor = te.Buffer.Clamp(te.cursor)
		}
		te.lastWasKill = false
		te.ensureCursorVisible()
		return true

	case ebiten.KeyTab:
		if te.LSP != nil && te.LSP.IsRunning() {
			te.triggerCompletion()
			return true
		}
		// No LSP: insert spaces
		te.insertChar('\t')
		return true

	case ebiten.KeyHome:
		te.cursor.Col = 0
		te.ensureCursorVisible()
		return true

	case ebiten.KeyEnd:
		te.cursor.Col = len([]rune(te.Buffer.Line(te.cursor.Line)))
		te.ensureCursorVisible()
		return true
	}

	return false
}

// --- Movement helpers ---

func (te *TextEditor) moveForward() {
	lineLen := len([]rune(te.Buffer.Line(te.cursor.Line)))
	if te.cursor.Col < lineLen {
		te.cursor.Col++
	} else if te.cursor.Line < te.Buffer.LineCount()-1 {
		te.cursor.Line++
		te.cursor.Col = 0
	}
	te.ensureCursorVisible()
}

func (te *TextEditor) moveBackward() {
	if te.cursor.Col > 0 {
		te.cursor.Col--
	} else if te.cursor.Line > 0 {
		te.cursor.Line--
		te.cursor.Col = len([]rune(te.Buffer.Line(te.cursor.Line)))
	}
	te.ensureCursorVisible()
}

func (te *TextEditor) moveForwardWord() {
	runes := []rune(te.Buffer.Line(te.cursor.Line))
	col := te.cursor.Col
	// Skip non-word chars
	for col < len(runes) && !isWordChar(runes[col]) {
		col++
	}
	// Skip word chars
	for col < len(runes) && isWordChar(runes[col]) {
		col++
	}
	te.cursor.Col = col
	te.ensureCursorVisible()
}

func (te *TextEditor) moveBackwardWord() {
	runes := []rune(te.Buffer.Line(te.cursor.Line))
	col := te.cursor.Col
	if col > len(runes) {
		col = len(runes)
	}
	// Skip non-word chars backward
	for col > 0 && !isWordChar(runes[col-1]) {
		col--
	}
	// Skip word chars backward
	for col > 0 && isWordChar(runes[col-1]) {
		col--
	}
	te.cursor.Col = col
	te.ensureCursorVisible()
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// --- Editing helpers ---

func (te *TextEditor) deleteBackward() {
	if te.markSet {
		text := te.Buffer.Selection(te.mark, te.cursor)
		te.killPush(text)
		te.deleteRegion()
	} else if te.cursor.Col > 0 {
		prev := CursorPos{te.cursor.Line, te.cursor.Col - 1}
		te.cursor = te.Buffer.Delete(prev, te.cursor)
	} else if te.cursor.Line > 0 {
		prevLine := te.cursor.Line - 1
		prevCol := len([]rune(te.Buffer.Line(prevLine)))
		prev := CursorPos{prevLine, prevCol}
		te.cursor = te.Buffer.Delete(prev, te.cursor)
	}
	te.markSet = false
	te.ensureCursorVisible()
	te.saveUndo()
	te.fireChange()
}

func (te *TextEditor) deleteForward() {
	if te.markSet {
		text := te.Buffer.Selection(te.mark, te.cursor)
		te.killPush(text)
		te.deleteRegion()
	} else {
		lineLen := len([]rune(te.Buffer.Line(te.cursor.Line)))
		if te.cursor.Col < lineLen {
			next := CursorPos{te.cursor.Line, te.cursor.Col + 1}
			te.Buffer.Delete(te.cursor, next)
		} else if te.cursor.Line < te.Buffer.LineCount()-1 {
			next := CursorPos{te.cursor.Line + 1, 0}
			te.Buffer.Delete(te.cursor, next)
		}
	}
	te.markSet = false
	te.ensureCursorVisible()
	te.saveUndo()
	te.fireChange()
}

func (te *TextEditor) killLine() {
	lineRunes := []rune(te.Buffer.Line(te.cursor.Line))
	if te.cursor.Col < len(lineRunes) {
		// Kill to end of line
		end := CursorPos{te.cursor.Line, len(lineRunes)}
		killed := te.Buffer.Selection(te.cursor, end)
		te.killPush(killed)
		te.Buffer.Delete(te.cursor, end)
	} else if te.cursor.Line < te.Buffer.LineCount()-1 {
		// At end of line: kill the newline (join with next line)
		next := CursorPos{te.cursor.Line + 1, 0}
		te.killPush("\n")
		te.Buffer.Delete(te.cursor, next)
	}
	te.lastWasKill = true
	te.ensureCursorVisible()
	te.saveUndo()
	te.fireChange()
}

// --- Clipboard compat (Cmd+C/X/V) ---

func isCmdOrCtrl() bool {
	return ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight) ||
		ebiten.IsKeyPressed(ebiten.KeyControl)
}

// Copy copies the selected text to the clipboard.
func (te *TextEditor) Copy() {
	if te.markSet {
		text := te.Buffer.Selection(te.mark, te.cursor)
		te.killPush(text)
	}
}

// Cut copies the selected text to the clipboard and deletes it.
func (te *TextEditor) Cut() {
	if te.markSet {
		text := te.Buffer.Selection(te.mark, te.cursor)
		te.killPush(text)
		te.deleteRegion()
		te.ensureCursorVisible()
		te.saveUndo()
		te.fireChange()
	}
}

// Paste inserts clipboard content at the cursor, replacing selection if any.
func (te *TextEditor) Paste() {
	text := te.yank()
	if text == "" {
		return
	}
	if te.markSet {
		te.deleteRegion()
	}
	te.cursor = te.Buffer.Insert(te.cursor, text)
	te.markSet = false
	te.ensureCursorVisible()
	te.saveUndo()
	te.fireChange()
}

// --- Mouse handling ---

// HandleClickLocal handles a click in editor-local coordinates.
func (te *TextEditor) HandleClickLocal(localX, localY int, shift bool) {
	if te.scrollbar.Contains(localX, localY) {
		te.scrollbar.HandleClick(localX, localY)
		te.scrollY = te.scrollbar.Offset
		return
	}

	pos := te.posFromLocal(localX, localY)

	if shift || te.markSet {
		// Extend selection
		if !te.markSet {
			te.mark = te.cursor
			te.markSet = true
		}
	} else {
		te.markSet = false
	}

	te.cursor = pos
	te.dragging = true
	te.lastWasKill = false
}

// HandleDragLocal handles a mouse drag in editor-local coordinates.
func (te *TextEditor) HandleDragLocal(localX, localY int) {
	if te.scrollbar.IsDragging() {
		te.scrollbar.HandleDrag(localY)
		te.scrollY = te.scrollbar.Offset
		return
	}

	if te.dragging {
		pos := te.posFromLocal(localX, localY)
		if !te.markSet {
			te.mark = te.cursor
			te.markSet = true
		}
		te.cursor = pos
		te.ensureCursorVisible()
	}
}

// HandleReleaseLocal handles mouse release.
func (te *TextEditor) HandleReleaseLocal() {
	te.scrollbar.HandleRelease()
	te.dragging = false
}

// HandleDoubleClickLocal selects the word at the click position.
func (te *TextEditor) HandleDoubleClickLocal(localX, localY int) {
	pos := te.posFromLocal(localX, localY)
	start, end := te.Buffer.WordAt(pos)
	te.mark = CursorPos{Line: pos.Line, Col: start}
	te.cursor = CursorPos{Line: pos.Line, Col: end}
	te.markSet = start != end
}

func (te *TextEditor) posFromLocal(localX, localY int) CursorPos {
	font := DefaultFont()
	lh := font.LineHeight()

	line := (localY - te.PadY) / lh + te.scrollY
	if line < 0 {
		line = 0
	}
	if line >= te.Buffer.LineCount() {
		line = te.Buffer.LineCount() - 1
	}
	col := te.colFromX(line, localX-te.PadX)
	return te.Buffer.Clamp(CursorPos{Line: line, Col: col})
}

func (te *TextEditor) colFromX(line, px int) int {
	font := DefaultFont()
	runes := []rune(te.Buffer.Line(line))
	x := 0
	for i, r := range runes {
		g := font.GlyphFor(r)
		adv := 6
		if g != nil {
			adv = g.Advance
		}
		if x+adv/2 > px {
			return i
		}
		x += adv
	}
	return len(runes)
}

func (te *TextEditor) fireChange() {
	// Notify LSP of document change
	if te.LSP != nil && te.LSP.IsRunning() && te.DocumentURI != "" {
		te.docVersion++
		te.LSP.DidChange(te.DocumentURI, te.Buffer.Text(), te.docVersion)
	}
	if te.OnChange != nil {
		te.OnChange(te.Buffer.Text())
	}
}

// triggerCompletion requests completions from the LSP.
func (te *TextEditor) triggerCompletion() {
	if te.LSP == nil || !te.LSP.IsRunning() {
		return
	}
	items := te.LSP.Complete(te.DocumentURI, te.cursor.Line, te.cursor.Col)
	if len(items) == 0 {
		te.showCompletion = false
		// Fallback: insert tab
		te.insertChar('\t')
		return
	}
	te.completions = items
	te.completionIdx = 0
	te.showCompletion = true
}

// acceptCompletion inserts the selected completion item.
func (te *TextEditor) acceptCompletion() {
	if !te.showCompletion || te.completionIdx >= len(te.completions) {
		te.showCompletion = false
		return
	}
	item := te.completions[te.completionIdx]
	te.showCompletion = false

	// Find the word prefix to replace
	runes := []rune(te.Buffer.Line(te.cursor.Line))
	wordStart := te.cursor.Col
	for wordStart > 0 && isIdentChar(runes[wordStart-1]) {
		wordStart--
	}

	// Delete the prefix and insert the completion
	if wordStart < te.cursor.Col {
		start := CursorPos{te.cursor.Line, wordStart}
		te.cursor = te.Buffer.Delete(start, te.cursor)
	}
	te.cursor = te.Buffer.Insert(te.cursor, item.Label)
	te.ensureCursorVisible()
	te.saveUndo()
	te.fireChange()
}

// NotifyOpen tells the LSP that this document is open.
func (te *TextEditor) NotifyOpen() {
	if te.LSP != nil && te.LSP.IsRunning() && te.DocumentURI != "" {
		te.LSP.DidOpen(te.DocumentURI, te.Buffer.Text())
	}
}

// HoverAt returns hover info at the cursor position.
func (te *TextEditor) HoverAt() string {
	if te.LSP == nil || !te.LSP.IsRunning() {
		return ""
	}
	return te.LSP.Hover(te.DocumentURI, te.cursor.Line, te.cursor.Col)
}

// --- Rendering ---

// Render draws the text, selection, and cursor into the editor's Form.
func (te *TextEditor) Render() {
	f := te.Form
	font := DefaultFont()
	lh := font.LineHeight()

	te.scrollbar.SetRange(te.Buffer.LineCount(), te.visibleLines())
	te.scrollbar.Offset = te.scrollY

	f.Fill(ColorRGB(255, 255, 255))

	te.blinkTick++
	if te.blinkTick >= 30 {
		te.blinkTick = 0
		te.blinkOn = !te.blinkOn
	}

	visLines := te.visibleLines()
	black := ColorRGB(0, 0, 0)
	selColor := ColorRGB(80, 120, 200)

	// Normalize selection range
	var selMin, selMax CursorPos
	if te.markSet {
		selMin, selMax = te.mark, te.cursor
		if selMin.Line > selMax.Line || (selMin.Line == selMax.Line && selMin.Col > selMax.Col) {
			selMin, selMax = selMax, selMin
		}
	}

	for i := 0; i < visLines && te.scrollY+i < te.Buffer.LineCount(); i++ {
		lineIdx := te.scrollY + i
		lineText := te.Buffer.Line(lineIdx)
		y := te.PadY + i*lh

		if te.markSet && lineIdx >= selMin.Line && lineIdx <= selMax.Line {
			te.drawSelectionLine(f, lineIdx, y, lineText, selMin, selMax, selColor, lh)
		}

		if te.SyntaxHighlight {
			DrawStringHighlighted(f, te.PadX, y, lineText, font)
		} else {
			DrawStringFont(f, te.PadX, y, lineText, black, font)
		}
	}

	// Draw cursor
	if te.blinkOn && te.cursor.Line >= te.scrollY && te.cursor.Line < te.scrollY+visLines {
		cx := te.PadX + te.xFromCol(te.cursor.Line, te.cursor.Col)
		cy := te.PadY + (te.cursor.Line-te.scrollY)*lh
		for dy := 0; dy < lh; dy++ {
			f.SetPixelAt(cx, cy+dy, black)
		}
	}

	te.scrollbar.Render(f)

	// Draw completion popup
	if te.showCompletion && len(te.completions) > 0 {
		te.renderCompletionPopup(f, font, lh)
	}
}

func (te *TextEditor) renderCompletionPopup(f *Form, font *Font, lh int) {
	popupBG := ColorRGB(255, 255, 240)
	popupBorder := ColorRGB(140, 140, 140)
	popupSelBG := ColorRGB(40, 40, 120)
	popupSelFG := ColorRGB(255, 255, 255)
	popupFG := ColorRGB(0, 0, 0)
	detailFG := ColorRGB(120, 120, 120)

	// Position below cursor
	cx := te.PadX + te.xFromCol(te.cursor.Line, te.cursor.Col)
	cy := te.PadY + (te.cursor.Line-te.scrollY+1)*lh

	maxVisible := 8
	if len(te.completions) < maxVisible {
		maxVisible = len(te.completions)
	}

	itemH := lh + 2
	popupW := 250
	popupH := maxVisible*itemH + 4

	// Keep popup in bounds
	if cx+popupW > f.Width() {
		cx = f.Width() - popupW
	}
	if cy+popupH > f.Height() {
		cy = te.PadY + (te.cursor.Line-te.scrollY)*lh - popupH
	}

	f.FillRectWH(popupBG, cx, cy, popupW, popupH)
	drawRectOutline(f, cx, cy, popupW, popupH, popupBorder)

	// Scroll the list if needed
	scrollOff := 0
	if te.completionIdx >= maxVisible {
		scrollOff = te.completionIdx - maxVisible + 1
	}

	for i := 0; i < maxVisible && scrollOff+i < len(te.completions); i++ {
		item := te.completions[scrollOff+i]
		iy := cy + 2 + i*itemH
		idx := scrollOff + i

		fg := popupFG
		if idx == te.completionIdx {
			f.FillRectWH(popupSelBG, cx+1, iy, popupW-2, itemH)
			fg = popupSelFG
		}

		DrawStringFont(f, cx+6, iy+1, item.Label, fg, font)
		if item.Detail != "" {
			detailX := cx + 6 + font.MeasureString(item.Label) + 8
			DrawStringFont(f, detailX, iy+1, item.Detail, detailFG, font)
		}
	}
}

func (te *TextEditor) drawSelectionLine(f *Form, lineIdx, y int, lineText string, selMin, selMax CursorPos, selColor uint32, lh int) {
	runes := []rune(lineText)
	startCol := 0
	endCol := len(runes)

	if lineIdx == selMin.Line {
		startCol = selMin.Col
	}
	if lineIdx == selMax.Line {
		endCol = selMax.Col
	}

	x1 := te.PadX + te.xFromCol(lineIdx, startCol)
	x2 := te.PadX + te.xFromCol(lineIdx, endCol)
	if x2 <= x1 {
		return
	}

	f.FillRectWH(selColor, x1, y, x2-x1, lh)
}

func (te *TextEditor) xFromCol(line, col int) int {
	font := DefaultFont()
	runes := []rune(te.Buffer.Line(line))
	if col > len(runes) {
		col = len(runes)
	}
	x := 0
	for i := 0; i < col; i++ {
		g := font.GlyphFor(runes[i])
		if g != nil {
			x += g.Advance
		} else {
			x += 6
		}
	}
	return x
}
