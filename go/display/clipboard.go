package display

// Clipboard is a simple in-process clipboard for cut/copy/paste.
// A system clipboard integration can be added later.
var clipboard string

// ClipboardGet returns the current clipboard content.
func ClipboardGet() string { return clipboard }

// ClipboardSet sets the clipboard content.
func ClipboardSet(s string) { clipboard = s }
