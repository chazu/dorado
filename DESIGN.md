# WIMP — ST-80 IDE for Maggie

## Vision

A near pixel-perfect recreation of the Smalltalk-80 IDE as the native graphical environment for Maggie. The terminal-based approach (alto) fights the medium — ST-80's overlapping windows, inspectors, browsers, and BitBlt compositing don't map to terminal constraints. This project gives Maggie the kind of tight language-environment feedback loop that made Smalltalk special.

## Architecture

```
Maggie code (Browser, Inspector, Workspace, etc.)
    │
    ▼
Display facade (stable Go API) ← Maggie binds here, once
    │
    ▼
DisplayBackend interface
    │
    ├── Ebitengine backend (primary — pixel-perfect, pure Go)
    ├── Gio backend (future — resolution-independent, Wasm)
    └── others (Canvas/WebGL, terminal fallback, etc.)
```

Maggie never sees the backend. Swapping backends is a Go-side build-time or init-time decision. The wrapper layer between Maggie and Go is written once against the facade and never changes.

## Backend Evaluation

| Option | Pros | Cons | cgo? |
|--------|------|------|------|
| **Ebitengine** | Raw pixel buffer maps to ST-80 Form/BitBlt model; pure Go; battle-tested; Wasm support | Game-oriented; must implement own text rendering and event dispatch | No |
| **Gio** | Pure Go; GPU-accelerated 2D; immediate-mode fits MVC re-render model; Wasm support | Higher-level abstractions may fight pixel-perfect goals | No |
| **ctx** | Tiny; vector-based; text protocol serialization | C library; double FFI (C→cgo→Maggie); kills cross-compilation; hard to debug | Yes |
| **Web (Canvas)** | Zero native deps; cross-platform for free | Less authentic; network roundtrip for input; latency | No |

**Primary choice: Ebitengine.** Pixel-level framebuffer control matches ST-80's display model directly. Pure Go, no cgo, actively maintained.

## Core Interfaces

```go
// Form — ST-80's fundamental display primitive
type Form interface {
    Width() int
    Height() int
    PixelAt(x, y int) color.Color
    SetPixelAt(x, y int, c color.Color)
    BitBlt(src Form, srcRect, dstRect image.Rectangle, rule int)
}

// DisplayBackend — the rendering contract
type DisplayBackend interface {
    Init(width, height int) error
    BeginFrame()
    DrawForm(f Form, at image.Point)
    EndFrame()
    PollEvents() []Event
    Close()
}
```

**Design rule:** Keep the interface in terms of Forms, rects, and events — never textures, shaders, or draw ops. Backend-specific concerns (dirty-rect vs full-frame redraw, GPU upload, etc.) stay inside each backend implementation.

---

## Components to Build

### 1. Display Primitives

#### 1a. Form
The fundamental bitmap object. A rectangular array of pixels with a depth (1-bit for classic ST-80, 8/32-bit for modern use).

- Pixel storage (backed by Go `image.RGBA` or custom packed format)
- Depth conversion (1-bit ↔ 32-bit for classic look with modern backends)
- Clipping rectangles
- Cursor/origin point

#### 1b. BitBlt (Bit Block Transfer)
The core compositing operation. Copies rectangular regions between Forms using combination rules.

- 16 standard ST-80 combination rules (AND, OR, XOR, source, destination, etc.)
- Source and destination rectangles with clipping
- Halftone pattern support (for ST-80 gray fills, stipples)
- This is performance-critical — the entire display system routes through it

#### 1c. Pen / Line Drawing
Vector drawing built on top of BitBlt.

- Bresenham line drawing
- Geometric shapes (rectangles, circles, arcs)
- Fill operations
- Used by ST-80 for borders, scroll bars, selection highlights

#### 1d. CharacterScanner / Text Rendering
ST-80 rendered text from bitmap fonts stored as Forms.

- Font loading (bitmap fonts as Forms, one glyph per sub-rectangle)
- Character placement and advance width
- Text measurement (for layout)
- Paragraph composition (word wrap, alignment)
- Selection highlighting
- Consider: ship a bitmap font matching the original ST-80 typeface

### 2. Display Backend Layer

#### 2a. DisplayBackend Interface
The stable abstraction described above.

#### 2b. Ebitengine Backend
Primary implementation.

- Map Form pixels to Ebitengine `ebiten.Image`
- Implement dirty-rect tracking to avoid full-screen redraws
- Translate Ebitengine input events to the internal Event type
- Handle window resize, DPI scaling (or deliberately ignore it for pixel-perfect mode)
- Frame timing / vsync

#### 2c. Display Facade
The single Go API that Maggie binds to.

- Wraps backend selection and initialization
- Exposes Form creation, BitBlt, text rendering, and event polling
- Manages the display
- Provides the stable surface for Maggie FFI wrappers

### 3. Event System

#### 3a. Event Types
Map platform input to ST-80's event model.

- Mouse events: down, up, move, enter, exit (with button state)
- Keyboard events: key down, key up, character input
- Window events: resize, close, focus
- ST-80 used a polling model (sensor) — decide whether to preserve that or use an event queue

#### 3b. InputSensor
ST-80's input abstraction.

- Mouse position and button state
- Keyboard state and character buffer
- Timestamp for events
- Maps to `Sensor` in the Maggie object model

### 4. Window System (the "WIMP" core)

#### 4a. Window Manager
Manages overlapping windows on screen, like ST-80's ControlManager.

- Window z-ordering (front-to-back stacking)
- Window activation / focus tracking
- Window operations: move, resize, close, collapse
- Hit testing (which window is under the cursor)
- Damage tracking and





































































































 redraw coordination — when a window moves, what needs repainting
- Clipping: each window only draws within its bounds

#### 4b. Window Chrome
The visual frame around each window.

- Title bar with label
- Close box, collapse box (the ST-80 icons)
- Resize handle (bottom-right corner in ST-80)
- Scroll bars (vertical and horizontal)
- Border rendering (1px black border, ST-80 style)

#### 4c. Menus
ST-80 used pop-up menus, not menu bars.

- Pop-up menu rendering (white background, black text, highlight bar)
- Menu item selection
- Submenus (rarely used in ST-80 but present)
- The "world menu" (background click)
- Yellow-button (middle-click) context menus per window

### 5. MVC Framework

ST-80's UI was built on Model-View-Controller. This is not optional — it's the architectural spine.

#### 5a. Model
- Base Model protocol: `changed`, `changed:`, `addDependent:`, `removeDependent:`, `dependents`
- Change/update notification mechanism
- Models are pure Maggie objects — no display knowledge

#### 5b. View
- Display rectangle and coordinate transformation
- `displayOn:` — render self onto a Form
- Sub-view hierarchy (views contain views)
- Damage/

 invalidation (`invalidate`, `displayIfInvalid`)
- Border drawing
- Scrolling support (viewport offset)

#### 5c. Controller
- Input handling: `controlActivity`, `isControlActive`
- Maps mouse/keyboard input to model operations
- Each view has exactly one controller
- Controller scheduling — which controller is "in charge"
- The control loop: `controlInitialize`, `controlLoop`, `controlTerminate`

### 6. Standard IDE Tools

These are the Maggie-side applications, built on MVC.

#### 6a. System Browser
The central tool. Four-pane browser for navigating and editing code.

- Category list (top-left pane)
- Class list (top-center pane)
- Protocol list (top-right pane)
- Method list (second row)
- Code pane (bottom) with syntax highlighting
- Accept (save), cancel, and do-it/print-it/inspect-it operations
- Class vs instance side toggle

#### 6b. Workspace
A text editor pane for evaluating expressions.

- Text editing with selection
- Do it (evaluate), Print it (evaluate and insert result), Inspect it
- Cut, copy, paste
- Multiple workspaces open simultaneously

#### 6c. Inspector
Examines a single object's state.

- Left pane: instance variable names
- Right pane: value of selected variable
- Drill-down (inspect a variable's value in a new inspector)
- Evaluation pane at bottom

#### 6d. Transcript
The system output window (like stdout).

- Append-only text display
- Scrolling
- Used for system messages and `Transcript show:`

#### 6e. Debugger
Stack-based debugger for exceptions.

- Stack frame list (top pane)
- Source code display with current position highlighted
- Variable inspector per frame
- Step into, step over, restart, proceed
- Full code editing from within the debugger

#### 6f. File List / File Browser
Browse and file-in/file-out code.

- File system navigation
- File contents display
- File-in (load code from file)

### 7. Text Editing Subsystem

Text editing is pervasive in ST-80 — workspaces, browsers, inspectors all embed it.

#### 7a. ParagraphEditor
The core text editing component.

- Text storage (attributed string — text + emphasis runs)
- Cursor positioning and blinking cursor display
- Selection (character-level, word-level, line-level via click count)
- Basic editing: insert, delete, backspace
- Cut, copy, paste (clipboard)
- Word wrap and re-flow on resize
- Undo (at minimum single-level)

#### 7b. Text Display
Rendering attributed text into a Form.

- Styled text (bold, italic, different sizes — ST-80 had limited styles)
- Line layout and paragraph composition
- Scroll offset
- Selection highlight rendering

### 8. Maggie Integration Layer

#### 8a. Go Display Facade → Maggie Bindings
Wrap the Go display facade so Maggie objects can call it.

- Form creation and manipulation from Maggie
- BitBlt invocation from Maggie
- Event polling / sensor access from Maggie
- These are the only FFI bindings needed — everything else is pure Maggie

#### 8b. Maggie Object Model for Display
Maggie-side classes that mirror ST-80's display hierarchy.

- `Form`, `Pen`, `BitBlt` as Maggie classes wrapping Go primitives
- `DisplayScreen` singleton
- `CharacterScanner`, `Paragraph`, `TextStyle`

#### 8c. Maggie MVC Classes
Pure Maggie implementations.

- `Model`, `View`, `Controller` base classes
- `StandardSystemView` (a window)
- `StandardSystemController`
- `StringHolderController`, `ParagraphEditor`
- `BrowserView`, `InspectorView`, etc.

### 9. System Infrastructure

#### 9a. Cursor
- Custom cursor Forms (the ST-80 cursors: arrow, crosshair, hand, etc.)
- Cursor change on context (resize handles, text editing, etc.)

#### 9b. Global State / SystemDictionary Integration
- `Smalltalk` equivalent — the namespace
- `Transcript` global
- `Display` global (the screen Form)
- `Sensor` global (the input sensor)

#### 9c. Popup Prompters and Confirmers
- Text input dialog (FillInTheBlankMorph equivalent)
- Confirm dialog (yes/no)
- List chooser (select from options)
- These are used everywhere in the IDE (save changes?, enter class name, etc.)

---

## Build Order (suggested)

1. **Form + BitBlt** — the foundation everything else depends on
2. **Ebitengine backend** — get pixels on screen
3. **Text rendering** — bitmap font, character scanner
4. **Event system** — mouse and keyboard input flowing
5. **Window manager + chrome** — overlapping windows you can drag
6. **Text editor** — editable text panes with selection
7. **MVC framework** — Model/View/Controller wiring
8. **Maggie bindings** — expose the Go layer to Maggie
9. **Workspace** — first functional tool (eval expressions)
10. **Inspector** — second tool (examine objects)
11. **Transcript** — system output
12. **System Browser** — the big one
13. **Debugger** — requires deep VM integration
14. **Polish** — cursors, prompters, scroll bars, menu refinement

Each step is usable on its own for testing. By step 5 you have a visible windowing system. By step 9 you have a working Maggie evaluation environment.
