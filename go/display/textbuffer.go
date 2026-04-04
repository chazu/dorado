package display

import (
	"strings"
	"unicode"
)

// TextBuffer stores editable text with line tracking.
type TextBuffer struct {
	lines []string
}

// NewTextBuffer creates a buffer with initial text.
func NewTextBuffer(text string) *TextBuffer {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	return &TextBuffer{lines: lines}
}

// Text returns the full text content.
func (b *TextBuffer) Text() string {
	return strings.Join(b.lines, "\n")
}

// LineCount returns the number of lines.
func (b *TextBuffer) LineCount() int { return len(b.lines) }

// Line returns the text of line n (0-based). Returns "" if out of range.
func (b *TextBuffer) Line(n int) string {
	if n < 0 || n >= len(b.lines) {
		return ""
	}
	return b.lines[n]
}

// SetText replaces the entire buffer content.
func (b *TextBuffer) SetText(text string) {
	b.lines = strings.Split(text, "\n")
	if len(b.lines) == 0 {
		b.lines = []string{""}
	}
}

// RuneCount returns the total number of runes including newlines.
func (b *TextBuffer) RuneCount() int {
	n := 0
	for i, line := range b.lines {
		n += len([]rune(line))
		if i < len(b.lines)-1 {
			n++ // newline
		}
	}
	return n
}

// CursorPos represents a position in the buffer as line and column (both 0-based, rune indices).
type CursorPos struct {
	Line int
	Col  int
}

// Clamp ensures the cursor position is valid within the buffer.
func (b *TextBuffer) Clamp(pos CursorPos) CursorPos {
	if pos.Line < 0 {
		pos.Line = 0
	}
	if pos.Line >= len(b.lines) {
		pos.Line = len(b.lines) - 1
	}
	lineLen := len([]rune(b.lines[pos.Line]))
	if pos.Col < 0 {
		pos.Col = 0
	}
	if pos.Col > lineLen {
		pos.Col = lineLen
	}
	return pos
}

// Insert inserts text at the given position. Returns the new cursor position after insertion.
func (b *TextBuffer) Insert(pos CursorPos, text string) CursorPos {
	pos = b.Clamp(pos)
	lineRunes := []rune(b.lines[pos.Line])
	before := string(lineRunes[:pos.Col])
	after := string(lineRunes[pos.Col:])

	insertLines := strings.Split(text, "\n")
	if len(insertLines) == 1 {
		// Single line insert
		b.lines[pos.Line] = before + insertLines[0] + after
		return CursorPos{Line: pos.Line, Col: pos.Col + len([]rune(insertLines[0]))}
	}

	// Multi-line insert
	firstLine := before + insertLines[0]
	lastLine := insertLines[len(insertLines)-1] + after
	newCursorCol := len([]rune(insertLines[len(insertLines)-1]))

	newLines := make([]string, 0, len(b.lines)+len(insertLines)-1)
	newLines = append(newLines, b.lines[:pos.Line]...)
	newLines = append(newLines, firstLine)
	for _, mid := range insertLines[1 : len(insertLines)-1] {
		newLines = append(newLines, mid)
	}
	newLines = append(newLines, lastLine)
	newLines = append(newLines, b.lines[pos.Line+1:]...)
	b.lines = newLines

	return CursorPos{Line: pos.Line + len(insertLines) - 1, Col: newCursorCol}
}

// Delete removes text between two positions (start inclusive, end exclusive).
// Returns the start position.
func (b *TextBuffer) Delete(start, end CursorPos) CursorPos {
	start = b.Clamp(start)
	end = b.Clamp(end)

	// Normalize order
	if start.Line > end.Line || (start.Line == end.Line && start.Col > end.Col) {
		start, end = end, start
	}

	if start.Line == end.Line {
		lineRunes := []rune(b.lines[start.Line])
		b.lines[start.Line] = string(lineRunes[:start.Col]) + string(lineRunes[end.Col:])
		return start
	}

	// Multi-line delete
	startRunes := []rune(b.lines[start.Line])
	endRunes := []rune(b.lines[end.Line])
	merged := string(startRunes[:start.Col]) + string(endRunes[end.Col:])

	newLines := make([]string, 0, len(b.lines)-(end.Line-start.Line))
	newLines = append(newLines, b.lines[:start.Line]...)
	newLines = append(newLines, merged)
	newLines = append(newLines, b.lines[end.Line+1:]...)
	b.lines = newLines

	return start
}

// WordAt returns the start and end column of the word at the given position on its line.
func (b *TextBuffer) WordAt(pos CursorPos) (start, end int) {
	pos = b.Clamp(pos)
	runes := []rune(b.lines[pos.Line])
	if len(runes) == 0 {
		return 0, 0
	}
	if pos.Col >= len(runes) {
		pos.Col = len(runes) - 1
	}

	isWord := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}

	// Expand left
	start = pos.Col
	for start > 0 && isWord(runes[start-1]) {
		start--
	}

	// Expand right
	end = pos.Col
	for end < len(runes) && isWord(runes[end]) {
		end++
	}

	return start, end
}

// Selection returns the text between two positions.
func (b *TextBuffer) Selection(start, end CursorPos) string {
	start = b.Clamp(start)
	end = b.Clamp(end)

	if start.Line > end.Line || (start.Line == end.Line && start.Col > end.Col) {
		start, end = end, start
	}

	if start.Line == end.Line {
		runes := []rune(b.lines[start.Line])
		return string(runes[start.Col:end.Col])
	}

	var sb strings.Builder
	// First line
	startRunes := []rune(b.lines[start.Line])
	sb.WriteString(string(startRunes[start.Col:]))
	sb.WriteByte('\n')
	// Middle lines
	for i := start.Line + 1; i < end.Line; i++ {
		sb.WriteString(b.lines[i])
		sb.WriteByte('\n')
	}
	// Last line
	endRunes := []rune(b.lines[end.Line])
	sb.WriteString(string(endRunes[:end.Col]))

	return sb.String()
}
