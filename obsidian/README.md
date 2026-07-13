# slop-chop for Obsidian

Chop AI slop from your notes with the same engine as slop-chop.com, running locally in the
app. Desktop only. Your text never leaves the vault.

## Build

From the repo root:

```
make obsidian
```

This builds the wasm engine, gzips it, inlines it as base64 ahead of the plugin source, and
minifies the result into `obsidian/dist/`: a self-contained `main.js` plus `manifest.json`
and `versions.json`. The plugin decodes the engine in memory, so it never reads the
filesystem. Releases are built and attested by the release workflow in
[dcadolph/slop-chop-obsidian](https://github.com/dcadolph/slop-chop-obsidian).

## Install

Copy the contents of `obsidian/dist/` into your vault's plugins folder:

```
<vault>/.obsidian/plugins/slop-chop/
```

Then turn on slop-chop under Settings, Community plugins.

## Use

- **Chop note.** The scissors ribbon icon, or the "slop-chop: Chop note" command, chops the
  whole note in place.
- **Chop selection.** The "slop-chop: Chop selection" command chops the selected text, or the
  whole note when nothing is selected. Bind it to a hotkey under Settings, Hotkeys.

Each chop shows the slop score before and after.

## Your voice

In the plugin settings, set the preset and your voice: keep, prefer, and avoid. They apply to
every chop, the same three lists as the [command line and web app](https://slop-chop.com/VOICE.html).
