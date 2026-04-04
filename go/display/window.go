package display

const (
	titleBarHeight = 20
	borderWidth    = 1
	closeBoxSize   = 12
	closeBoxPad    = 4
	resizeGripSize = 12 // bottom-right resize grip
	minWindowW     = 120
	minWindowH     = 80
)

// Window represents a single overlapping window on the display.
type Window struct {
	X, Y          int    // position on screen
	Width, Height int    // outer dimensions including chrome
	Title         string
	Content       *Form        // the content area (excludes chrome)
	Editor        *TextEditor  // optional embedded text editor
	OnContentClick func(localX, localY int)
	OnKeyEvent     func(e Event) bool
	OnResize       func(contentW, contentH int) // called after resize
	Closed        bool
	Collapsed     bool // title-bar only

	// internal
	form     *Form
	contentX int
	contentY int
	contentW int
	contentH int
	dirty    bool

	// Saved dimensions for collapse/expand
	savedHeight int
}

// NewWindow creates a window with the given content area dimensions.
func NewWindow(x, y, contentW, contentH int, title string) *Window {
	w := contentW + borderWidth*2
	h := contentH + titleBarHeight + borderWidth*2

	win := &Window{
		X:        x,
		Y:        y,
		Width:    w,
		Height:   h,
		Title:    title,
		contentX: borderWidth,
		contentY: borderWidth + titleBarHeight,
		contentW: contentW,
		contentH: contentH,
		dirty:    true,
	}

	win.form = NewForm(w, h)
	win.Content = NewForm(contentW, contentH)
	win.Content.Fill(ColorRGB(255, 255, 255))

	return win
}

// SetEditor attaches a text editor to the window's content area.
func (w *Window) SetEditor(text string) *TextEditor {
	te := NewTextEditor(w.Content, text)
	w.Editor = te
	w.dirty = true
	return te
}

// ScreenToContent converts screen-space coordinates to content-local coordinates.
func (w *Window) ScreenToContent(sx, sy int) (int, int) {
	return sx - w.X - w.contentX, sy - w.Y - w.contentY
}

// MarkDirty flags the window for redraw.
func (w *Window) MarkDirty() { w.dirty = true }

// ContentBounds returns the screen-space rectangle of the content area.
func (w *Window) ContentBounds() (x, y, width, height int) {
	return w.X + w.contentX, w.Y + w.contentY, w.contentW, w.contentH
}

// Resize changes the window's content area dimensions and reallocates forms.
func (w *Window) Resize(newContentW, newContentH int) {
	if newContentW < minWindowW-borderWidth*2 {
		newContentW = minWindowW - borderWidth*2
	}
	if newContentH < minWindowH-titleBarHeight-borderWidth*2 {
		newContentH = minWindowH - titleBarHeight - borderWidth*2
	}

	w.contentW = newContentW
	w.contentH = newContentH
	w.Width = newContentW + borderWidth*2
	w.Height = newContentH + titleBarHeight + borderWidth*2

	w.form = NewForm(w.Width, w.Height)

	// Preserve editor text if present
	oldText := ""
	if w.Editor != nil {
		oldText = w.Editor.Text()
	}

	w.Content = NewForm(newContentW, newContentH)
	w.Content.Fill(ColorRGB(255, 255, 255))

	if w.Editor != nil {
		w.Editor = NewTextEditor(w.Content, oldText)
	}

	if w.OnResize != nil {
		w.OnResize(newContentW, newContentH)
	}

	w.dirty = true
}

// ToggleCollapse collapses or expands the window.
func (w *Window) ToggleCollapse() {
	if w.Collapsed {
		// Expand
		w.Height = w.savedHeight
		w.contentH = w.Height - titleBarHeight - borderWidth*2
		w.form = NewForm(w.Width, w.Height)
		w.Collapsed = false
	} else {
		// Collapse to title bar only
		w.savedHeight = w.Height
		w.Height = titleBarHeight + borderWidth*2
		w.form = NewForm(w.Width, w.Height)
		w.Collapsed = true
	}
	w.dirty = true
}

// Render composites the window chrome and content into the window's form.
func (w *Window) Render() *Form {
	if !w.dirty {
		return w.form
	}
	w.dirty = false

	f := w.form
	black := ColorRGB(0, 0, 0)
	white := ColorRGB(255, 255, 255)
	darkGray := ColorRGB(80, 80, 80)

	// Clear
	f.Fill(white)

	// Title bar background
	f.FillRectWH(darkGray, borderWidth, borderWidth, w.Width-borderWidth*2, titleBarHeight)

	// Close box
	cbX := borderWidth + closeBoxPad
	cbY := borderWidth + (titleBarHeight-closeBoxSize)/2
	f.FillRectWH(white, cbX, cbY, closeBoxSize, closeBoxSize)
	drawRectOutline(f, cbX, cbY, closeBoxSize, closeBoxSize, black)

	// Collapse box (next to close box)
	colX := cbX + closeBoxSize + 4
	colY := cbY
	f.FillRectWH(white, colX, colY, closeBoxSize, closeBoxSize)
	drawRectOutline(f, colX, colY, closeBoxSize, closeBoxSize, black)
	// Draw a horizontal line in the collapse box to indicate minimize
	midY := colY + closeBoxSize/2
	for cx := colX + 2; cx < colX+closeBoxSize-2; cx++ {
		f.SetPixelAt(cx, midY, black)
	}

	// Title text
	textX := colX + closeBoxSize + 8
	textY := borderWidth + (titleBarHeight-DefaultFont().LineHeight())/2
	DrawString(f, textX, textY, w.Title, white)

	if !w.Collapsed {
		// Render editor into content form if present
		if w.Editor != nil {
			w.Editor.Render()
		}

		// Blit content into form
		CopyBits(f, w.contentX, w.contentY, w.Content)

		// Resize grip (bottom-right corner, small triangle)
		gripX := w.Width - resizeGripSize - 1
		gripY := w.Height - resizeGripSize - 1
		for dy := 0; dy < resizeGripSize; dy++ {
			for dx := resizeGripSize - dy; dx < resizeGripSize; dx++ {
				if (dx+dy)%3 == 0 {
					f.SetPixelAt(gripX+dx, gripY+dy, ColorRGB(120, 120, 120))
				}
			}
		}
	}

	// Outer border
	drawRectOutline(f, 0, 0, w.Width, w.Height, black)

	// Title bar separator
	for x := 0; x < w.Width; x++ {
		f.SetPixelAt(x, borderWidth+titleBarHeight, black)
	}

	return f
}

// HitZone identifies which part of a window was clicked.
type HitZone int

const (
	HitNone       HitZone = iota
	HitTitleBar
	HitCloseBox
	HitCollapseBox
	HitContent
	HitResizeGrip
)

// HitTest returns the zone hit by screen-space coordinates.
func (w *Window) HitTest(sx, sy int) HitZone {
	lx := sx - w.X
	ly := sy - w.Y

	if lx < 0 || ly < 0 || lx >= w.Width || ly >= w.Height {
		return HitNone
	}

	// Close box
	cbX := borderWidth + closeBoxPad
	cbY := borderWidth + (titleBarHeight-closeBoxSize)/2
	if lx >= cbX && lx < cbX+closeBoxSize && ly >= cbY && ly < cbY+closeBoxSize {
		return HitCloseBox
	}

	// Collapse box
	colX := cbX + closeBoxSize + 4
	if lx >= colX && lx < colX+closeBoxSize && ly >= cbY && ly < cbY+closeBoxSize {
		return HitCollapseBox
	}

	// Title bar
	if ly >= borderWidth && ly < borderWidth+titleBarHeight {
		return HitTitleBar
	}

	if w.Collapsed {
		return HitNone
	}

	// Resize grip (bottom-right corner)
	if lx >= w.Width-resizeGripSize-1 && ly >= w.Height-resizeGripSize-1 {
		return HitResizeGrip
	}

	// Content area
	if lx >= w.contentX && lx < w.contentX+w.contentW &&
		ly >= w.contentY && ly < w.contentY+w.contentH {
		return HitContent
	}

	return HitNone
}

// Contains returns true if the screen-space point is inside the window.
func (w *Window) Contains(sx, sy int) bool {
	return sx >= w.X && sx < w.X+w.Width && sy >= w.Y && sy < w.Y+w.Height
}

func drawRectOutline(f *Form, x, y, w, h int, rgba uint32) {
	for i := 0; i < w; i++ {
		f.SetPixelAt(x+i, y, rgba)
		f.SetPixelAt(x+i, y+h-1, rgba)
	}
	for i := 0; i < h; i++ {
		f.SetPixelAt(x, y+i, rgba)
		f.SetPixelAt(x+w-1, y+i, rgba)
	}
}
