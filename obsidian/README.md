# slop-chop for Obsidian

Chop AI slop from your notes with the same engine as slop-chop.com, running locally in the
app. Desktop only. Your text never leaves the vault.

## Build

From the repo root:

```
make obsidian
```

This builds the wasm engine and stages it into `obsidian/engine/`.

## Install

Copy the plugin into your vault's plugins folder:

```
<vault>/.obsidian/plugins/slop-chop/
```

It needs `manifest.json`, `main.js`, and the `engine/` folder. Then turn on slop-chop under
Settings, Community plugins.

## Use

- **Chop note.** The scissors ribbon icon, or the "slop-chop: Chop note" command, chops the
  whole note in place.
- **Chop selection.** The "slop-chop: Chop selection" command chops the selected text, or the
  whole note when nothing is selected. Bind it to a hotkey under Settings, Hotkeys.

Each chop shows the slop score before and after.

## Your voice

In the plugin settings, set the preset and your voice: keep, prefer, and avoid. They apply to
every chop, the same three lists as the [command line and web app](https://slop-chop.com/VOICE.html).
