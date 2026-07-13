package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// TestServer drives the server over a scripted message stream and checks the initialize
// result, the diagnostics from an opened document, and the formatting edit that chops it.
func TestServer(t *testing.T) {
	t.Parallel()
	san, err := sanitize.New(sanitize.Profile{
		WordReplace:    map[string]string{"leverage": "use"},
		CollapseSpaces: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var in strings.Builder
	for _, body := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":` +
			`{"textDocument":{"uri":"file:///a","text":"we leverage it"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"textDocument/formatting","params":` +
			`{"textDocument":{"uri":"file:///a"}}}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	} {
		fmt.Fprintf(&in, "Content-Length: %d\r\n\r\n%s", len(body), body)
	}

	var out bytes.Buffer
	if err := NewServer(san, strings.NewReader(in.String()), &out).Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	frames := splitFrames(t, out.Bytes())
	if len(frames) != 3 {
		t.Fatalf("frames = %d, want 3", len(frames))
	}

	// Test 0: initialize advertises the server name and its capabilities.
	var init struct {
		Result initializeResult `json:"result"`
	}
	mustUnmarshal(t, frames[0], &init)
	if init.Result.ServerInfo.Name != "slop-chop" {
		t.Errorf("server name = %q, want slop-chop", init.Result.ServerInfo.Name)
	}
	if !init.Result.Capabilities.DocumentFormattingProvider || !init.Result.Capabilities.CodeActionProvider {
		t.Errorf("capabilities = %+v, want formatting and code actions", init.Result.Capabilities)
	}

	// Test 1: opening a document publishes a diagnostic for the tell at the right span.
	var diag struct {
		Method string                   `json:"method"`
		Params publishDiagnosticsParams `json:"params"`
	}
	mustUnmarshal(t, frames[1], &diag)
	if diag.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("method = %q, want publishDiagnostics", diag.Method)
	}
	if len(diag.Params.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(diag.Params.Diagnostics))
	}
	d := diag.Params.Diagnostics[0]
	if d.Source != "slop-chop" || d.Range.Start.Character != 3 || d.Range.End.Character != 11 {
		t.Errorf("diagnostic = %+v, want source slop-chop at 3..11", d)
	}

	// Test 2: formatting returns the whole document chopped.
	var format struct {
		Result []TextEdit `json:"result"`
	}
	mustUnmarshal(t, frames[2], &format)
	if len(format.Result) != 1 || format.Result[0].NewText != "we use it" {
		t.Errorf("format result = %+v, want one edit to \"we use it\"", format.Result)
	}
}

// splitFrames breaks a Content-Length framed stream into its message bodies.
func splitFrames(t *testing.T, b []byte) [][]byte {
	t.Helper()
	var out [][]byte
	for len(b) > 0 {
		sep := bytes.Index(b, []byte("\r\n\r\n"))
		if sep < 0 {
			break
		}
		var n int
		for _, line := range strings.Split(string(b[:sep]), "\r\n") {
			if after, ok := strings.CutPrefix(line, "Content-Length:"); ok {
				n, _ = strconv.Atoi(strings.TrimSpace(after))
			}
		}
		body := b[sep+4 : sep+4+n]
		out = append(out, body)
		b = b[sep+4+n:]
	}
	return out
}

// mustUnmarshal decodes a frame body into v or fails the test.
func mustUnmarshal(t *testing.T, body []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("unmarshal %s: %v", body, err)
	}
}
