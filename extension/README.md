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

## Use it

Focus any text field and press the hotkey: `Ctrl+Shift+U`, or `Command+Shift+U` on a Mac.
The field is rewritten in place and a badge shows the slop score before and after. Set or
change the shortcut at `chrome://extensions/shortcuts`.

## What it does today

It applies the built-in default rules and the cleaver preset, the same as the site's default.
Your voice and per-site options come next.
