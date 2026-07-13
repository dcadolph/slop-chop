// Package lsp implements a small Language Server for slop-chop. It speaks the Language Server
// Protocol over stdio with no third-party dependency: findings become diagnostics and the fix
// pass becomes a formatting edit and a code action, so any LSP-aware editor can flag and chop
// slop as you write.
package lsp

import "encoding/json"

// request is one decoded JSON-RPC message from the client. A request carries an id, a
// notification does not.
type request struct {
	// JSONRPC is the protocol version, always "2.0".
	JSONRPC string `json:"jsonrpc"`
	// ID is the request id, absent on a notification. It is echoed on the response.
	ID json.RawMessage `json:"id,omitempty"`
	// Method is the LSP method name.
	Method string `json:"method"`
	// Params is the raw params, decoded per method.
	Params json.RawMessage `json:"params"`
}

// response is a JSON-RPC response to a request id.
type response struct {
	// JSONRPC is the protocol version, always "2.0".
	JSONRPC string `json:"jsonrpc"`
	// ID echoes the request id.
	ID json.RawMessage `json:"id"`
	// Result is the method result, omitted when Error is set.
	Result any `json:"result,omitempty"`
	// Error is set when the request failed.
	Error *responseError `json:"error,omitempty"`
}

// notification is a JSON-RPC message with no id, used by the server to push diagnostics.
type notification struct {
	// JSONRPC is the protocol version, always "2.0".
	JSONRPC string `json:"jsonrpc"`
	// Method is the LSP method name.
	Method string `json:"method"`
	// Params is the method params.
	Params any `json:"params"`
}

// responseError is a JSON-RPC error object.
type responseError struct {
	// Code is the JSON-RPC error code.
	Code int `json:"code"`
	// Message is a short description of the error.
	Message string `json:"message"`
}

// Position is a zero-based line and UTF-16 character offset.
type Position struct {
	// Line is the zero-based line number.
	Line int `json:"line"`
	// Character is the zero-based UTF-16 code-unit offset within the line.
	Character int `json:"character"`
}

// Range is a span between two positions.
type Range struct {
	// Start is the first position in the span.
	Start Position `json:"start"`
	// End is the position just past the span.
	End Position `json:"end"`
}

// Diagnostic is one flagged span in a document.
type Diagnostic struct {
	// Range is where the tell sits.
	Range Range `json:"range"`
	// Severity is the diagnostic severity; slop-chop uses Information.
	Severity int `json:"severity"`
	// Source names the producer, always "slop-chop".
	Source string `json:"source"`
	// Code is the rule name that matched.
	Code string `json:"code"`
	// Message describes the tell and its fix.
	Message string `json:"message"`
}

// TextEdit replaces a range with new text.
type TextEdit struct {
	// Range is the span to replace.
	Range Range `json:"range"`
	// NewText is the replacement.
	NewText string `json:"newText"`
}

// textDocumentIdentifier names a document by URI.
type textDocumentIdentifier struct {
	// URI is the document URI.
	URI string `json:"uri"`
}

// textDocumentItem is a document with its full text, sent on open.
type textDocumentItem struct {
	// URI is the document URI.
	URI string `json:"uri"`
	// Text is the full document text.
	Text string `json:"text"`
}

// didOpenParams carries a newly opened document.
type didOpenParams struct {
	// TextDocument is the opened document.
	TextDocument textDocumentItem `json:"textDocument"`
}

// contentChange is one change to a document. With full sync, Text is the whole new document.
type contentChange struct {
	// Text is the full new document text.
	Text string `json:"text"`
}

// didChangeParams carries the changes to a document.
type didChangeParams struct {
	// TextDocument identifies the changed document.
	TextDocument textDocumentIdentifier `json:"textDocument"`
	// ContentChanges holds the changes; the last one's Text is the current document.
	ContentChanges []contentChange `json:"contentChanges"`
}

// didCloseParams carries a closed document.
type didCloseParams struct {
	// TextDocument identifies the closed document.
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

// codeActionParams asks for actions available in a document range.
type codeActionParams struct {
	// TextDocument identifies the document.
	TextDocument textDocumentIdentifier `json:"textDocument"`
	// Range is the selection or cursor span the actions apply to.
	Range Range `json:"range"`
}

// documentFormattingParams asks for edits that format a whole document.
type documentFormattingParams struct {
	// TextDocument identifies the document.
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

// publishDiagnosticsParams pushes the diagnostics for one document.
type publishDiagnosticsParams struct {
	// URI is the document the diagnostics belong to.
	URI string `json:"uri"`
	// Diagnostics is the full set of tells for the document.
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// workspaceEdit groups text edits by document URI.
type workspaceEdit struct {
	// Changes maps a document URI to its edits.
	Changes map[string][]TextEdit `json:"changes"`
}

// codeAction is one offered action, here the chop of a whole document.
type codeAction struct {
	// Title is the label shown in the editor.
	Title string `json:"title"`
	// Kind is the LSP code-action kind.
	Kind string `json:"kind"`
	// Edit is the workspace edit the action applies.
	Edit workspaceEdit `json:"edit"`
}

// serverCapabilities advertises what the server supports.
type serverCapabilities struct {
	// TextDocumentSync is 1 for full document sync.
	TextDocumentSync int `json:"textDocumentSync"`
	// CodeActionProvider reports that code actions are offered.
	CodeActionProvider bool `json:"codeActionProvider"`
	// DocumentFormattingProvider reports that whole-document formatting is offered.
	DocumentFormattingProvider bool `json:"documentFormattingProvider"`
}

// serverInfo names the server in the initialize result.
type serverInfo struct {
	// Name is the server name.
	Name string `json:"name"`
}

// initializeResult answers the initialize request.
type initializeResult struct {
	// Capabilities is what the server supports.
	Capabilities serverCapabilities `json:"capabilities"`
	// ServerInfo names the server.
	ServerInfo serverInfo `json:"serverInfo"`
}
