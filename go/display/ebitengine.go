package display

import (
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// EbitengineBackend implements DisplayBackend using Ebitengine.
type EbitengineBackend struct {
	screen   *Form
	ebiImage *ebiten.Image

	events  []Event
	eventMu sync.Mutex

	// OnUpdate is called once per frame before drawing.
	OnUpdate func()

	// OnResize is called when the window size changes.
	// The screen Form is reallocated before this is called.
	OnResize func(width, height int)

	// Logical dimensions — tracks the actual usable area.
	logicalW, logicalH int
}

// NewEbitengineBackend creates a backend that displays the given screen Form.
func NewEbitengineBackend(screen *Form) *EbitengineBackend {
	return &EbitengineBackend{
		screen:   screen,
		logicalW: screen.Width(),
		logicalH: screen.Height(),
	}
}

// Run starts the Ebitengine game loop. This blocks until the window is closed.
func (b *EbitengineBackend) Run() error {
	ebiten.SetWindowSize(b.logicalW, b.logicalH)
	ebiten.SetWindowTitle("Dorado")
	// Allow resizing — we adapt the Form to match
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	return ebiten.RunGame(b)
}

// --- ebiten.Game interface ---

func (b *EbitengineBackend) Update() error {
	b.pollInput()

	// Check if window was resized by the OS/WM
	// Use the monitor-aware approach: outsideWidth from Layout is the
	// real logical size Ebitengine wants us to render at.
	if b.OnUpdate != nil {
		b.OnUpdate()
	}
	return nil
}

func (b *EbitengineBackend) Draw(screen *ebiten.Image) {
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()

	// If the Ebitengine screen size changed (window was resized), adapt
	if sw != b.logicalW || sh != b.logicalH {
		b.logicalW = sw
		b.logicalH = sh
		b.screen = NewForm(sw, sh)
		b.ebiImage = nil
		if b.OnResize != nil {
			b.OnResize(sw, sh)
		}
	}

	if b.ebiImage == nil || b.ebiImage.Bounds().Dx() != b.logicalW {
		b.ebiImage = ebiten.NewImage(b.logicalW, b.logicalH)
	}
	b.ebiImage.WritePixels(b.screen.Pix())
	screen.DrawImage(b.ebiImage, nil)
}

func (b *EbitengineBackend) Layout(outsideWidth, outsideHeight int) (int, int) {
	// Return the outside dimensions so our logical space always matches
	// the actual window. This means no scaling, no black bars.
	return outsideWidth, outsideHeight
}

// --- DisplayBackend interface ---

func (b *EbitengineBackend) Init(width, height int) error {
	b.logicalW = width
	b.logicalH = height
	return nil
}

func (b *EbitengineBackend) BeginFrame() {}

func (b *EbitengineBackend) DrawForm(f *Form, x, y int) {
	CopyBits(b.screen, x, y, f)
}

func (b *EbitengineBackend) EndFrame() {}

func (b *EbitengineBackend) PollEvents() []Event {
	b.eventMu.Lock()
	events := b.events
	b.events = nil
	b.eventMu.Unlock()
	return events
}

func (b *EbitengineBackend) Close() {}

// Screen returns the backing Form.
func (b *EbitengineBackend) Screen() *Form { return b.screen }

// --- Input polling ---

func (b *EbitengineBackend) pollInput() {
	b.eventMu.Lock()
	defer b.eventMu.Unlock()

	mx, my := ebiten.CursorPosition()

	// Clamp to logical bounds
	if mx < 0 {
		mx = 0
	}
	if my < 0 {
		my = 0
	}
	if mx >= b.logicalW {
		mx = b.logicalW - 1
	}
	if my >= b.logicalH {
		my = b.logicalH - 1
	}

	b.events = append(b.events, Event{
		Type: EventMouseMove,
		X:    mx,
		Y:    my,
	})

	// Mouse buttons
	for _, btn := range []ebiten.MouseButton{
		ebiten.MouseButtonLeft,
		ebiten.MouseButtonMiddle,
		ebiten.MouseButtonRight,
	} {
		if inpututil.IsMouseButtonJustPressed(btn) {
			magBtn := mouseButtonMap(btn)
			b.events = append(b.events, Event{
				Type:   EventMouseDown,
				X:      mx,
				Y:      my,
				Button: magBtn,
			})
		}
		if inpututil.IsMouseButtonJustReleased(btn) {
			magBtn := mouseButtonMap(btn)
			b.events = append(b.events, Event{
				Type:   EventMouseUp,
				X:      mx,
				Y:      my,
				Button: magBtn,
			})
		}
	}

	// Keyboard
	for _, key := range inpututil.AppendJustPressedKeys(nil) {
		b.events = append(b.events, Event{
			Type: EventKeyDown,
			Key:  int(key),
		})
	}
	for _, key := range inpututil.AppendJustReleasedKeys(nil) {
		b.events = append(b.events, Event{
			Type: EventKeyUp,
			Key:  int(key),
		})
	}

	// Character input
	for _, ch := range ebiten.AppendInputChars(nil) {
		b.events = append(b.events, Event{
			Type: EventKeyChar,
			Char: ch,
		})
	}
}

func mouseButtonMap(btn ebiten.MouseButton) int {
	switch btn {
	case ebiten.MouseButtonLeft:
		return ButtonLeft
	case ebiten.MouseButtonMiddle:
		return ButtonMiddle
	case ebiten.MouseButtonRight:
		return ButtonRight
	}
	return 0
}
