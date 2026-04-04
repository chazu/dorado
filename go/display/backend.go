package display

// EventType identifies the kind of input or window event.
type EventType int

const (
	EventMouseDown   EventType = iota
	EventMouseUp
	EventMouseMove
	EventKeyDown
	EventKeyUp
	EventKeyChar
	EventWindowResize
	EventWindowClose
	EventWindowFocus
)

// MouseButton identifies which mouse button was pressed.
const (
	ButtonLeft   = 1
	ButtonMiddle = 2
	ButtonRight  = 3
)

// Event represents a single input or window event.
type Event struct {
	Type   EventType
	X      int  // mouse X or window width (resize)
	Y      int  // mouse Y or window height (resize)
	Button int  // mouse button (1=left, 2=middle, 3=right)
	Key    int  // key code (ebiten key value)
	Char   rune // character for KeyChar events
}

// DisplayBackend is the rendering contract.
// Implementations translate between the Form/Event model and a platform graphics API.
type DisplayBackend interface {
	Init(width, height int) error
	BeginFrame()
	DrawForm(f *Form, x, y int)
	EndFrame()
	PollEvents() []Event
	Close()
}
