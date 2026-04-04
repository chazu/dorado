package display

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// TextEditor is an editable text pane that renders into a Form.
type TextEditor struct {
	Buffer *TextBuffer
	Form   *Form

	cursor   CursorPos
	selStart CursorPos // anchor of selection
	hasSel   bool

	scrollY   int        // first visible line
	scrollbar *Scrollbar // vertical scrollbar

	// Cursor blink
	blinkTick int
	blinkOn   bool

	// Appearance
	PadX     int
	PadY     int
	TabWidth int

	// Callbacks
	OnChange func(text string) // called after any edit
}

// NewTextEditor creates a text editor that renders into the given Form.
func NewTextEditor(f *Form, text string) *TextEditor {
	sb := NewScrollbar(f.Width()-scrollbarWidth, 0, f.Height())
	return &TextEditor{
		Buffer:    NewTextBuffer(text),
		Form:      f,
		scrollbar: sb,
		blinkOn:   true,
		PadX:      4,
		PadY:      4,
		TabWidth:  4,
	}
}

// textAreaWidth returns the width available for text (excluding scrollbar).
func (te *TextEditor) textAreaWidth() int {
	return te.Form.Width() - scrollbarWidth
}

// Cursor returns the current cursor position.
func (te *TextEditor) Cursor() CursorPos { return te.cursor }

// SetCursor moves the cursor and clears selection.
func (te *TextEditor) SetCursor(pos CursorPos) {
	te.cursor = te.Buffer.Clamp(pos)
	te.hasSel = false
}

// SelectedText returns the currently selected text, or "".
func (te *TextEditor) SelectedText() string {
	if !te.hasSel {
		return ""
	}
	return te.Buffer.Selection(te.selStart, te.cursor)
}

// Text returns the full buffer text.
func (te *TextEditor) Text() string { return te.Buffer.Text() }

// SetText replaces the buffer content and resets cursor.
func (te *TextEditor) SetText(text string) {
	te.Buffer.SetText(text)
	te.cursor = CursorPos{0, 0}
	te.hasSel = false
	te.scrollY = 0
}

// visibleLines returns the number of text lines that fit in the form.
func (te *TextEditor) visibleLines() int {
	lh := DefaultFont().LineHeight()
	return (te.Form.Height() - te.PadY*2) / lh
}

// ensureCursorVisible scrolls to keep cursor in view.
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

// HandleEvent processes an input event. Returns true if consumed.
func (te *TextEditor) HandleEvent(e Event) bool {
	switch e.Type {
	case EventKeyChar:
		te.insertChar(e.Char)
		return true
	case EventKeyDown:
		return te.handleKey(e.Key)
	case EventMouseDown:
		if e.Button == ButtonLeft {
			return true // mouse clicks handled via HandleClickLocal from window manager
		}
	}
	return false
}

func (te *TextEditor) insertChar(ch rune) {
	if ch == '\r' {
		ch = '\n'
	}
	if te.hasSel {
		te.deleteSelection()
	}
	s := string(ch)
	if ch == '\t' {
		s = "    " // expand tabs
	}
	te.cursor = te.Buffer.Insert(te.cursor, s)
	te.hasSel = false
	te.ensureCursorVisible()
	te.fireChange()
}

func (te *TextEditor) deleteSelection() {
	if !te.hasSel {
		return
	}
	te.cursor = te.Buffer.Delete(te.selStart, te.cursor)
	te.hasSel = false
}

func (te *TextEditor) handleKey(key int) bool {
	k := ebiten.Key(key)
	shift := ebiten.IsKeyPressed(ebiten.KeyShift)

	switch k {
	case ebiten.KeyBackspace:
		if te.hasSel {
			te.deleteSelection()
		} else if te.cursor.Col > 0 {
			prev := CursorPos{te.cursor.Line, te.cursor.Col - 1}
			te.cursor = te.Buffer.Delete(prev, te.cursor)
		} else if te.cursor.Line > 0 {
			// Join with previous line
			prevLine := te.cursor.Line - 1
			prevCol := len([]rune(te.Buffer.Line(prevLine)))
			prev := CursorPos{prevLine, prevCol}
			te.cursor = te.Buffer.Delete(prev, te.cursor)
		}
		te.hasSel = false
		te.ensureCursorVisible()
		te.fireChange()
		return true

	case ebiten.KeyDelete:
		if te.hasSel {
			te.deleteSelection()
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
		te.hasSel = false
		te.ensureCursorVisible()
		te.fireChange()
		return true

	case ebiten.KeyEnter, ebiten.KeyNumpadEnter:
		te.insertChar('\n')
		return true

	case ebiten.KeyLeft:
		te.startSelIfShift(shift)
		if te.cursor.Col > 0 {
			te.cursor.Col--
		} else if te.cursor.Line > 0 {
			te.cursor.Line--
			te.cursor.Col = len([]rune(te.Buffer.Line(te.cursor.Line)))
		}
		if !shift {
			te.hasSel = false
		}
		te.ensureCursorVisible()
		return true

	case ebiten.KeyRight:
		te.startSelIfShift(shift)
		lineLen := len([]rune(te.Buffer.Line(te.cursor.Line)))
		if te.cursor.Col < lineLen {
			te.cursor.Col++
		} else if te.cursor.Line < te.Buffer.LineCount()-1 {
			te.cursor.Line++
			te.cursor.Col = 0
		}
		if !shift {
			te.hasSel = false
		}
		te.ensureCursorVisible()
		return true

	case ebiten.KeyUp:
		te.startSelIfShift(shift)
		if te.cursor.Line > 0 {
			te.cursor.Line--
			te.cursor = te.Buffer.Clamp(te.cursor)
		}
		if !shift {
			te.hasSel = false
		}
		te.ensureCursorVisible()
		return true

	case ebiten.KeyDown:
		te.startSelIfShift(shift)
		if te.cursor.Line < te.Buffer.LineCount()-1 {
			te.cursor.Line++
			te.cursor = te.Buffer.Clamp(te.cursor)
		}
		if !shift {
			te.hasSel = false
		}
		te.ensureCursorVisible()
		return true

	case ebiten.KeyHome:
		te.startSelIfShift(shift)
		te.cursor.Col = 0
		if !shift {
			te.hasSel = false
		}
		return true

	case ebiten.KeyEnd:
		te.startSelIfShift(shift)
		te.cursor.Col = len([]rune(te.Buffer.Line(te.cursor.Line)))
		if !shift {
			te.hasSel = false
		}
		return true

	case ebiten.KeyA:
		if isCmdOrCtrl() {
			// Select all
			te.selStart = CursorPos{0, 0}
			lastLine := te.Buffer.LineCount() - 1
			te.cursor = CursorPos{lastLine, len([]rune(te.Buffer.Line(lastLine)))}
			te.hasSel = true
			return true
		}

	case ebiten.KeyC:
		if isCmdOrCtrl() {
			te.Copy()
			return true
		}

	case ebiten.KeyX:
		if isCmdOrCtrl() {
			te.Cut()
			return true
		}

	case ebiten.KeyV:
		if isCmdOrCtrl() {
			te.Paste()
			return true
		}
	}
	return false
}

func isCmdOrCtrl() bool {
	return ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight) ||
		ebiten.IsKeyPressed(ebiten.KeyControl)
}

// Copy copies the selected text to the clipboard.
func (te *TextEditor) Copy() {
	if te.hasSel {
		ClipboardSet(te.Buffer.Selection(te.selStart, te.cursor))
	}
}

// Cut copies the selected text to the clipboard and deletes it.
func (te *TextEditor) Cut() {
	if te.hasSel {
		ClipboardSet(te.Buffer.Selection(te.selStart, te.cursor))
		te.deleteSelection()
		te.ensureCursorVisible()
		te.fireChange()
	}
}

// Paste inserts clipboard content at the cursor, replacing selection if any.
func (te *TextEditor) Paste() {
	text := ClipboardGet()
	if text == "" {
		return
	}
	if te.hasSel {
		te.deleteSelection()
	}
	te.cursor = te.Buffer.Insert(te.cursor, text)
	te.hasSel = false
	te.ensureCursorVisible()
	te.fireChange()
}

func (te *TextEditor) startSelIfShift(shift bool) {
	if shift && !te.hasSel {
		te.selStart = te.cursor
		te.hasSel = true
	}
}

// HandleClickLocal handles a click in editor-local coordinates.
func (te *TextEditor) HandleClickLocal(localX, localY int, shift bool) {
	// Check scrollbar first
	if te.scrollbar.Contains(localX, localY) {
		te.scrollbar.HandleClick(localX, localY)
		te.scrollY = te.scrollbar.Offset
		return
	}

	font := DefaultFont()
	lh := font.LineHeight()

	line := (localY - te.PadY) / lh + te.scrollY
	if line < 0 {
		line = 0
	}
	if line >= te.Buffer.LineCount() {
		line = te.Buffer.LineCount() - 1
	}

	// Find column by measuring character widths
	col := te.colFromX(line, localX-te.PadX)

	if shift {
		if !te.hasSel {
			te.selStart = te.cursor
			te.hasSel = true
		}
	} else {
		te.hasSel = false
	}

	te.cursor = te.Buffer.Clamp(CursorPos{Line: line, Col: col})
}

// HandleDragLocal handles a mouse drag in editor-local coordinates.
func (te *TextEditor) HandleDragLocal(localX, localY int) {
	if te.scrollbar.IsDragging() {
		te.scrollbar.HandleDrag(localY)
		te.scrollY = te.scrollbar.Offset
		return
	}
}

// HandleReleaseLocal handles mouse release.
func (te *TextEditor) HandleReleaseLocal() {
	te.scrollbar.HandleRelease()
}

// HandleDoubleClickLocal selects the word at the click position.
func (te *TextEditor) HandleDoubleClickLocal(localX, localY int) {
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
	pos := te.Buffer.Clamp(CursorPos{Line: line, Col: col})

	start, end := te.Buffer.WordAt(pos)
	te.selStart = CursorPos{Line: pos.Line, Col: start}
	te.cursor = CursorPos{Line: pos.Line, Col: end}
	te.hasSel = start != end
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
	if te.OnChange != nil {
		te.OnChange(te.Buffer.Text())
	}
}

// Render draws the text, selection, and cursor into the editor's Form.
func (te *TextEditor) Render() {
	f := te.Form
	font := DefaultFont()
	lh := font.LineHeight()

	// Sync scrollbar state
	te.scrollbar.SetRange(te.Buffer.LineCount(), te.visibleLines())
	te.scrollbar.Offset = te.scrollY

	// Clear to white
	f.Fill(ColorRGB(255, 255, 255))

	// Update blink
	te.blinkTick++
	if te.blinkTick >= 30 { // ~0.5s at 60fps
		te.blinkTick = 0
		te.blinkOn = !te.blinkOn
	}

	visLines := te.visibleLines()
	black := ColorRGB(0, 0, 0)
	selColor := ColorRGB(80, 120, 200)

	// Normalize selection range
	var selMin, selMax CursorPos
	if te.hasSel {
		selMin, selMax = te.selStart, te.cursor
		if selMin.Line > selMax.Line || (selMin.Line == selMax.Line && selMin.Col > selMax.Col) {
			selMin, selMax = selMax, selMin
		}
	}

	for i := 0; i < visLines && te.scrollY+i < te.Buffer.LineCount(); i++ {
		lineIdx := te.scrollY + i
		lineText := te.Buffer.Line(lineIdx)
		y := te.PadY + i*lh

		// Draw selection highlight for this line
		if te.hasSel && lineIdx >= selMin.Line && lineIdx <= selMax.Line {
			te.drawSelectionLine(f, lineIdx, y, lineText, selMin, selMax, selColor, lh)
		}

		// Draw text
		DrawStringFont(f, te.PadX, y, lineText, black, font)
	}

	// Draw cursor
	if te.blinkOn && te.cursor.Line >= te.scrollY && te.cursor.Line < te.scrollY+visLines {
		cx := te.PadX + te.xFromCol(te.cursor.Line, te.cursor.Col)
		cy := te.PadY + (te.cursor.Line-te.scrollY)*lh
		for dy := 0; dy < lh; dy++ {
			f.SetPixelAt(cx, cy+dy, black)
		}
	}

	// Draw scrollbar
	te.scrollbar.Render(f)
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
