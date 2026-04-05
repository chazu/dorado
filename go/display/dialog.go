package display

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// Dialog is a modal popup for text input or confirmation.
type Dialog struct {
	Title   string
	Message string
	Input   string // current text input (for prompters)
	IsInput bool   // true = text input dialog, false = confirm dialog

	// Result
	Confirmed bool
	Dismissed bool

	// Callbacks
	OnConfirm func(input string)
	OnCancel  func()

	// Internal
	cursorPos int
	blinkTick int
	blinkOn   bool
	form      *Form
	width     int
	height    int
}

const (
	dialogWidth  = 360
	dialogHeight = 140
	dialogInputH = 180
)

// NewConfirmDialog creates a yes/no confirmation dialog.
func NewConfirmDialog(title, message string, onConfirm func(string), onCancel func()) *Dialog {
	return &Dialog{
		Title:     title,
		Message:   message,
		IsInput:   false,
		OnConfirm: onConfirm,
		OnCancel:  onCancel,
		width:     dialogWidth,
		height:    dialogHeight,
		form:      NewForm(dialogWidth, dialogHeight),
	}
}

// NewPromptDialog creates a text input dialog.
func NewPromptDialog(title, message string, defaultText string, onConfirm func(string), onCancel func()) *Dialog {
	return &Dialog{
		Title:     title,
		Message:   message,
		Input:     defaultText,
		IsInput:   true,
		OnConfirm: onConfirm,
		OnCancel:  onCancel,
		cursorPos: len([]rune(defaultText)),
		width:     dialogWidth,
		height:    dialogInputH,
		form:      NewForm(dialogWidth, dialogInputH),
	}
}

// HandleEvent processes input for the dialog. Returns true if consumed.
func (d *Dialog) HandleEvent(e Event) bool {
	if d.Dismissed {
		return false
	}

	switch e.Type {
	case EventKeyDown:
		k := ebiten.Key(e.Key)
		switch k {
		case ebiten.KeyEnter, ebiten.KeyNumpadEnter:
			d.Confirmed = true
			d.Dismissed = true
			if d.OnConfirm != nil {
				d.OnConfirm(d.Input)
			}
			return true
		case ebiten.KeyEscape:
			d.Dismissed = true
			if d.OnCancel != nil {
				d.OnCancel()
			}
			return true
		case ebiten.KeyBackspace:
			if d.IsInput && d.cursorPos > 0 {
				runes := []rune(d.Input)
				d.Input = string(runes[:d.cursorPos-1]) + string(runes[d.cursorPos:])
				d.cursorPos--
			}
			return true
		case ebiten.KeyDelete:
			if d.IsInput {
				runes := []rune(d.Input)
				if d.cursorPos < len(runes) {
					d.Input = string(runes[:d.cursorPos]) + string(runes[d.cursorPos+1:])
				}
			}
			return true
		case ebiten.KeyLeft:
			if d.cursorPos > 0 {
				d.cursorPos--
			}
			return true
		case ebiten.KeyRight:
			if d.cursorPos < len([]rune(d.Input)) {
				d.cursorPos++
			}
			return true
		case ebiten.KeyHome:
			d.cursorPos = 0
			return true
		case ebiten.KeyEnd:
			d.cursorPos = len([]rune(d.Input))
			return true
		}

	case EventKeyChar:
		if d.IsInput {
			if ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight) {
				return true
			}
			runes := []rune(d.Input)
			d.Input = string(runes[:d.cursorPos]) + string(e.Char) + string(runes[d.cursorPos:])
			d.cursorPos++
			return true
		}

	case EventMouseDown:
		// Check button clicks
		return d.handleClick(e.X, e.Y)
	}

	return true // consume all events while dialog is open
}

func (d *Dialog) handleClick(screenX, screenY int) bool {
	// Dialog is centered — calculate position
	// (caller passes screen coords, we need to convert)
	return true
}

// Render draws the dialog and returns its form.
func (d *Dialog) Render(screenW, screenH int) (*Form, int, int) {
	f := d.form
	font := DefaultFont()
	lh := font.LineHeight()

	white := ColorRGB(255, 255, 255)
	black := ColorRGB(0, 0, 0)
	gray := ColorRGB(180, 180, 180)
	darkGray := ColorRGB(80, 80, 80)
	btnBG := ColorRGB(200, 200, 200)
	inputBG := ColorRGB(255, 255, 255)

	// Position (centered)
	dx := (screenW - d.width) / 2
	dy := (screenH - d.height) / 2

	f.Fill(ColorRGB(240, 240, 240))

	// Title bar
	f.FillRectWH(darkGray, 0, 0, d.width, 20)
	DrawString(f, 8, 3, d.Title, white)

	// Border
	drawRectOutline(f, 0, 0, d.width, d.height, black)

	// Message
	y := 28
	for _, line := range wrapTextSimple(d.Message, d.width-24, font) {
		DrawString(f, 12, y, line, black)
		y += lh + 2
	}

	// Input field (if prompter)
	if d.IsInput {
		inputY := y + 4
		inputW := d.width - 24
		inputH := lh + 8
		f.FillRectWH(inputBG, 12, inputY, inputW, inputH)
		drawRectOutline(f, 12, inputY, inputW, inputH, gray)
		DrawString(f, 16, inputY+4, d.Input, black)

		// Cursor
		d.blinkTick++
		if d.blinkTick >= 30 {
			d.blinkTick = 0
			d.blinkOn = !d.blinkOn
		}
		if d.blinkOn {
			cx := 16 + font.MeasureString(string([]rune(d.Input)[:d.cursorPos]))
			for cy := inputY + 3; cy < inputY+inputH-3; cy++ {
				f.SetPixelAt(cx, cy, black)
			}
		}
		y = inputY + inputH + 4
	}

	// Buttons
	btnW := 80
	btnH := 24
	btnY := d.height - btnH - 10

	if d.IsInput {
		// OK and Cancel
		okX := d.width/2 - btnW - 8
		cancelX := d.width/2 + 8

		f.FillRectWH(btnBG, okX, btnY, btnW, btnH)
		drawRectOutline(f, okX, btnY, btnW, btnH, gray)
		okLabel := "OK"
		DrawString(f, okX+(btnW-font.MeasureString(okLabel))/2, btnY+5, okLabel, black)

		f.FillRectWH(btnBG, cancelX, btnY, btnW, btnH)
		drawRectOutline(f, cancelX, btnY, btnW, btnH, gray)
		cancelLabel := "Cancel"
		DrawString(f, cancelX+(btnW-font.MeasureString(cancelLabel))/2, btnY+5, cancelLabel, black)
	} else {
		// Yes and No
		yesX := d.width/2 - btnW - 8
		noX := d.width/2 + 8

		f.FillRectWH(btnBG, yesX, btnY, btnW, btnH)
		drawRectOutline(f, yesX, btnY, btnW, btnH, gray)
		yesLabel := "Yes"
		DrawString(f, yesX+(btnW-font.MeasureString(yesLabel))/2, btnY+5, yesLabel, black)

		f.FillRectWH(btnBG, noX, btnY, btnW, btnH)
		drawRectOutline(f, noX, btnY, btnW, btnH, gray)
		noLabel := "No"
		DrawString(f, noX+(btnW-font.MeasureString(noLabel))/2, btnY+5, noLabel, black)
	}

	return f, dx, dy
}

// HandleClickAt handles a click in screen-space coordinates.
func (d *Dialog) HandleClickAt(screenX, screenY, dialogX, dialogY int) bool {
	lx := screenX - dialogX
	ly := screenY - dialogY

	if lx < 0 || ly < 0 || lx >= d.width || ly >= d.height {
		return false
	}

	btnW := 80
	btnH := 24
	btnY := d.height - btnH - 10

	leftBtnX := d.width/2 - btnW - 8
	rightBtnX := d.width/2 + 8

	if ly >= btnY && ly < btnY+btnH {
		if lx >= leftBtnX && lx < leftBtnX+btnW {
			// Left button (OK/Yes)
			d.Confirmed = true
			d.Dismissed = true
			if d.OnConfirm != nil {
				d.OnConfirm(d.Input)
			}
			return true
		}
		if lx >= rightBtnX && lx < rightBtnX+btnW {
			// Right button (Cancel/No)
			d.Dismissed = true
			if d.OnCancel != nil {
				d.OnCancel()
			}
			return true
		}
	}

	return true
}

func wrapTextSimple(text string, maxW int, font *Font) []string {
	var lines []string
	for _, line := range splitLines(text) {
		if font.MeasureString(line) <= maxW {
			lines = append(lines, line)
		} else {
			// Crude word wrap
			words := splitWords(line)
			cur := ""
			for _, w := range words {
				test := cur
				if test != "" {
					test += " "
				}
				test += w
				if font.MeasureString(test) > maxW && cur != "" {
					lines = append(lines, cur)
					cur = w
				} else {
					cur = test
				}
			}
			if cur != "" {
				lines = append(lines, cur)
			}
		}
	}
	return lines
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, ch := range s {
		if ch == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

func splitWords(s string) []string {
	var words []string
	word := ""
	for _, ch := range s {
		if ch == ' ' || ch == '\t' {
			if word != "" {
				words = append(words, word)
				word = ""
			}
		} else {
			word += string(ch)
		}
	}
	if word != "" {
		words = append(words, word)
	}
	return words
}
