package display

// CursorShape identifies the current cursor appearance.
type CursorShape int

const (
	CursorArrow CursorShape = iota
	CursorText
	CursorHand
	CursorResize
	CursorCrosshair
)

// Cursor manages the software cursor rendered on top of the display.
type Cursor struct {
	Shape CursorShape
	X, Y  int
	forms map[CursorShape]*Form
}

// NewCursor creates a cursor with pre-built cursor Forms.
func NewCursor() *Cursor {
	c := &Cursor{
		Shape: CursorArrow,
		forms: make(map[CursorShape]*Form),
	}
	c.forms[CursorArrow] = buildArrowCursor()
	c.forms[CursorText] = buildTextCursor()
	c.forms[CursorResize] = buildResizeCursor()
	c.forms[CursorCrosshair] = buildCrosshairCursor()
	return c
}

// Draw renders the cursor onto the screen form at its current position.
func (c *Cursor) Draw(screen *Form) {
	f := c.forms[c.Shape]
	if f == nil {
		f = c.forms[CursorArrow]
	}
	// Use XOR so cursor is visible on any background
	op := &BitBltOp{
		Dst:    screen,
		Src:    f,
		DstX:   c.X,
		DstY:   c.Y,
		Width:  f.Width(),
		Height: f.Height(),
		Rule:   RuleXor,
	}
	op.Execute()
}

func buildArrowCursor() *Form {
	// 11x16 arrow cursor
	pixels := []string{
		"#..........",
		"##.........",
		"#.#........",
		"#..#.......",
		"#...#......",
		"#....#.....",
		"#.....#....",
		"#......#...",
		"#.......#..",
		"#....#####.",
		"#..#.#.....",
		"#.#..#.....",
		"##...#.....",
		"#.....#....",
		"......#....",
		"......#....",
	}
	return cursorFromPixels(pixels, 11, 16)
}

func buildTextCursor() *Form {
	pixels := []string{
		".###.",
		"..#..",
		"..#..",
		"..#..",
		"..#..",
		"..#..",
		"..#..",
		"..#..",
		"..#..",
		"..#..",
		"..#..",
		".###.",
	}
	return cursorFromPixels(pixels, 5, 12)
}

func buildResizeCursor() *Form {
	pixels := []string{
		"......#......",
		".....###.....",
		"....#.#.#....",
		"......#......",
		"......#......",
		"..#...#...#..",
		".##...#...##.",
		"#.#.#####.#.#",
		".##...#...##.",
		"..#...#...#..",
		"......#......",
		"......#......",
		"....#.#.#....",
		".....###.....",
		"......#......",
	}
	return cursorFromPixels(pixels, 13, 15)
}

func buildCrosshairCursor() *Form {
	pixels := []string{
		"......#......",
		"......#......",
		"......#......",
		"......#......",
		"......#......",
		"......#......",
		"#############",
		"......#......",
		"......#......",
		"......#......",
		"......#......",
		"......#......",
		"......#......",
	}
	return cursorFromPixels(pixels, 13, 13)
}

func cursorFromPixels(pixels []string, w, h int) *Form {
	f := NewForm(w, h)
	black := ColorRGBA(0, 0, 0, 255)
	for y, row := range pixels {
		for x, ch := range row {
			if ch == '#' {
				f.SetPixelAt(x, y, black)
			}
		}
	}
	return f
}
