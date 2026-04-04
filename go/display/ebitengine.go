package display

import (
	"image"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// EbitengineBackend implements DisplayBackend using Ebitengine.
//
// Usage:
//
//	screen := NewForm(1280, 960)
//	backend := NewEbitengineBackend(screen)
//	backend.OnUpdate = func() { /* draw into screen each frame */ }
//	backend.Run() // blocks
type EbitengineBackend struct {
	screen   *Form
	ebiImage *ebiten.Image // reused for uploading pixels

	events  []Event
	eventMu sync.Mutex

	// OnUpdate is called once per frame before drawing.
	// Use this to poll events, update state, and draw into the screen Form.
	OnUpdate func()

	width, height int
}

// NewEbitengineBackend creates a backend that displays the given screen Form.
func NewEbitengineBackend(screen *Form) *EbitengineBackend {
	return &EbitengineBackend{
		screen: screen,
		width:  screen.Width(),
		height: screen.Height(),
	}
}

// Run starts the Ebitengine game loop. This blocks until the window is closed.
func (b *EbitengineBackend) Run() error {
	ebiten.SetWindowSize(b.width, b.height)
	ebiten.SetWindowTitle("Dorado")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	return ebiten.RunGame(b)
}

// --- ebiten.Game interface ---

func (b *EbitengineBackend) Update() error {
	b.pollInput()
	if b.OnUpdate != nil {
		b.OnUpdate()
	}
	return nil
}

func (b *EbitengineBackend) Draw(screen *ebiten.Image) {
	if b.ebiImage == nil || b.ebiImage.Bounds() != b.screen.Bounds() {
		b.ebiImage = ebiten.NewImage(b.screen.Width(), b.screen.Height())
	}
	b.ebiImage.WritePixels(b.screen.Pix())
	screen.DrawImage(b.ebiImage, nil)
}

func (b *EbitengineBackend) Layout(outsideWidth, outsideHeight int) (int, int) {
	return b.width, b.height
}

// --- DisplayBackend interface ---

func (b *EbitengineBackend) Init(width, height int) error {
	b.width = width
	b.height = height
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

	// Mouse movement
	if ebiten.CursorMode() != ebiten.CursorModeHidden {
		// Only emit move events when position changes -- but for simplicity
		// in this foundation layer, always emit. Consumers can debounce.
		b.events = append(b.events, Event{
			Type: EventMouseMove,
			X:    mx,
			Y:    my,
		})
	}

	// Mouse buttons
	for _, btn := range []ebiten.MouseButton{
		ebiten.MouseButtonLeft,
		ebiten.MouseButtonMiddle,
		ebiten.MouseButtonRight,
	} {
		magBtn := mouseButtonMap(btn)
		if inpututil.IsMouseButtonJustPressed(btn) {
			b.events = append(b.events, Event{
				Type:   EventMouseDown,
				X:      mx,
				Y:      my,
				Button: magBtn,
			})
		}
		if inpututil.IsMouseButtonJustReleased(btn) {
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
	chars := ebiten.AppendInputChars(nil)
	for _, ch := range chars {
		b.events = append(b.events, Event{
			Type: EventKeyChar,
			Char: ch,
		})
	}

	// Window resize
	w, h := ebiten.WindowSize()
	if w != b.width || h != b.height {
		b.width = w
		b.height = h
		b.events = append(b.events, Event{
			Type: EventWindowResize,
			X:    w,
			Y:    h,
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

// Ensure the Layout uses the correct Form bounds when the screen is an
// arbitrary size (e.g., after calling Init with different dimensions).
var _ image.Image = (*image.RGBA)(nil) // compile-time check
