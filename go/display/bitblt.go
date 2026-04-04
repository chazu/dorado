package display

import "image"

// BitBlt combination rules (ST-80 standard 16 rules).
// For each pixel, S=source, D=destination. The result is written to destination.
const (
	RuleClear        = 0  // 0
	RuleAnd          = 1  // S AND D
	RuleAndInverted  = 2  // S AND ~D
	RuleSource       = 3  // S (copy)
	RuleAndReverse   = 4  // ~S AND D
	RuleDestination  = 5  // D (nop)
	RuleXor          = 6  // S XOR D
	RuleOr           = 7  // S OR D
	RuleNor          = 8  // ~(S OR D)
	RuleXnor         = 9  // ~(S XOR D)
	RuleInvertDest   = 10 // ~D
	RuleOrInverted   = 11 // S OR ~D
	RuleInvertSource = 12 // ~S
	RuleOrReverse    = 13 // ~S OR D
	RuleNand         = 14 // ~(S AND D)
	RuleSet          = 15 // 1 (all bits set)
)

// BitBltOp describes a BitBlt operation between two Forms.
type BitBltOp struct {
	Dst    *Form
	Src    *Form // nil for unary ops (clear, invert dest, set)
	DstX   int
	DstY   int
	SrcX   int
	SrcY   int
	Width  int
	Height int
	Rule   int
}

// Execute performs the BitBlt operation.
func (op *BitBltOp) Execute() {
	if op.Dst == nil || op.Width <= 0 || op.Height <= 0 {
		return
	}

	// Compute effective rectangles clipped to form bounds and dst clip region
	dstBounds := op.Dst.Clip()
	dstRect := image.Rect(op.DstX, op.DstY, op.DstX+op.Width, op.DstY+op.Height)
	dstRect = dstRect.Intersect(dstBounds)
	if dstRect.Empty() {
		return
	}

	// Adjust source origin based on clipping offset
	srcOffX := dstRect.Min.X - op.DstX
	srcOffY := dstRect.Min.Y - op.DstY
	srcX := op.SrcX + srcOffX
	srcY := op.SrcY + srcOffY
	w := dstRect.Dx()
	h := dstRect.Dy()

	// Clip to source bounds if source exists
	if op.Src != nil {
		srcBounds := op.Src.Bounds()
		if srcX < srcBounds.Min.X {
			d := srcBounds.Min.X - srcX
			srcX += d
			dstRect.Min.X += d
			w -= d
		}
		if srcY < srcBounds.Min.Y {
			d := srcBounds.Min.Y - srcY
			srcY += d
			dstRect.Min.Y += d
			h -= d
		}
		if srcX+w > srcBounds.Max.X {
			w = srcBounds.Max.X - srcX
		}
		if srcY+h > srcBounds.Max.Y {
			h = srcBounds.Max.Y - srcY
		}
	}

	if w <= 0 || h <= 0 {
		return
	}

	// Get raw pixel slices and strides for direct access
	dstPix := op.Dst.pix.Pix
	dstStride := op.Dst.pix.Stride
	dstX0 := dstRect.Min.X
	dstY0 := dstRect.Min.Y

	var srcPix []byte
	var srcStride int
	if op.Src != nil {
		srcPix = op.Src.pix.Pix
		srcStride = op.Src.pix.Stride
	}

	rule := op.Rule

	// Unary rules (no source needed)
	if rule == RuleClear || rule == RuleDestination || rule == RuleInvertDest || rule == RuleSet {
		for row := 0; row < h; row++ {
			dOff := (dstY0+row)*dstStride + dstX0*4
			for col := 0; col < w; col++ {
				d := readPixel(dstPix, dOff)
				writePixel(dstPix, dOff, applyUnaryRule(rule, d))
				dOff += 4
			}
		}
		return
	}

	// Binary rules (source required)
	if op.Src == nil {
		return
	}

	// Handle overlapping src/dst: if same form and rects overlap, use temp buffer
	if op.Src == op.Dst {
		srcRect := image.Rect(srcX, srcY, srcX+w, srcY+h)
		destRect := image.Rect(dstX0, dstY0, dstX0+w, dstY0+h)
		if srcRect.Overlaps(destRect) {
			// Copy source region to temp
			tmp := make([]byte, w*h*4)
			for row := 0; row < h; row++ {
				sOff := (srcY+row)*srcStride + srcX*4
				tOff := row * w * 4
				copy(tmp[tOff:tOff+w*4], srcPix[sOff:sOff+w*4])
			}
			for row := 0; row < h; row++ {
				dOff := (dstY0+row)*dstStride + dstX0*4
				tOff := row * w * 4
				for col := 0; col < w; col++ {
					s := readPixel(tmp, tOff)
					d := readPixel(dstPix, dOff)
					writePixel(dstPix, dOff, applyRule(rule, s, d))
					dOff += 4
					tOff += 4
				}
			}
			return
		}
	}

	for row := 0; row < h; row++ {
		sOff := (srcY+row)*srcStride + srcX*4
		dOff := (dstY0+row)*dstStride + dstX0*4
		for col := 0; col < w; col++ {
			s := readPixel(srcPix, sOff)
			d := readPixel(dstPix, dOff)
			writePixel(dstPix, dOff, applyRule(rule, s, d))
			sOff += 4
			dOff += 4
		}
	}
}

func readPixel(pix []byte, off int) uint32 {
	return uint32(pix[off])<<24 | uint32(pix[off+1])<<16 | uint32(pix[off+2])<<8 | uint32(pix[off+3])
}

func writePixel(pix []byte, off int, v uint32) {
	pix[off] = byte(v >> 24)
	pix[off+1] = byte(v >> 16)
	pix[off+2] = byte(v >> 8)
	pix[off+3] = byte(v)
}

func applyRule(rule int, s, d uint32) uint32 {
	switch rule {
	case RuleClear:
		return 0
	case RuleAnd:
		return s & d
	case RuleAndInverted:
		return s & ^d
	case RuleSource:
		return s
	case RuleAndReverse:
		return ^s & d
	case RuleDestination:
		return d
	case RuleXor:
		return s ^ d
	case RuleOr:
		return s | d
	case RuleNor:
		return ^(s | d)
	case RuleXnor:
		return ^(s ^ d)
	case RuleInvertDest:
		return ^d
	case RuleOrInverted:
		return s | ^d
	case RuleInvertSource:
		return ^s
	case RuleOrReverse:
		return ^s | d
	case RuleNand:
		return ^(s & d)
	case RuleSet:
		return 0xFFFFFFFF
	}
	return d
}

func applyUnaryRule(rule int, d uint32) uint32 {
	switch rule {
	case RuleClear:
		return 0
	case RuleDestination:
		return d
	case RuleInvertDest:
		return ^d
	case RuleSet:
		return 0xFFFFFFFF
	}
	return d
}

// CopyBits is a convenience function: copy src into dst at (dstX, dstY) using RuleSource.
func CopyBits(dst *Form, dstX, dstY int, src *Form) {
	op := &BitBltOp{
		Dst:    dst,
		Src:    src,
		DstX:   dstX,
		DstY:   dstY,
		SrcX:   0,
		SrcY:   0,
		Width:  src.Width(),
		Height: src.Height(),
		Rule:   RuleSource,
	}
	op.Execute()
}
