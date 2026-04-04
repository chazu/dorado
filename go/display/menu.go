package display

// MenuItem represents a single entry in a popup menu.
type MenuItem struct {
	Label    string
	Action   func()
	Disabled bool
	IsSep    bool // separator line
}

// Separator returns a separator menu item.
func Separator() MenuItem {
	return MenuItem{IsSep: true}
}

// Menu is a popup menu that renders as an overlapping panel.
type Menu struct {
	Items []MenuItem
	X, Y  int  // screen position
	form  *Form
	hover int // index of hovered item, -1 for none

	// Appearance
	itemHeight int
	padX       int
	width      int
}

// NewMenu creates a popup menu at screen position (x, y).
func NewMenu(x, y int, items []MenuItem) *Menu {
	font := DefaultFont()
	padX := 12
	sepH := 7
	itemH := font.LineHeight() + 6

	// Calculate width from longest label
	maxW := 80
	for _, item := range items {
		if item.IsSep {
			continue
		}
		w := font.MeasureString(item.Label)
		if w > maxW {
			maxW = w
		}
	}
	width := maxW + padX*2

	// Calculate total height
	totalH := 4 // top padding
	for _, item := range items {
		if item.IsSep {
			totalH += sepH
		} else {
			totalH += itemH
		}
	}
	totalH += 4 // bottom padding

	return &Menu{
		Items:      items,
		X:          x,
		Y:          y,
		hover:      -1,
		itemHeight: itemH,
		padX:       padX,
		width:      width,
		form:       NewForm(width, totalH),
	}
}

// Height returns the total menu height.
func (m *Menu) Height() int { return m.form.Height() }

// Width returns the menu width.
func (m *Menu) Width() int { return m.width }

// Contains returns true if screen-space point is inside the menu.
func (m *Menu) Contains(sx, sy int) bool {
	return sx >= m.X && sx < m.X+m.width && sy >= m.Y && sy < m.Y+m.form.Height()
}

// ItemAt returns the index of the item at screen-space (sx, sy), or -1.
func (m *Menu) ItemAt(sx, sy int) int {
	if !m.Contains(sx, sy) {
		return -1
	}
	ly := sy - m.Y - 4 // top padding
	sepH := 7
	y := 0
	for i, item := range m.Items {
		h := m.itemHeight
		if item.IsSep {
			h = sepH
		}
		if ly >= y && ly < y+h {
			if item.IsSep || item.Disabled {
				return -1
			}
			return i
		}
		y += h
	}
	return -1
}

// SetHover updates which item is highlighted.
func (m *Menu) SetHover(sx, sy int) {
	m.hover = m.ItemAt(sx, sy)
}

// Render draws the menu into its form and returns it.
func (m *Menu) Render() *Form {
	f := m.form
	white := ColorRGB(255, 255, 255)
	black := ColorRGB(0, 0, 0)
	highlight := ColorRGB(40, 40, 120)
	highlightText := ColorRGB(255, 255, 255)
	gray := ColorRGB(140, 140, 140)
	sepH := 7

	// White background
	f.Fill(white)

	// Draw items
	y := 4
	for i, item := range m.Items {
		if item.IsSep {
			// Separator line
			sepY := y + sepH/2
			for x := 4; x < m.width-4; x++ {
				f.SetPixelAt(x, sepY, gray)
			}
			y += sepH
			continue
		}

		h := m.itemHeight

		// Highlight hovered item
		textColor := black
		if i == m.hover {
			f.FillRectWH(highlight, 2, y, m.width-4, h)
			textColor = highlightText
		}

		if item.Disabled {
			textColor = gray
		}

		// Label centered vertically in item
		font := DefaultFont()
		textY := y + (h-font.LineHeight())/2
		DrawString(f, m.padX, textY, item.Label, textColor)

		y += h
	}

	// 1px black border
	drawRectOutline(f, 0, 0, m.width, f.Height(), black)

	return f
}
