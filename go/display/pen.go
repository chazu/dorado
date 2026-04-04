package display

// DrawHLine draws a horizontal line from (x,y) for w pixels.
func DrawHLine(f *Form, x, y, w int, rgba uint32) {
	for i := 0; i < w; i++ {
		f.SetPixelAt(x+i, y, rgba)
	}
}

// DrawVLine draws a vertical line from (x,y) for h pixels.
func DrawVLine(f *Form, x, y, h int, rgba uint32) {
	for i := 0; i < h; i++ {
		f.SetPixelAt(x, y+i, rgba)
	}
}

// DrawRect draws a rectangle outline.
func DrawRect(f *Form, x, y, w, h int, rgba uint32) {
	drawRectOutline(f, x, y, w, h, rgba)
}

// DrawLine draws a line between two points using Bresenham's algorithm.
func DrawLine(f *Form, x0, y0, x1, y1 int, rgba uint32) {
	dx := x1 - x0
	dy := y1 - y0
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}

	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}

	err := dx - dy

	for {
		f.SetPixelAt(x0, y0, rgba)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// FillCircle draws a filled circle at (cx, cy) with radius r.
func FillCircle(f *Form, cx, cy, r int, rgba uint32) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				f.SetPixelAt(cx+x, cy+y, rgba)
			}
		}
	}
}

// DrawCircle draws a circle outline at (cx, cy) with radius r.
func DrawCircle(f *Form, cx, cy, r int, rgba uint32) {
	x := r
	y := 0
	err := 1 - r

	for x >= y {
		f.SetPixelAt(cx+x, cy+y, rgba)
		f.SetPixelAt(cx+y, cy+x, rgba)
		f.SetPixelAt(cx-y, cy+x, rgba)
		f.SetPixelAt(cx-x, cy+y, rgba)
		f.SetPixelAt(cx-x, cy-y, rgba)
		f.SetPixelAt(cx-y, cy-x, rgba)
		f.SetPixelAt(cx+y, cy-x, rgba)
		f.SetPixelAt(cx+x, cy-y, rgba)
		y++
		if err < 0 {
			err += 2*y + 1
		} else {
			x--
			err += 2*(y-x) + 1
		}
	}
}
