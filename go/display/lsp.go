package display

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// LSPClient communicates with a Language Server Protocol server over stdio.
type LSPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	nextID atomic.Int64

	// Response tracking
	pending   map[int64]chan json.RawMessage
	pendingMu sync.Mutex

	running bool
}

// LSPCompletionItem represents a completion suggestion.
type LSPCompletionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind"`
	Detail string `json:"detail"`
}

// LSPHoverResult represents hover information.
type LSPHoverResult struct {
	Contents string
}

// NewLSPClient starts a language server process and returns a client.
// Returns nil if the LSP binary is not available.
func NewLSPClient(command string, args ...string) *LSPClient {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}
	// Discard stderr
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil
	}

	c := &LSPClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		pending: make(map[int64]chan json.RawMessage),
		running: true,
	}

	// Read responses in background
	go c.readLoop()

	return c
}

// Initialize sends the LSP initialize request.
func (c *LSPClient) Initialize(rootURI string) error {
	params := map[string]interface{}{
		"processId": nil,
		"rootUri":   rootURI,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"completion": map[string]interface{}{
					"completionItem": map[string]interface{}{
						"snippetSupport": false,
					},
				},
			},
		},
	}

	_, err := c.request("initialize", params)
	if err != nil {
		return err
	}

	// Send initialized notification
	c.notify("initialized", map[string]interface{}{})
	return nil
}

// DidOpen notifies the server that a document was opened.
func (c *LSPClient) DidOpen(uri, text string) {
	c.notify("textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": "maggie",
			"version":    1,
			"text":       text,
		},
	})
}

// DidChange notifies the server that a document changed.
func (c *LSPClient) DidChange(uri, text string, version int) {
	c.notify("textDocument/didChange", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     uri,
			"version": version,
		},
		"contentChanges": []map[string]interface{}{
			{"text": text},
		},
	})
}

// Complete requests completions at the given position.
func (c *LSPClient) Complete(uri string, line, col int) []LSPCompletionItem {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": line, "character": col},
	}

	resp, err := c.request("textDocument/completion", params)
	if err != nil {
		return nil
	}

	// Parse response — can be []CompletionItem or CompletionList
	var items []LSPCompletionItem

	// Try as array first
	if err := json.Unmarshal(resp, &items); err == nil {
		return items
	}

	// Try as CompletionList
	var list struct {
		Items []LSPCompletionItem `json:"items"`
	}
	if err := json.Unmarshal(resp, &list); err == nil {
		return list.Items
	}

	return nil
}

// Hover requests hover info at the given position.
func (c *LSPClient) Hover(uri string, line, col int) string {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": line, "character": col},
	}

	resp, err := c.request("textDocument/hover", params)
	if err != nil || resp == nil {
		return ""
	}

	var hover struct {
		Contents interface{} `json:"contents"`
	}
	if err := json.Unmarshal(resp, &hover); err != nil {
		return ""
	}

	return extractHoverText(hover.Contents)
}

// Close shuts down the LSP server.
func (c *LSPClient) Close() {
	if !c.running {
		return
	}
	c.running = false
	c.request("shutdown", nil)
	c.notify("exit", nil)
	c.stdin.Close()
	c.cmd.Wait()
}

// IsRunning returns true if the LSP process is alive.
func (c *LSPClient) IsRunning() bool {
	return c.running
}

// --- JSON-RPC ---

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int64      `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *int64           `json:"id"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *json.RawMessage `json:"error,omitempty"`
}

func (c *LSPClient) request(method string, params interface{}) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	msg := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  params,
	}

	ch := make(chan json.RawMessage, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	if err := c.send(msg); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, err
	}

	// Wait for response with timeout-like behavior
	result := <-ch
	return result, nil
}

func (c *LSPClient) notify(method string, params interface{}) {
	msg := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	c.send(msg)
}

func (c *LSPClient) send(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := c.stdin.Write(data); err != nil {
		return err
	}
	return nil
}

func (c *LSPClient) readLoop() {
	for c.running {
		// Read header
		var contentLength int
		for {
			line, err := c.stdout.ReadString('\n')
			if err != nil {
				c.running = false
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break // end of headers
			}
			if strings.HasPrefix(line, "Content-Length:") {
				lenStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
				contentLength, _ = strconv.Atoi(lenStr)
			}
		}

		if contentLength <= 0 {
			continue
		}

		// Read body
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(c.stdout, body); err != nil {
			c.running = false
			return
		}

		// Parse response
		var resp jsonRPCResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		if resp.ID != nil {
			c.pendingMu.Lock()
			ch, ok := c.pending[*resp.ID]
			if ok {
				delete(c.pending, *resp.ID)
			}
			c.pendingMu.Unlock()

			if ok {
				ch <- resp.Result
			}
		}
		// Notifications from server are ignored for now
	}
}

func extractHoverText(contents interface{}) string {
	switch v := contents.(type) {
	case string:
		return v
	case map[string]interface{}:
		if val, ok := v["value"]; ok {
			return fmt.Sprintf("%v", val)
		}
	}
	return ""
}
