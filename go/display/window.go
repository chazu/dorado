package display

// Window represents a single overlapping window on the display.
type Window struct {
	X, Y          int    // position on screen
	Width, Height int    // outer dimensions including chrome
	Title         string
	Content       *Form  // the content area (excludes chrome)
	Editor        *TextEditor // optional embedded text editor
	OnContentClick func(localX, localY int) // custom click handler (overrides editor)
	OnKeyEvent     func(e Event) bool       // custom key handler (before editor)
	Closed        bool

	// internal
	form     *Form // full window form including chrome
	contentX int   // content area offset within form
	contentY int   // content area offset within form
	contentW int   // content area width
	contentH int   // content area height
	dirty    bool  // needs redraw
}

const (
	titleBarHeight = 20
	borderWidth    = 1
	closeBoxSize   = 12
	closeBoxPad    = 4 // padding from top-left corner of title bar
)

// NewWindow creates a window with the given content area dimensions.
// The actual window is larger due to chrome (title bar, borders).
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
	// Fill content white by default
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

// Render composites the window chrome and content into the window's form.
func (w *Window) Render() *Form {
	if !w.dirty {
		return w.form
	}
	w.dirty = false

	f := w.form

	// Clear
	f.Fill(ColorRGB(255, 255, 255))

	// Title bar background
	f.FillRectWH(ColorRGB(80, 80, 80), borderWidth, borderWidth, w.Width-borderWidth*2, titleBarHeight)

	// Close box
	cbX := borderWidth + closeBoxPad
	cbY := borderWidth + (titleBarHeight-closeBoxSize)/2
	f.FillRectWH(ColorRGB(255, 255, 255), cbX, cbY, closeBoxSize, closeBoxSize)
	drawRectOutline(f, cbX, cbY, closeBoxSize, closeBoxSize, ColorRGB(0, 0, 0))

	// Title text
	textX := cbX + closeBoxSize + 8
	textY := borderWidth + (titleBarHeight-DefaultFont().LineHeight())/2
	DrawString(f, textX, textY, w.Title, ColorRGB(255, 255, 255))

	// Render editor into content form if present
	if w.Editor != nil {
		w.Editor.Render()
	}

	// Blit content into form
	CopyBits(f, w.contentX, w.contentY, w.Content)

	// Outer border
	drawRectOutline(f, 0, 0, w.Width, w.Height, ColorRGB(0, 0, 0))

	// Title bar separator
	for x := 0; x < w.Width; x++ {
		f.SetPixelAt(x, borderWidth+titleBarHeight, ColorRGB(0, 0, 0))
	}

	return f
}

// HitTest checks what part of the window a screen-space point hits.
type HitZone int

const (
	HitNone    HitZone = iota
	HitTitleBar
	HitCloseBox
	HitContent
)

// HitTest returns the zone hit by screen-space coordinates.
func (w *Window) HitTest(sx, sy int) HitZone {
	// Convert to window-local coords
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

	// Title bar
	if ly >= borderWidth && ly < borderWidth+titleBarHeight {
		return HitTitleBar
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
