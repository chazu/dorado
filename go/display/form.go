package display

import "image"

// Form is the fundamental bitmap object in the ST-80 display model.
// A rectangular array of pixels backed by an image.RGBA buffer.
type Form struct {
	pix  *image.RGBA
	clip image.Rectangle
}

// NewForm creates a Form with the given dimensions, filled with transparent black.
func NewForm(width, height int) *Form {
	r := image.Rect(0, 0, width, height)
	return &Form{
		pix:  image.NewRGBA(r),
		clip: r,
	}
}

// NewFormFromImage wraps an existing RGBA image as a Form.
func NewFormFromImage(img *image.RGBA) *Form {
	return &Form{
		pix:  img,
		clip: img.Bounds(),
	}
}

func (f *Form) Width() int              { return f.pix.Bounds().Dx() }
func (f *Form) Height() int             { return f.pix.Bounds().Dy() }
func (f *Form) Bounds() image.Rectangle { return f.pix.Bounds() }

// Pix returns the raw RGBA pixel slice. Used by the backend for upload.
func (f *Form) Pix() []byte { return f.pix.Pix }

// Stride returns the byte stride between rows.
func (f *Form) Stride() int { return f.pix.Stride }

// Image returns the underlying image.RGBA.
func (f *Form) Image() *image.RGBA { return f.pix }

// SetClip restricts drawing operations to the given rectangle.
func (f *Form) SetClip(x, y, w, h int) {
	f.clip = image.Rect(x, y, x+w, y+h).Intersect(f.pix.Bounds())
}

// ResetClip restores the clip to the full form bounds.
func (f *Form) ResetClip() {
	f.clip = f.pix.Bounds()
}

// Clip returns the current clipping rectangle.
func (f *Form) Clip() image.Rectangle { return f.clip }

// PixelAt returns the pixel at (x,y) as a packed 0xRRGGBBAA uint32.
func (f *Form) PixelAt(x, y int) uint32 {
	if !(image.Pt(x, y).In(f.pix.Bounds())) {
		return 0
	}
	off := f.pix.PixOffset(x, y)
	return uint32(f.pix.Pix[off])<<24 |
		uint32(f.pix.Pix[off+1])<<16 |
		uint32(f.pix.Pix[off+2])<<8 |
		uint32(f.pix.Pix[off+3])
}

// SetPixelAt sets the pixel at (x,y) from a packed 0xRRGGBBAA uint32.
func (f *Form) SetPixelAt(x, y int, rgba uint32) {
	if !(image.Pt(x, y).In(f.clip)) {
		return
	}
	off := f.pix.PixOffset(x, y)
	f.pix.Pix[off] = byte(rgba >> 24)
	f.pix.Pix[off+1] = byte(rgba >> 16)
	f.pix.Pix[off+2] = byte(rgba >> 8)
	f.pix.Pix[off+3] = byte(rgba)
}

// Fill fills the entire clipping region with a packed RGBA color.
func (f *Form) Fill(rgba uint32) {
	r, g, b, a := byte(rgba>>24), byte(rgba>>16), byte(rgba>>8), byte(rgba)
	clip := f.clip
	for y := clip.Min.Y; y < clip.Max.Y; y++ {
		off := f.pix.PixOffset(clip.Min.X, y)
		for x := clip.Min.X; x < clip.Max.X; x++ {
			f.pix.Pix[off] = r
			f.pix.Pix[off+1] = g
			f.pix.Pix[off+2] = b
			f.pix.Pix[off+3] = a
			off += 4
		}
	}
}

// FillRect fills a rectangle within the form with a packed RGBA color.
func (f *Form) FillRect(rgba uint32, x, y, w int) {
	f.FillRectWH(rgba, x, y, w, w) // square by default
}

// FillRectWH fills a rectangle with explicit width and height.
// Respects the current clip region.
func (f *Form) FillRectWH(rgba uint32, x, y, w, h int) {
	rect := image.Rect(x, y, x+w, y+h).Intersect(f.clip)
	if rect.Empty() {
		return
	}
	r, g, b, a := byte(rgba>>24), byte(rgba>>16), byte(rgba>>8), byte(rgba)
	for py := rect.Min.Y; py < rect.Max.Y; py++ {
		off := f.pix.PixOffset(rect.Min.X, py)
		for px := rect.Min.X; px < rect.Max.X; px++ {
			f.pix.Pix[off] = r
			f.pix.Pix[off+1] = g
			f.pix.Pix[off+2] = b
			f.pix.Pix[off+3] = a
			off += 4
		}
	}
}

// ColorRGBA packs r, g, b, a (0-255) into a uint32.
func ColorRGBA(r, g, b, a int) uint32 {
	return uint32(r&0xFF)<<24 | uint32(g&0xFF)<<16 | uint32(b&0xFF)<<8 | uint32(a&0xFF)
}

// ColorRGB packs r, g, b (0-255) into a fully opaque uint32.
func ColorRGB(r, g, b int) uint32 {
	return ColorRGBA(r, g, b, 255)
}
