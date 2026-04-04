package display

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
)

//go:embed font/cozette.bdf
var cozetteBDF []byte

// Glyph holds the bitmap data for a single character.
type Glyph struct {
	Encoding int    // Unicode code point
	Width    int    // bitmap width in pixels
	Height   int    // bitmap height in pixels
	XOffset  int    // X bearing from origin
	YOffset  int    // Y bearing from baseline (bottom-up in BDF)
	Advance  int    // horizontal advance width
	Bitmap   []byte // packed row data, 1 bit per pixel, MSB first, one byte per row (padded)
}

// Font holds a parsed bitmap font.
type Font struct {
	Ascent  int
	Descent int
	Glyphs  map[rune]*Glyph
}

// DefaultFont returns the embedded Cozette bitmap font.
// Parsed once on first call and cached.
var defaultFont *Font

func DefaultFont() *Font {
	if defaultFont == nil {
		f, err := ParseBDF(cozetteBDF)
		if err != nil {
			panic(fmt.Sprintf("failed to parse embedded font: %v", err))
		}
		defaultFont = f
	}
	return defaultFont
}

// LineHeight returns the total line height (ascent + descent).
func (f *Font) LineHeight() int {
	return f.Ascent + f.Descent
}

// GlyphFor returns the glyph for a rune, or nil if not found.
func (f *Font) GlyphFor(r rune) *Glyph {
	return f.Glyphs[r]
}

// MeasureString returns the pixel width of a string.
func (f *Font) MeasureString(s string) int {
	w := 0
	for _, r := range s {
		g := f.Glyphs[r]
		if g != nil {
			w += g.Advance
		} else {
			w += f.Glyphs[' '].Advance // fallback to space width
		}
	}
	return w
}

// ParseBDF parses a BDF font file from raw bytes.
func ParseBDF(data []byte) (*Font, error) {
	font := &Font{
		Glyphs: make(map[rune]*Glyph, 4096),
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var g *Glyph
	inBitmap := false

	for scanner.Scan() {
		line := scanner.Text()

		if inBitmap {
			if line == "ENDCHAR" {
				if g != nil && g.Encoding >= 0 {
					font.Glyphs[rune(g.Encoding)] = g
				}
				g = nil
				inBitmap = false
				continue
			}
			// Parse hex bitmap row
			if g != nil {
				b, err := strconv.ParseUint(strings.TrimSpace(line), 16, 64)
				if err == nil {
					// BDF hex is MSB-first. For widths <= 8, it's 1 byte.
					// For widths <= 16, it's 2 bytes, etc.
					hexLen := len(strings.TrimSpace(line))
					byteCount := hexLen / 2
					for i := byteCount - 1; i >= 0; i-- {
						g.Bitmap = append(g.Bitmap, byte(b>>(uint(i)*8)))
					}
				}
			}
			continue
		}

		if strings.HasPrefix(line, "FONT_ASCENT ") {
			font.Ascent, _ = strconv.Atoi(strings.TrimPrefix(line, "FONT_ASCENT "))
		} else if strings.HasPrefix(line, "FONT_DESCENT ") {
			font.Descent, _ = strconv.Atoi(strings.TrimPrefix(line, "FONT_DESCENT "))
		} else if strings.HasPrefix(line, "STARTCHAR ") {
			g = &Glyph{Encoding: -1}
		} else if strings.HasPrefix(line, "ENCODING ") {
			if g != nil {
				g.Encoding, _ = strconv.Atoi(strings.TrimPrefix(line, "ENCODING "))
			}
		} else if strings.HasPrefix(line, "DWIDTH ") {
			if g != nil {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					g.Advance, _ = strconv.Atoi(parts[1])
				}
			}
		} else if strings.HasPrefix(line, "BBX ") {
			if g != nil {
				parts := strings.Fields(line)
				if len(parts) >= 5 {
					g.Width, _ = strconv.Atoi(parts[1])
					g.Height, _ = strconv.Atoi(parts[2])
					g.XOffset, _ = strconv.Atoi(parts[3])
					g.YOffset, _ = strconv.Atoi(parts[4])
				}
			}
		} else if line == "BITMAP" {
			inBitmap = true
		}
	}

	return font, scanner.Err()
}

// DrawString renders a string onto a Form at (x, y) in the given color.
// y is the top of the text line (not the baseline).
// Returns the x position after the last character (for chaining).
func DrawString(dst *Form, x, y int, s string, rgba uint32) int {
	f := DefaultFont()
	return DrawStringFont(dst, x, y, s, rgba, f)
}

// DrawStringFont renders a string using a specific font.
func DrawStringFont(dst *Form, x, y int, s string, rgba uint32, f *Font) int {
	for _, r := range s {
		g := f.GlyphFor(r)
		if g == nil {
			g = f.GlyphFor('?') // fallback
			if g == nil {
				x += 6
				continue
			}
		}
		drawGlyph(dst, x+g.XOffset, y+f.Ascent-g.YOffset-g.Height, g, rgba)
		x += g.Advance
	}
	return x
}

// drawGlyph renders a single glyph bitmap onto a Form.
func drawGlyph(dst *Form, x, y int, g *Glyph, rgba uint32) {
	bytesPerRow := (g.Width + 7) / 8
	for row := 0; row < g.Height; row++ {
		rowStart := row * bytesPerRow
		if rowStart >= len(g.Bitmap) {
			break
		}
		for col := 0; col < g.Width; col++ {
			byteIdx := rowStart + col/8
			if byteIdx >= len(g.Bitmap) {
				break
			}
			bit := g.Bitmap[byteIdx] & (0x80 >> uint(col%8))
			if bit != 0 {
				dst.SetPixelAt(x+col, y+row, rgba)
			}
		}
	}
}
