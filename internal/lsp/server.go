package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// severityInformation is the LSP diagnostic severity for an informational note, the level a
// prose tell warrants: visible but not an error.
const severityInformation = 3

// methodNotFound is the JSON-RPC error code for an unknown request method.
const methodNotFound = -32601

// Server is a Language Server that flags and chops slop. It reads framed JSON-RPC from r and
// writes replies and diagnostics to w. It holds each open document's text so a chop can act on
// the whole buffer.
type Server struct {
	// san is the configured rules engine.
	san *sanitize.Sanitizer
	// docs maps a document URI to its current text.
	docs map[string]string
	// r reads framed messages from the client.
	r *bufio.Reader
	// w writes framed messages to the client.
	w io.Writer
	// mu serializes writes so replies and diagnostics do not interleave.
	mu sync.Mutex
}

// NewServer builds a Server around a sanitizer, reading from r and writing to w. It panics on
// a nil sanitizer, which is a developer error.
func NewServer(san *sanitize.Sanitizer, r io.Reader, w io.Writer) *Server {
	if san == nil {
		panic("lsp.NewServer: sanitizer required")
	}
	return &Server{san: san, docs: make(map[string]string), r: bufio.NewReader(r), w: w}
}

// Run reads and handles messages until the client sends exit or the stream ends.
func (srv *Server) Run() error {
	for {
		body, err := srv.readMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		var req request
		if err := json.Unmarshal(body, &req); err != nil {
			continue
		}
		if srv.handle(req) {
			return nil
		}
	}
}

// readMessage reads one Content-Length framed message body.
func (srv *Server) readMessage() ([]byte, error) {
	length := -1
	for {
		line, err := srv.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if after, ok := strings.CutPrefix(line, "Content-Length:"); ok {
			n, err := strconv.Atoi(strings.TrimSpace(after))
			if err != nil {
				return nil, fmt.Errorf("bad Content-Length: %w", err)
			}
			length = n
		}
	}
	if length < 0 {
		return nil, errors.New("missing Content-Length header")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(srv.r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// handle dispatches one message and reports whether the server should stop.
func (srv *Server) handle(req request) bool {
	switch req.Method {
	case "initialize":
		srv.reply(req.ID, initializeResult{
			Capabilities: serverCapabilities{
				TextDocumentSync:           1,
				CodeActionProvider:         true,
				DocumentFormattingProvider: true,
			},
			ServerInfo: serverInfo{Name: "slop-chop"},
		})
	case "textDocument/didOpen":
		var p didOpenParams
		if json.Unmarshal(req.Params, &p) == nil {
			srv.docs[p.TextDocument.URI] = p.TextDocument.Text
			srv.publish(p.TextDocument.URI)
		}
	case "textDocument/didChange":
		var p didChangeParams
		if json.Unmarshal(req.Params, &p) == nil && len(p.ContentChanges) > 0 {
			srv.docs[p.TextDocument.URI] = p.ContentChanges[len(p.ContentChanges)-1].Text
			srv.publish(p.TextDocument.URI)
		}
	case "textDocument/didClose":
		var p didCloseParams
		if json.Unmarshal(req.Params, &p) == nil {
			delete(srv.docs, p.TextDocument.URI)
			srv.notify("textDocument/publishDiagnostics",
				publishDiagnosticsParams{URI: p.TextDocument.URI, Diagnostics: []Diagnostic{}})
		}
	case "textDocument/codeAction":
		var p codeActionParams
		_ = json.Unmarshal(req.Params, &p)
		srv.reply(req.ID, srv.codeActions(p.TextDocument.URI))
	case "textDocument/formatting":
		var p documentFormattingParams
		_ = json.Unmarshal(req.Params, &p)
		srv.reply(req.ID, srv.formatEdits(p.TextDocument.URI))
	case "shutdown":
		srv.reply(req.ID, json.RawMessage("null"))
	case "exit":
		return true
	default:
		if len(req.ID) > 0 {
			srv.replyError(req.ID, methodNotFound, "method not found: "+req.Method)
		}
	}
	return false
}

// publish runs the rules over a document and pushes its diagnostics.
func (srv *Server) publish(uri string) {
	text := srv.docs[uri]
	findings := srv.san.Check(text)
	diags := make([]Diagnostic, 0, len(findings))
	for _, f := range findings {
		diags = append(diags, Diagnostic{
			Range:    findingRange(text, f),
			Severity: severityInformation,
			Source:   "slop-chop",
			Code:     f.Rule,
			Message:  diagMessage(f),
		})
	}
	srv.notify("textDocument/publishDiagnostics",
		publishDiagnosticsParams{URI: uri, Diagnostics: diags})
}

// codeActions offers a single whole-document chop when it would change anything.
func (srv *Server) codeActions(uri string) []codeAction {
	edits := srv.formatEdits(uri)
	if len(edits) == 0 {
		return []codeAction{}
	}
	return []codeAction{{
		Title: "Chop the slop",
		Kind:  "quickfix",
		Edit:  workspaceEdit{Changes: map[string][]TextEdit{uri: edits}},
	}}
}

// formatEdits returns the edits that replace a document with its chopped text, or none when
// the chop changes nothing.
func (srv *Server) formatEdits(uri string) []TextEdit {
	text := srv.docs[uri]
	out, _ := srv.san.Fix(text)
	if out == text {
		return []TextEdit{}
	}
	return []TextEdit{{
		Range:   Range{Start: Position{Line: 0, Character: 0}, End: offsetToPosition(text, len(text))},
		NewText: out,
	}}
}

// reply writes a successful response for a request id.
func (srv *Server) reply(id json.RawMessage, result any) {
	_ = srv.write(response{JSONRPC: "2.0", ID: id, Result: result})
}

// replyError writes an error response for a request id.
func (srv *Server) replyError(id json.RawMessage, code int, msg string) {
	_ = srv.write(response{JSONRPC: "2.0", ID: id, Error: &responseError{Code: code, Message: msg}})
}

// notify writes a server-initiated notification.
func (srv *Server) notify(method string, params any) {
	_ = srv.write(notification{JSONRPC: "2.0", Method: method, Params: params})
}

// write frames one message as Content-Length plus its JSON body.
func (srv *Server) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if _, err := fmt.Fprintf(srv.w, "Content-Length: %d\r\n\r\n", len(b)); err != nil {
		return err
	}
	_, err = srv.w.Write(b)
	return err
}

// diagMessage renders a finding as a short diagnostic message.
func diagMessage(f sanitize.Finding) string {
	if f.Replacement != nil {
		if *f.Replacement == "" {
			return fmt.Sprintf("%q: drop", f.Match)
		}
		return fmt.Sprintf("%q -> %q", f.Match, *f.Replacement)
	}
	return fmt.Sprintf("%q: %s", f.Match, f.Rule)
}

// findingRange maps a finding's byte span to an LSP range in the document text.
func findingRange(text string, f sanitize.Finding) Range {
	return Range{
		Start: offsetToPosition(text, f.Offset),
		End:   offsetToPosition(text, f.Offset+len(f.Match)),
	}
}

// offsetToPosition converts a byte offset into a zero-based line and UTF-16 character
// position, the units LSP uses.
func offsetToPosition(text string, off int) Position {
	line, char := 0, 0
	for i, r := range text {
		if i >= off {
			break
		}
		if r == '\n' {
			line++
			char = 0
			continue
		}
		if r > 0xFFFF {
			char += 2
		} else {
			char++
		}
	}
	return Position{Line: line, Character: char}
}
