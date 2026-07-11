//go:build js && wasm

// Command wasm compiles the slop-chop rules engine for the browser. It registers a
// small set of functions on the JavaScript global object so slop-chop.com can chop
// text client side, with no server and no model.
package main

import (
	"encoding/json"
	"errors"
	"syscall/js"

	"github.com/dcadolph/slop-chop/internal/rewrite/prompt"
	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// version is stamped by the Makefile at build time.
var version = "dev"

// ErrRequest marks a malformed call from the page, like a missing argument or JSON
// that does not decode.
var ErrRequest = errors.New("bad request")

// chopRequest is the payload slopChop accepts, decoded from its single JSON argument.
type chopRequest struct {
	// Text is the input to clean.
	Text string `json:"text"`
	// Profile is the full profile to apply. The page builds it, defaults included, so
	// the engine sees exactly what the settings panel shows.
	Profile sanitize.Profile `json:"profile"`
	// Presets names built-in presets merged on top, with the profile winning on any
	// conflict, matching the CLI's --preset flag.
	Presets []string `json:"presets"`
}

// chopResult is what slopChop returns, encoded as JSON.
type chopResult struct {
	// Output is the cleaned text.
	Output string `json:"output"`
	// Findings lists every rule match against the original text.
	Findings []sanitize.Finding `json:"findings"`
	// Score rates the original text from 0 for clean to 100 for heavy slop.
	Score sanitize.Score `json:"score"`
}

// main registers the engine functions on the JavaScript global object and blocks
// forever, keeping the WASM instance alive for the page.
func main() {
	js.Global().Set("slopChop", js.FuncOf(chop))
	js.Global().Set("slopDefaults", js.FuncOf(defaults))
	js.Global().Set("slopPresets", js.FuncOf(presets))
	js.Global().Set("slopRewritePrompt", js.FuncOf(rewritePrompt))
	js.Global().Set("slopJudgePrompt", js.FuncOf(judgePrompt))
	js.Global().Set("slopVersion", js.FuncOf(engineVersion))
	select {}
}

// chop runs the rules pass. It takes one JSON string argument shaped like chopRequest
// and returns a JSON string shaped like chopResult, or {"error": "..."} when the
// request or the profile does not hold up.
func chop(_ js.Value, args []js.Value) any {
	if len(args) != 1 {
		return errJSON(errors.Join(ErrRequest, errors.New("slopChop takes one JSON argument")))
	}
	var req chopRequest
	if err := json.Unmarshal([]byte(args[0].String()), &req); err != nil {
		return errJSON(errors.Join(ErrRequest, err))
	}
	profile := req.Profile
	if len(req.Presets) > 0 {
		merged, err := sanitize.ApplyPresets(profile, req.Presets...)
		if err != nil {
			return errJSON(err)
		}
		profile = merged
	}
	s, err := sanitize.New(profile)
	if err != nil {
		return errJSON(err)
	}
	out, findings := s.Fix(req.Text)
	return marshal(chopResult{
		Output:   out,
		Findings: orEmpty(findings),
		Score:    s.Score(req.Text),
	})
}

// defaults returns the built-in default profile as JSON, so the page can render the
// settings panel from the same source of truth the CLI uses.
func defaults(_ js.Value, _ []js.Value) any {
	return marshal(sanitize.DefaultProfile())
}

// presets returns the built-in preset names as a JSON array.
func presets(_ js.Value, _ []js.Value) any {
	return marshal(sanitize.PresetNames())
}

// rewritePrompt returns the system prompt for the model rewrite pass as
// {"system": "..."}. It takes the profile JSON so the profile's tone notes shape the
// prompt, exactly as they do in the CLI. The page sends the prompt to whichever model
// backend the user configured.
func rewritePrompt(_ js.Value, args []js.Value) any {
	if len(args) != 1 {
		return errJSON(errors.Join(ErrRequest, errors.New("slopRewritePrompt takes one JSON argument")))
	}
	var p sanitize.Profile
	if err := json.Unmarshal([]byte(args[0].String()), &p); err != nil {
		return errJSON(errors.Join(ErrRequest, err))
	}
	return marshal(map[string]string{"system": prompt.System(p.Tone, nil)})
}

// judgePrompt returns the system prompt for the meaning check as {"system": "..."},
// the same instruction the CLI's --verify pass sends.
func judgePrompt(_ js.Value, _ []js.Value) any {
	return marshal(map[string]string{"system": prompt.Judge()})
}

// engineVersion returns the stamped build version.
func engineVersion(_ js.Value, _ []js.Value) any {
	return version
}

// marshal encodes v as a JSON string for the page, falling back to an error payload
// when encoding fails.
func marshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return errJSON(err)
	}
	return string(b)
}

// errJSON wraps an error in the {"error": "..."} payload the page checks for.
func errJSON(err error) string {
	b, mErr := json.Marshal(map[string]string{"error": err.Error()})
	if mErr != nil {
		return `{"error":"encode failed"}`
	}
	return string(b)
}

// orEmpty returns a non-nil slice so the JSON shows an empty array instead of null.
func orEmpty(f []sanitize.Finding) []sanitize.Finding {
	if f == nil {
		return []sanitize.Finding{}
	}
	return f
}
