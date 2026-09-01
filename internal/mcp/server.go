// Package mcp serves the slop-chop engine over the Model Context Protocol, so any MCP client
// can chop text without shelling out to the command line. The rules pass runs in this process
// and sends nothing anywhere. The model rewrite runs only when a call asks for it.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dcadolph/slop-chop/internal/jsonutil"
	"github.com/dcadolph/slop-chop/sanitize"
)

// serverName is what the server calls itself in the protocol handshake, and the name a client
// shows next to its tools.
const serverName = "slop-chop"

// maxTextBytes bounds the text one call may carry, matching the hosted API's cap. Anything
// larger is a mistake rather than a draft, and refusing it keeps one call from pinning the
// process on a pathological input.
const maxTextBytes = 1 << 20

// Sanitizers builds the rules engine for one call. The presets and dialect are that call's
// overrides, and empty values leave the server's own configuration standing.
type Sanitizers interface {
	Sanitizer(presets []string, dialect string) (*sanitize.Sanitizer, sanitize.Profile, error)
}

// SanitizerFunc adapts an ordinary function to Sanitizers.
type SanitizerFunc func(presets []string, dialect string) (*sanitize.Sanitizer, sanitize.Profile, error)

// Sanitizer calls f.
func (f SanitizerFunc) Sanitizer(presets []string, dialect string) (*sanitize.Sanitizer, sanitize.Profile, error) {
	return f(presets, dialect)
}

// Rewriter runs the optional model pass over text the rules have already cleaned. The tone
// lines carry the caller's voice notes, so the model's output sounds like them.
type Rewriter interface {
	Rewrite(ctx context.Context, text string, tone []string) (string, error)
}

// RewriterFunc adapts an ordinary function to Rewriter.
type RewriterFunc func(ctx context.Context, text string, tone []string) (string, error)

// Rewrite calls f.
func (f RewriterFunc) Rewrite(ctx context.Context, text string, tone []string) (string, error) {
	return f(ctx, text, tone)
}

// Fingerprints supplies the writer's measured fingerprint for one call, so the drift tool
// reads whatever the voice file holds now rather than a copy taken at launch.
type Fingerprints interface {
	Fingerprint() (sanitize.Fingerprint, error)
}

// FingerprintFunc adapts an ordinary function to Fingerprints.
type FingerprintFunc func() (sanitize.Fingerprint, error)

// Fingerprint calls f.
func (f FingerprintFunc) Fingerprint() (sanitize.Fingerprint, error) { return f() }

// Server serves the chop, check, presets, and drift tools over the Model Context Protocol.
type Server struct {
	// sdk is the protocol server the tools are registered on.
	sdk *mcpsdk.Server
	// sanitizers builds the rules engine for each call.
	sanitizers Sanitizers
	// rewriter runs the model pass. It is nil when no backend is wired, which makes a call
	// asking for the model pass an error rather than a silent deterministic-only chop.
	rewriter Rewriter
	// fingerprints resolves the writer's fingerprint for the drift tool. It is nil when no
	// voice is wired, which makes a drift call an error rather than a silent pass.
	fingerprints Fingerprints
}

// NewServer builds a Server serving the tools from sanitizers, reporting version in the
// handshake. A nil rewriter refuses any call that asks for the model pass instead of quietly
// skipping it, and a nil fingerprints does the same for drift. It panics on a nil Sanitizers,
// which is a developer error.
func NewServer(sanitizers Sanitizers, rewriter Rewriter, fingerprints Fingerprints, version string) *Server {
	if sanitizers == nil {
		panic("mcp.NewServer: Sanitizers required")
	}
	srv := &Server{
		sdk: mcpsdk.NewServer(&mcpsdk.Implementation{
			Name:    serverName,
			Title:   serverName,
			Version: version,
		}, nil),
		sanitizers:   sanitizers,
		rewriter:     rewriter,
		fingerprints: fingerprints,
	}
	srv.register()
	return srv
}

// Run serves the tools over stdin and stdout until the client disconnects or ctx is canceled.
// A client closing the pipe is how this server is meant to end, so an end of stream and a
// canceled context are both a clean return: reporting them as failures would have every
// normal shutdown log an error and exit non-zero.
func (srv *Server) Run(ctx context.Context) error {
	err := srv.sdk.Run(ctx, &mcpsdk.StdioTransport{})
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// Connect attaches the server to a transport and returns the session. Run covers the stdio
// case; this is the seam a test uses to drive the tools in process over an in-memory pair.
func (srv *Server) Connect(ctx context.Context, t mcpsdk.Transport) (*mcpsdk.ServerSession, error) {
	return srv.sdk.Connect(ctx, t, nil)
}

// chopInput is the argument object of the chop tool.
type chopInput struct {
	// Text is the text to clean.
	Text string `json:"text" jsonschema:"The text to clean. Required."`
	// Presets names built-in packs to apply for this call in place of the server's own, such
	// as cleaver for the aggressive swaps or plain for corporate phrasing.
	Presets []string `json:"presets,omitempty" jsonschema:"Built-in preset packs to apply for this call, replacing the server defaults. Call the presets tool for the list."`
	// Dialect enforces a spelling variant for this call.
	Dialect string `json:"dialect,omitempty" jsonschema:"Spelling variant to enforce: american, british, or off."`
	// ModelRewrite adds the optional model pass on top of the rules. It is off unless the
	// call asks for it, since it needs a provider key and costs money.
	ModelRewrite bool `json:"model_rewrite,omitempty" jsonschema:"Run the optional model rewrite after the rules pass. Off by default. It needs a provider API key and makes a paid call, so ask for it only when the deterministic clean is not enough."`
}

// chopOutput is what the chop tool returns.
type chopOutput struct {
	// Text is the cleaned text.
	Text string `json:"text"`
	// Findings is every tell the rules found in the input.
	Findings []sanitize.Finding `json:"findings"`
	// Score rates the input from 0 for clean to 100 for heavy slop.
	Score int `json:"score"`
	// ScoreAfter is the same rating for the cleaned text, so the caller sees the movement.
	ScoreAfter int `json:"scoreAfter"`
	// ModelRewrite reports whether the optional model pass ran.
	ModelRewrite bool `json:"modelRewrite"`
}

// checkInput is the argument object of the check tool.
type checkInput struct {
	// Text is the text to scan.
	Text string `json:"text" jsonschema:"The text to scan for AI writing tells. Required."`
	// Presets names built-in packs to apply for this call in place of the server's own.
	Presets []string `json:"presets,omitempty" jsonschema:"Built-in preset packs to apply for this call, replacing the server defaults. Call the presets tool for the list."`
	// Dialect enforces a spelling variant for this call.
	Dialect string `json:"dialect,omitempty" jsonschema:"Spelling variant to enforce: american, british, or off."`
}

// checkOutput is what the check tool returns.
type checkOutput struct {
	// Findings is every tell found in the text, each with its rule, position, and the swap
	// the chop tool would make.
	Findings []sanitize.Finding `json:"findings"`
	// Score rates the text from 0 for clean to 100 for heavy slop.
	Score int `json:"score"`
}

// driftInput is the argument object of the drift tool.
type driftInput struct {
	// Text is the draft to measure against the writer's fingerprint.
	Text string `json:"text" jsonschema:"The draft to measure against the user's own writing. Required. Needs a paragraph or more to read anything."`
}

// driftOutput is what the drift tool returns.
type driftOutput struct {
	// Drift is every trait that landed outside the writer's range, the furthest out first.
	Drift []sanitize.Drift `json:"drift"`
	// Traits is how many traits were measured, so a caller can read the drift count against it.
	Traits int `json:"traits"`
	// ReadsLikeYou reports whether every measured trait landed inside the writer's range.
	ReadsLikeYou bool `json:"readsLikeYou"`
}

// presetsInput takes no arguments. The protocol wants an object schema, so it is an empty
// struct rather than nothing at all.
type presetsInput struct{}

// presetsOutput is what the presets tool returns.
type presetsOutput struct {
	// Presets is the built-in preset names, sorted.
	Presets []string `json:"presets"`
}

// register adds the three tools to the protocol server. The descriptions are written in the
// words people use when they ask for this, so a client's tool picker surfaces them.
func (srv *Server) register() {
	mcpsdk.AddTool(srv.sdk, &mcpsdk.Tool{
		Name:  "chop",
		Title: "Chop the slop",
		Description: "Rewrite text so it reads like a person wrote it instead of an AI. " +
			"Use it to de-slop or humanize AI-generated text, remove AI tells from a draft, " +
			"strip em-dashes, or hold writing to a house style. It cuts em-dash splices, " +
			"semicolon habits, hedging and filler transitions, stock openers such as " +
			"\"In summary\", the listicle reflex, and buzzwords such as \"delve\", " +
			"\"leverage\", and \"robust\". The rules pass is deterministic, free, and runs " +
			"on this machine with nothing uploaded, so the same input always gives the same " +
			"output. Markdown code blocks and inline backtick spans are left alone. Returns " +
			"the cleaned text, the tells it found, and the slop score before and after.",
	}, srv.chop)

	mcpsdk.AddTool(srv.sdk, &mcpsdk.Tool{
		Name:  "check",
		Title: "Report the AI tells",
		Description: "Report the AI writing tells in text without changing a word. Use it to " +
			"find out whether a draft reads as AI-generated, and where. Each finding carries " +
			"the rule that matched, the exact text, its line and column, and the swap the " +
			"chop tool would make. It also returns a slop score from 0 for clean to 100 for " +
			"heavy slop. Deterministic, free, and local: nothing is uploaded.",
	}, srv.check)

	mcpsdk.AddTool(srv.sdk, &mcpsdk.Tool{
		Name:  "presets",
		Title: "List the preset packs",
		Description: "List the built-in preset packs the chop and check tools accept. A preset " +
			"overlays extra rules on the standard profile, such as cleaver for the aggressive " +
			"swaps or plain for turning corporate phrasing into plain English.",
	}, srv.presets)

	mcpsdk.AddTool(srv.sdk, &mcpsdk.Tool{
		Name:  "drift",
		Title: "Say whether a draft sounds like the user",
		Description: "Measure a draft against the user's own writing and report where it " +
			"stops sounding like them. Use it before handing over a draft you wrote for " +
			"them, or to check whether text matches their voice. It compares sentence " +
			"rhythm, punctuation habits, and register against the fingerprint the user " +
			"measured from their own writing, and names each trait that landed outside " +
			"their range, such as longer sentences or a heavier vocabulary. Drift is not " +
			"slop: a trait outside the range can be good writing that is not theirs, so " +
			"nothing is rewritten. It needs a fingerprint, which the user makes by running " +
			"slop-chop voice fingerprint on their own writing. Deterministic, free, and " +
			"local.",
	}, srv.drift)
}

// chop cleans the text and returns it, with the tells found and the score movement. The
// cleaned text is the content block, so a caller can use the reply directly, and the detail
// rides along as structured output.
func (srv *Server) chop(ctx context.Context, _ *mcpsdk.CallToolRequest, in chopInput) (
	*mcpsdk.CallToolResult, chopOutput, error) {
	san, profile, err := srv.engine(in.Text, in.Presets, in.Dialect)
	if err != nil {
		return nil, chopOutput{}, err
	}
	cleaned, findings := san.Fix(in.Text)
	if in.ModelRewrite {
		cleaned, err = srv.modelPass(ctx, san, cleaned, profile.Tone)
		if err != nil {
			return nil, chopOutput{}, err
		}
	}
	out := chopOutput{
		Text:         cleaned,
		Findings:     jsonutil.OrEmpty(findings),
		Score:        san.Score(in.Text).Value,
		ScoreAfter:   san.Score(cleaned).Value,
		ModelRewrite: in.ModelRewrite,
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: cleaned}},
	}, out, nil
}

// check reports the tells in the text and scores it, leaving the text alone.
func (srv *Server) check(_ context.Context, _ *mcpsdk.CallToolRequest, in checkInput) (
	*mcpsdk.CallToolResult, checkOutput, error) {
	san, _, err := srv.engine(in.Text, in.Presets, in.Dialect)
	if err != nil {
		return nil, checkOutput{}, err
	}
	findings := san.Check(in.Text)
	return nil, checkOutput{Findings: jsonutil.OrEmpty(findings), Score: san.Score(in.Text).Value}, nil
}

// presets lists the built-in preset names.
func (srv *Server) presets(_ context.Context, _ *mcpsdk.CallToolRequest, _ presetsInput) (
	*mcpsdk.CallToolResult, presetsOutput, error) {
	return nil, presetsOutput{Presets: sanitize.PresetNames()}, nil
}

// drift measures the text against the writer's fingerprint and names what reads unlike
// them. The summary is the content block, so a caller can act on the reply directly, and
// the measurements ride along as structured output.
func (srv *Server) drift(_ context.Context, _ *mcpsdk.CallToolRequest, in driftInput) (
	*mcpsdk.CallToolResult, driftOutput, error) {
	if srv.fingerprints == nil {
		return nil, driftOutput{}, fmt.Errorf("drift needs a voice: this server was started without one")
	}
	if err := checkText(in.Text); err != nil {
		return nil, driftOutput{}, err
	}
	f, err := srv.fingerprints.Fingerprint()
	if err != nil {
		return nil, driftOutput{}, err
	}
	drifts, err := f.Compare(in.Text)
	if err != nil {
		return nil, driftOutput{}, err
	}
	out := driftOutput{
		Drift:        jsonutil.OrEmpty(drifts),
		Traits:       len(f.Metrics),
		ReadsLikeYou: len(drifts) == 0,
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: driftSummary(drifts, len(f.Metrics))}},
	}, out, nil
}

// driftSummary writes the report a caller reads as prose.
func driftSummary(drifts []sanitize.Drift, traits int) string {
	if len(drifts) == 0 {
		return fmt.Sprintf("This reads like the user on all %d measured traits.", traits)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "This reads unlike the user on %d of %d traits:", len(drifts), traits)
	for _, d := range drifts {
		fmt.Fprintf(&sb, "\n- %s (%.2f against their %.2f, %s)", d.Note, d.Got, d.Want, d.Unit)
	}
	return sb.String()
}

// engine validates the text and builds the rules engine for one call. A bad preset or dialect
// comes back as a tool error naming what was wrong, so the caller can correct itself rather
// than seeing the call fail at the protocol level.
func (srv *Server) engine(text string, presets []string, dialect string) (
	*sanitize.Sanitizer, sanitize.Profile, error) {
	if err := checkText(text); err != nil {
		return nil, sanitize.Profile{}, err
	}
	san, profile, err := srv.sanitizers.Sanitizer(presets, dialect)
	if err != nil {
		return nil, sanitize.Profile{}, err
	}
	return san, profile, nil
}

// modelPass runs the optional model rewrite over the rules output and re-runs the rules over
// the reply. A model can undo the rules or reintroduce the tells they just cut, so the text
// that comes back is always cleaned again before it leaves. With no rewriter wired the call
// is refused, since silently returning the deterministic clean would look like the model pass
// ran when it never did.
func (srv *Server) modelPass(ctx context.Context, san *sanitize.Sanitizer, cleaned string,
	tone []string) (string, error) {
	if srv.rewriter == nil {
		return "", fmt.Errorf("the model rewrite is not configured: " +
			"set ANTHROPIC_API_KEY, or start the server with --provider and --base-url " +
			"for a local model")
	}
	rewritten, err := srv.rewriter.Rewrite(ctx, cleaned, tone)
	if err != nil {
		return "", fmt.Errorf("model rewrite: %w", err)
	}
	out, _ := san.Fix(rewritten)
	return out, nil
}

// checkText rejects the inputs no tool can do anything with: nothing at all, and more text
// than one call is meant to carry.
func checkText(text string) error {
	if text == "" {
		return fmt.Errorf("text is required")
	}
	if len(text) > maxTextBytes {
		return fmt.Errorf("text is %d bytes, over the %d byte limit", len(text), maxTextBytes)
	}
	return nil
}
