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

	// Double-click detection
	lastClickX    int
	lastClickY    int
	lastClickTick int
	clickCount    int
}

// NewWindowManager creates a window manager that composites onto the given screen Form.
func NewWindowManager(screen *Form) *WindowManager {
	return &WindowManager{screen: screen}
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

// HandleEvent processes an input event and updates window state.
// Returns true if the event was consumed.
func (wm *WindowManager) HandleEvent(e Event) bool {
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

func (wm *WindowManager) handleMouseDown(e Event) bool {
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
	case HitTitleBar:
		wm.dragging = w
		wm.dragOffX = e.X - w.X
		wm.dragOffY = e.Y - w.Y
		return true
	case HitContent:
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

func (wm *WindowManager) handleMouseUp(e Event) bool {
	if wm.dragging != nil {
		wm.dragging.MarkDirty()
		wm.dragging = nil
		return true
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
	return false
}

func (wm *WindowManager) handleKeyboard(e Event) bool {
	if wm.focused != nil && wm.focused.Editor != nil {
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
}
