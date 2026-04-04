package display

// Scrollbar draws and handles a vertical scrollbar within a Form.
type Scrollbar struct {
	X, Y          int // position within the parent form
	Width, Height int
	Total         int // total number of items (lines)
	Visible       int // number of visible items
	Offset        int // current scroll offset

	// Drag state
	dragging   bool
	dragStartY int
	dragStartO int
}

const scrollbarWidth = 14

// NewScrollbar creates a vertical scrollbar.
func NewScrollbar(x, y, height int) *Scrollbar {
	return &Scrollbar{
		X:      x,
		Y:      y,
		Width:  scrollbarWidth,
		Height: height,
	}
}

// SetRange updates the scrollbar's total and visible counts.
func (sb *Scrollbar) SetRange(total, visible int) {
	sb.Total = total
	sb.Visible = visible
	if sb.Total <= sb.Visible {
		sb.Offset = 0
	}
	if sb.Offset > sb.Total-sb.Visible {
		sb.Offset = sb.Total - sb.Visible
	}
	if sb.Offset < 0 {
		sb.Offset = 0
	}
}

// thumbRect returns the position and height of the thumb in local coords.
func (sb *Scrollbar) thumbRect() (y, h int) {
	if sb.Total <= sb.Visible || sb.Total <= 0 {
		return 0, sb.Height
	}
	trackH := sb.Height
	thumbH := trackH * sb.Visible / sb.Total
	if thumbH < 16 {
		thumbH = 16
	}
	if thumbH > trackH {
		thumbH = trackH
	}
	maxOffset := sb.Total - sb.Visible
	if maxOffset <= 0 {
		return 0, trackH
	}
	thumbY := (trackH - thumbH) * sb.Offset / maxOffset
	return thumbY, thumbH
}

// Contains returns true if a local (form-relative) point is in the scrollbar.
func (sb *Scrollbar) Contains(lx, ly int) bool {
	return lx >= sb.X && lx < sb.X+sb.Width && ly >= sb.Y && ly < sb.Y+sb.Height
}

// HandleClick handles a mouse down in the scrollbar area (local coords).
// Returns true if consumed.
func (sb *Scrollbar) HandleClick(lx, ly int) bool {
	if !sb.Contains(lx, ly) {
		return false
	}
	ry := ly - sb.Y
	thumbY, thumbH := sb.thumbRect()

	if ry >= thumbY && ry < thumbY+thumbH {
		// Start dragging thumb
		sb.dragging = true
		sb.dragStartY = ly
		sb.dragStartO = sb.Offset
	} else if ry < thumbY {
		// Click above thumb — page up
		sb.Offset -= sb.Visible
		if sb.Offset < 0 {
			sb.Offset = 0
		}
	} else {
		// Click below thumb — page down
		sb.Offset += sb.Visible
		max := sb.Total - sb.Visible
		if max < 0 {
			max = 0
		}
		if sb.Offset > max {
			sb.Offset = max
		}
	}
	return true
}

// HandleDrag handles mouse move during drag (local coords).
func (sb *Scrollbar) HandleDrag(ly int) {
	if !sb.dragging {
		return
	}
	_, thumbH := sb.thumbRect()
	trackH := sb.Height - thumbH
	if trackH <= 0 {
		return
	}
	dy := ly - sb.dragStartY
	maxOffset := sb.Total - sb.Visible
	if maxOffset <= 0 {
		return
	}
	sb.Offset = sb.dragStartO + dy*maxOffset/trackH
	if sb.Offset < 0 {
		sb.Offset = 0
	}
	if sb.Offset > maxOffset {
		sb.Offset = maxOffset
	}
}

// HandleRelease ends a drag.
func (sb *Scrollbar) HandleRelease() {
	sb.dragging = false
}

// IsDragging returns true if the thumb is being dragged.
func (sb *Scrollbar) IsDragging() bool { return sb.dragging }

// Render draws the scrollbar into the given Form.
func (sb *Scrollbar) Render(f *Form) {
	bg := ColorRGB(220, 220, 220)
	thumb := ColorRGB(140, 140, 140)
	border := ColorRGB(100, 100, 100)

	// Track background
	f.FillRectWH(bg, sb.X, sb.Y, sb.Width, sb.Height)

	// Thumb
	thumbY, thumbH := sb.thumbRect()
	if sb.Total > sb.Visible {
		f.FillRectWH(thumb, sb.X+1, sb.Y+thumbY+1, sb.Width-2, thumbH-2)
	}

	// Border
	drawRectOutline(f, sb.X, sb.Y, sb.Width, sb.Height, border)
}
