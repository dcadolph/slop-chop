# slop-chop browser extension

Chop AI slop from any text field, right where you write it. The rules engine is the same
WebAssembly build that powers slop-chop.com, running inside the extension. Your text never
leaves the browser.

## Build

From the repo root:

```
make extension
```

This builds the wasm engine and stages it into `extension/engine/`.

## Load it

1. Open `chrome://extensions` (or `edge://extensions`).
2. Turn on Developer mode.
3. Choose "Load unpacked" and pick the `extension/` folder.

Firefox: open `about:debugging`, choose This Firefox, then Load Temporary Add-on and pick
`extension/manifest.json`.

## Use it

- Focus any text field. A small chop button appears in its corner, or press the hotkey
  (`Ctrl+Shift+U`, or `Command+Shift+U` on a Mac). The field is rewritten in place and a badge
  shows the slop score before and after.
- The toolbar icon opens a paste-and-chop popup.
- Options (the popup's Settings link) hold your voice: keep, prefer, and avoid, plus which
  presets to apply. You can import a `voice.json` there too. Settings apply to every chop.

Change the shortcut at `chrome://extensions/shortcuts`.

## Package

```
make extension-package
```

Writes `slop-chop-extension.zip` for a store upload.
