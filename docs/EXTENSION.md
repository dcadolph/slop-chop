# Browser extension

Chop AI slop in any text field, on any site, without leaving the page you are on. The
extension runs the same engine as this site, compiled to WebAssembly and hosted inside the
extension. Your text never leaves the browser.

<div id="sc-ext-demo" class="sc-ext-demo" aria-hidden="true">
  <div class="sc-ext-bar">
    <span class="sc-ext-dot"></span><span class="sc-ext-dot"></span><span class="sc-ext-dot"></span>
    <span class="sc-ext-url">mail.example.com</span>
    <span id="sc-ext-score" class="sc-ext-score high">slop 80</span>
  </div>
  <div class="sc-ext-field">
    <p id="sc-ext-text" class="sc-ext-text"></p>
    <button id="sc-ext-btn" class="sc-ext-btn" tabindex="-1" aria-hidden="true">&#9986;</button>
  </div>
  <div id="sc-ext-toast" class="sc-ext-toast"></div>
</div>

## What it does

- Press the hotkey, or click the small chop button that appears at the corner of a focused
  field, and the field is rewritten in place.
- A badge shows the slop score before and after, so you see how much came out.
- The toolbar icon opens a paste-and-chop popup for one-off text.

## Install

The extension is not on the stores yet. Load it from source.

1. Build it once from the repo root with `make extension`.
2. Open `chrome://extensions` (or `edge://extensions`) and turn on Developer mode.
3. Choose **Load unpacked** and pick the `extension/` folder.

On Firefox, open `about:debugging`, choose This Firefox, then **Load Temporary Add-on** and
pick `extension/manifest.json`.

## Use it

- **Hotkey.** Focus a field and press `Ctrl+Shift+U` (`Command+Shift+U` on a Mac). Change it
  at `chrome://extensions/shortcuts`.
- **Chop button.** A small button sits at the bottom-right of the focused field. Click it for
  the same chop, no shortcut to remember.
- **Popup.** Click the toolbar icon to paste text and chop it on its own, with the score.

## Your voice

Open the options page from the popup's Settings link. It holds your voice, the same three
lists as the [command line and web app](VOICE.md):

- **Keep** protects words and phrases so no rule or preset cuts them.
- **Prefer** swaps a word or phrase to the one you want.
- **Avoid** flags your own words wherever they appear.

Pick which presets to apply, or import a `voice.json` you already have. Settings apply to
every chop, in every field.

## Privacy

The rules engine runs entirely inside the extension. No text is sent anywhere, there is no
account, and it works offline. The optional model rewrite on the web app is a separate,
opt-in feature; the extension's in-place chop is all local.

## Build from source

```
make extension          # build and stage the engine into extension/
make extension-package  # zip it for a store upload
```

See the [extension README](https://github.com/dcadolph/slop-chop/tree/main/extension) for
details.
