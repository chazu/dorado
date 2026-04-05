package display

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// WindowManager manages a set of overlapping windows with z-ordering.
// Windows are stored front-to-back: index 0 is the topmost window.
type WindowManager struct {
	windows []*Window
	screen  *Form
	focused *Window // window that receives keyboard input

	// Drag state
	dragging *Window
	dragOffX int
	dragOffY int

	// Resize state
	resizing      *Window
	resizeStartW  int
	resizeStartH  int
	resizeStartMX int
	resizeStartMY int

	// Double-click detection
	lastClickX    int
	lastClickY    int
	lastClickTick int
	clickCount    int

	// Active popup menu
	activeMenu *Menu

	// Menu providers
	WorldMenuFunc   func(x, y int) []MenuItem           // right-click on desktop
	WindowMenuFunc  func(w *Window, x, y int) []MenuItem // right-click in window content

	// Software cursor
	cursor *Cursor
}

// NewWindowManager creates a window manager that composites onto the given screen Form.
func NewWindowManager(screen *Form) *WindowManager {
	return &WindowManager{
		screen: screen,
		cursor: NewCursor(),
	}
}

// AddWindow adds a window to the top of the z-order and focuses it.
func (wm *WindowManager) AddWindow(w *Window) {
	wm.windows = append([]*Window{w}, wm.windows...)
	wm.focused = w
}

// RemoveWindow removes a window from the manager.
func (wm *WindowManager) RemoveWindow(w *Window) {
	for i, win := range wm.windows {
		if win == w {
			wm.windows = append(wm.windows[:i], wm.windows[i+1:]...)
			if wm.focused == w {
				wm.focused = nil
				if len(wm.windows) > 0 {
					wm.focused = wm.windows[0]
				}
			}
			return
		}
	}
}

// BringToFront moves a window to the top of the z-order and focuses it.
func (wm *WindowManager) BringToFront(w *Window) {
	for i, win := range wm.windows {
		if win == w {
			wm.windows = append(wm.windows[:i], wm.windows[i+1:]...)
			wm.windows = append([]*Window{w}, wm.windows...)
			wm.focused = w
			return
		}
	}
}

// Focused returns the currently focused window, or nil.
func (wm *WindowManager) Focused() *Window { return wm.focused }

// WindowAt returns the topmost window containing screen-space point (x, y), or nil.
func (wm *WindowManager) WindowAt(x, y int) *Window {
	for _, w := range wm.windows {
		if w.Contains(x, y) {
			return w
		}
	}
	return nil
}

// Windows returns the window list (front-to-back order).
func (wm *WindowManager) Windows() []*Window {
	return wm.windows
}

// CloseMenu dismisses any open popup menu.
func (wm *WindowManager) CloseMenu() {
	wm.activeMenu = nil
}

// HasMenu returns true if a popup menu is open.
func (wm *WindowManager) HasMenu() bool {
	return wm.activeMenu != nil
}

// HandleEvent processes an input event and updates window state.
// Returns true if the event was consumed.
func (wm *WindowManager) HandleEvent(e Event) bool {
	// Always track cursor position
	if e.Type == EventMouseMove {
		wm.cursor.X = e.X
		wm.cursor.Y = e.Y
		wm.updateCursorShape(e.X, e.Y)
	}

	// If a menu is open, it gets priority
	if wm.activeMenu != nil {
		return wm.handleMenuEvent(e)
	}

	switch e.Type {
	case EventMouseDown:
		return wm.handleMouseDown(e)
	case EventMouseUp:
		return wm.handleMouseUp(e)
	case EventMouseMove:
		return wm.handleMouseMove(e)
	case EventKeyDown, EventKeyChar:
		return wm.handleKeyboard(e)
	}
	return false
}

func (wm *WindowManager) updateCursorShape(x, y int) {
	if wm.resizing != nil {
		wm.cursor.Shape = CursorResize
		return
	}
	if wm.dragging != nil {
		wm.cursor.Shape = CursorArrow
		return
	}

	w := wm.WindowAt(x, y)
	if w == nil {
		wm.cursor.Shape = CursorArrow
		return
	}

	switch w.HitTest(x, y) {
	case HitContent:
		if w.Editor != nil {
			wm.cursor.Shape = CursorText
		} else {
			wm.cursor.Shape = CursorArrow
		}
	case HitResizeGrip:
		wm.cursor.Shape = CursorResize
	default:
		wm.cursor.Shape = CursorArrow
	}
}

func (wm *WindowManager) handleMenuEvent(e Event) bool {
	m := wm.activeMenu
	switch e.Type {
	case EventMouseMove:
		m.SetHover(e.X, e.Y)
		return true
	case EventMouseDown:
		idx := m.ItemAt(e.X, e.Y)
		if idx >= 0 && m.Items[idx].Action != nil {
			action := m.Items[idx].Action
			wm.activeMenu = nil
			action()
		} else {
			wm.activeMenu = nil
		}
		return true
	case EventKeyDown:
		if ebiten.Key(e.Key) == ebiten.KeyEscape {
			wm.activeMenu = nil
			return true
		}
	}
	return false
}

func (wm *WindowManager) handleMouseDown(e Event) bool {
	// Right-click: open context menu
	if e.Button == ButtonRight {
		return wm.handleRightClick(e)
	}

	if e.Button != ButtonLeft {
		return false
	}

	w := wm.WindowAt(e.X, e.Y)
	if w == nil {
		return false
	}

	wm.BringToFront(w)

	zone := w.HitTest(e.X, e.Y)
	switch zone {
	case HitCloseBox:
		w.Closed = true
		wm.RemoveWindow(w)
		return true
	case HitCollapseBox:
		w.ToggleCollapse()
		return true
	case HitTitleBar:
		wm.dragging = w
		wm.dragOffX = e.X - w.X
		wm.dragOffY = e.Y - w.Y
		return true
	case HitResizeGrip:
		wm.resizing = w
		wm.resizeStartW = w.Width
		wm.resizeStartH = w.Height
		wm.resizeStartMX = e.X
		wm.resizeStartMY = e.Y
		return true
	case HitContent:
		// Custom click handler takes priority
		if w.OnContentClick != nil {
			lx, ly := w.ScreenToContent(e.X, e.Y)
			w.OnContentClick(lx, ly)
			w.MarkDirty()
			return true
		}
		if w.Editor != nil {
			lx, ly := w.ScreenToContent(e.X, e.Y)
			shift := ebiten.IsKeyPressed(ebiten.KeyShift)

			// Detect double-click (within 20 frames and 4px)
			dx := e.X - wm.lastClickX
			dy := e.Y - wm.lastClickY
			if wm.lastClickTick > 0 && wm.lastClickTick < 20 && dx*dx+dy*dy < 16 {
				wm.clickCount++
			} else {
				wm.clickCount = 1
			}
			wm.lastClickX = e.X
			wm.lastClickY = e.Y
			wm.lastClickTick = 0

			if wm.clickCount >= 2 {
				w.Editor.HandleDoubleClickLocal(lx, ly)
				wm.clickCount = 0
			} else {
				w.Editor.HandleClickLocal(lx, ly, shift)
			}
			w.MarkDirty()
		}
		return true
	}

	return true
}

func (wm *WindowManager) handleRightClick(e Event) bool {
	w := wm.WindowAt(e.X, e.Y)
	if w == nil {
		// Desktop right-click — world menu
		if wm.WorldMenuFunc != nil {
			items := wm.WorldMenuFunc(e.X, e.Y)
			if len(items) > 0 {
				wm.activeMenu = NewMenu(e.X, e.Y, items)
			}
		}
		return true
	}

	// Window content right-click
	zone := w.HitTest(e.X, e.Y)
	if zone == HitContent && wm.WindowMenuFunc != nil {
		items := wm.WindowMenuFunc(w, e.X, e.Y)
		if len(items) > 0 {
			wm.activeMenu = NewMenu(e.X, e.Y, items)
		}
	}
	return true
}

func (wm *WindowManager) handleMouseUp(e Event) bool {
	if wm.dragging != nil {
		wm.dragging.MarkDirty()
		wm.dragging = nil
		return true
	}
	if wm.resizing != nil {
		wm.resizing.MarkDirty()
		wm.resizing = nil
		return true
	}
	// Release scrollbar drag
	if wm.focused != nil && wm.focused.Editor != nil {
		wm.focused.Editor.HandleReleaseLocal()
		wm.focused.MarkDirty()
	}
	return false
}

func (wm *WindowManager) handleMouseMove(e Event) bool {
	if wm.dragging != nil {
		wm.dragging.X = e.X - wm.dragOffX
		wm.dragging.Y = e.Y - wm.dragOffY
		wm.dragging.MarkDirty()
		return true
	}
	if wm.resizing != nil {
		dx := e.X - wm.resizeStartMX
		dy := e.Y - wm.resizeStartMY
		newW := wm.resizeStartW + dx - borderWidth*2
		newH := wm.resizeStartH + dy - titleBarHeight - borderWidth*2
		wm.resizing.Resize(newW, newH)
		return true
	}
	// Editor drag (scrollbar or text selection)
	if wm.focused != nil && wm.focused.Editor != nil {
		lx, ly := wm.focused.ScreenToContent(e.X, e.Y)
		if wm.focused.Editor.scrollbar.IsDragging() {
			wm.focused.Editor.HandleDragLocal(lx, ly)
			wm.focused.MarkDirty()
			return true
		}
		if wm.focused.Editor.dragging {
			wm.focused.Editor.HandleDragLocal(lx, ly)
			wm.focused.MarkDirty()
			return true
		}
	}
	return false
}

func (wm *WindowManager) handleKeyboard(e Event) bool {
	if wm.focused == nil {
		return false
	}
	// Custom key handler takes priority
	if wm.focused.OnKeyEvent != nil {
		if wm.focused.OnKeyEvent(e) {
			wm.focused.MarkDirty()
			return true
		}
	}
	if wm.focused.Editor != nil {
		consumed := wm.focused.Editor.HandleEvent(e)
		if consumed {
			wm.focused.MarkDirty()
		}
		return consumed
	}
	return false
}

// Composite renders all windows onto the screen Form (back-to-front).
// Fills the background first with the given color.
func (wm *WindowManager) Composite(bgColor uint32) {
	// Tick double-click timer
	wm.lastClickTick++

	// Mark focused editor windows dirty for cursor blink
	if wm.focused != nil && wm.focused.Editor != nil {
		wm.focused.MarkDirty()
	}

	wm.screen.Fill(bgColor)

	// Draw back-to-front
	for i := len(wm.windows) - 1; i >= 0; i-- {
		w := wm.windows[i]
		form := w.Render()
		CopyBits(wm.screen, w.X, w.Y, form)
	}

	// Draw popup menu on top of everything
	if wm.activeMenu != nil {
		menuForm := wm.activeMenu.Render()
		CopyBits(wm.screen, wm.activeMenu.X, wm.activeMenu.Y, menuForm)
	}

	// Draw software cursor on top
	wm.cursor.Draw(wm.screen)
}
