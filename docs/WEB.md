# The web app

slop-chop.com runs the same rules engine as the CLI, compiled to WebAssembly. One
codebase, two front ends, byte-identical output. Nothing is uploaded and there is no
server: the page is static files on GitHub Pages.

## How the pieces fit

The Go engine in `internal/sanitize` compiles to `slop-chop.wasm` through the shim in
`wasm/main.go`. The page never touches the engine directly: a Web Worker owns the WASM
instance so a giant paste chops off the main thread and typing never freezes.

| Piece                  | Job                                                                    |
| ---------------------- | ----------------------------------------------------------------------- |
| `wasm/main.go`         | Registers the engine calls on the worker's global scope.&nbsp;&nbsp;    |
| `docs/assets/worker.js`| Boots the WASM, answers `{id, fn, arg}` messages from the page.         |
| `docs/assets/app.js`   | The whole UI: panes, marks, drawer, share links, connectors.            |
| `overrides/main.html`  | Social card metadata on every page.                                     |

The shim exposes four calls. `slopChop` takes text, a full profile, and preset names,
and returns the output, the findings, and the score. `slopDefaults` and `slopPresets`
feed the settings panel from the same source of truth the CLI uses. `slopRewritePrompt`
and `slopJudgePrompt` return the model instructions from `internal/rewrite/prompt`, a
package split off so the WASM build shares the CLI's exact prompts without pulling the
HTTP client into the binary. Adding a shim export means adding it to the worker's
allowlist too.

## Profile semantics

The page mirrors the CLI. The settings panel builds one profile object: the built-in
defaults merged under the user's entries, with the user winning on any key both set.
Chosen presets merge on top through the same `ApplyPresets` the CLI uses, profile
winning on conflicts. Copy profile JSON exports exactly what the page runs, and the
file works verbatim with `--profile`.

## The marks

Both panes sit on mirror layers. The input mirror places a mark under every finding,
cut on byte offsets from the engine. The output mirror runs a Myers token diff between
input and output and marks what changed, bailing to a plain mirror past 1,500 edits.
The mirrors match the textarea's font metrics and compensate for scrollbar width, so
marks line up with characters on every platform.

## Files in and out

A text file dropped on the input pane loads and chops in place, and the Download
button saves the output pane under the dropped file's name, so a file round-trips
through the chopper without a copy and paste. Text with no file behind it downloads
as `chopped.txt`. A file past two megabytes, or one that looks binary, is refused
with a message and the panes keep what they had.

## Model connectors

The rewrite pass is optional and browser-direct. Anthropic calls go straight from the
page with the user's own key and the CORS opt-in header. Any OpenAI-compatible endpoint
works for local models: Ollama, LM Studio, vLLM. Keys live in localStorage and are sent
only to the chosen endpoint. The reply streams into the output pane as the model writes
it, and a stream that dies puts the rules output back instead of leaving half a reply.
After a rewrite, the same provider judges the result against the original with the
CLI's verify prompt, the page reports whether meaning held, and a Restore button
returns the pane to the rules output when the model's version loses.

## Share links

Copy link packs the settings into the URL hash as base64 JSON. API keys are stripped
before encoding. On load, a valid hash applies the settings and cleans itself from the
URL. A mangled hash degrades to a normal visit.

## Verifying changes

`make wasm` builds the engine into `docs/assets`, `mkdocs build` assembles the site,
and the suite in `e2e/` drives the result in Chromium, Firefox, and WebKit with model
providers mocked at the network layer. The `e2e` workflow runs it on every push and
pull request that touches the site, the engine, or the shim. See the [e2e
readme](https://github.com/dcadolph/slop-chop/tree/main/e2e) for the local recipe.

## Releasing

Push a `v*` tag on the commit you want stamped. The release workflow cross-builds the
CLI, publishes the archives, and bumps the Homebrew formula. The docs workflow stamps
the wasm with `git describe`, so tagging the deployed commit itself yields a clean
version in the settings panel footer.
