/* slop-chop for Obsidian. Chops AI slop from a note or a selection with the same rules
   engine as slop-chop.com, loaded from the plugin folder as WebAssembly. Desktop only, since
   it reads the engine off disk. Your text never leaves the vault. Plain CommonJS: Obsidian
   provides the "obsidian" module at runtime, so there is no build step. */
"use strict";

const { Plugin, PluginSettingTab, Setting, Notice, MarkdownView } = require("obsidian");

// DEFAULT_SETTINGS is the plugin's saved state before the user changes anything.
const DEFAULT_SETTINGS = {
  preset: "cleaver",
  voiceKeep: "",
  voicePrefer: "",
  voiceAvoid: "",
};

// parseLines splits a textarea into trimmed, non-empty lines.
function parseLines(text) {
  return (text || "")
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);
}

// parsePairs reads "from => to" lines into a map, an empty to meaning drop.
function parsePairs(text) {
  const out = {};
  for (const line of parseLines(text)) {
    const i = line.indexOf("=>");
    const from = (i < 0 ? line : line.slice(0, i)).trim();
    const to = i < 0 ? "" : line.slice(i + 2).trim();
    if (from) out[from] = to;
  }
  return out;
}

// dedupe returns the array with duplicates dropped, order kept.
function dedupe(arr) {
  return [...new Set(arr)];
}

// replaceAll swaps the whole editor contents while keeping the scroll position and a
// clamped cursor, so a chop does not jump the view back to the top of the note.
function replaceAll(editor, output) {
  const cursor = editor.getCursor();
  const scroll = editor.getScrollInfo();
  editor.setValue(output);
  const line = Math.min(cursor.line, editor.lastLine());
  const ch = Math.min(cursor.ch, editor.getLine(line).length);
  editor.setCursor({ line, ch });
  editor.scrollTo(scroll.left, scroll.top);
}

// SlopChopPlugin wires the engine to Obsidian commands and a settings tab.
class SlopChopPlugin extends Plugin {
  // onload boots the engine and registers the ribbon, commands, and settings.
  async onload() {
    this.settings = Object.assign({}, DEFAULT_SETTINGS, await this.loadData());
    this.engineReady = false;
    this.bootPromise = null;
    try {
      await this.bootEngine();
    } catch (err) {
      new Notice("slop-chop: engine failed to load: " + (err && err.message ? err.message : err));
    }

    this.addRibbonIcon("scissors", "Chop the slop", () => this.chopActiveFile());
    this.addCommand({ id: "chop-note", name: "Chop note", callback: () => this.chopActiveFile() });
    this.addCommand({
      id: "chop-selection",
      name: "Chop selection",
      editorCallback: (editor) => this.chopSelection(editor),
    });
    this.addSettingTab(new SlopChopSettingTab(this.app, this));
  }

  // bootEngine instantiates the wasm engine once and caches the default profile. Concurrent
  // callers share a single in-flight boot, so a chop fired before onload finished booting
  // cannot start a second Go runtime. A failed boot clears the guard so a later call retries.
  bootEngine() {
    if (this.engineReady) return Promise.resolve();
    if (!this.bootPromise) {
      this.bootPromise = this.instantiateEngine().then(
        () => {
          this.engineReady = true;
          this.bootPromise = null;
        },
        (err) => {
          this.bootPromise = null;
          throw err;
        },
      );
    }
    return this.bootPromise;
  }

  // instantiateEngine loads and starts the wasm engine, caching the default profile. The
  // published build inlines the Go runtime and a base64 wasm ahead of this file, so it runs
  // with no disk access; the in-repo build reads both from the plugin's engine folder.
  async instantiateEngine() {
    let bytes;
    if (globalThis.SLOP_WASM_B64) {
      bytes = Uint8Array.from(atob(globalThis.SLOP_WASM_B64), (c) => c.charCodeAt(0));
    } else {
      const fs = require("fs");
      const path = require("path");
      const adapter = this.app.vault.adapter;
      const base =
        typeof adapter.getBasePath === "function" ? adapter.getBasePath() : adapter.basePath;
      const dir = path.join(base, this.manifest.dir, "engine");
      (0, eval)(fs.readFileSync(path.join(dir, "wasm_exec.js"), "utf8"));
      bytes = fs.readFileSync(path.join(dir, "slop-chop.wasm"));
    }
    const go = new globalThis.Go();
    const result = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(result.instance);
    await new Promise((r) => setTimeout(r, 0));
    this.defaults = JSON.parse(globalThis.slopDefaults());
  }

  // presets returns the active preset list.
  presets() {
    return this.settings.preset ? [this.settings.preset] : [];
  }

  // voiceProfile folds the saved voice into the default profile: keep into allow, avoid into
  // blockWords, and prefer into word or phrase swaps. Voice wins over the presets.
  voiceProfile() {
    const base = this.defaults;
    const wordReplace = Object.assign({}, base.wordReplace);
    const phraseReplace = Object.assign({}, base.phraseReplace);
    for (const [from, to] of Object.entries(parsePairs(this.settings.voicePrefer))) {
      if (from.trim().split(/\s+/).length === 1) wordReplace[from] = to;
      else phraseReplace[from] = to;
    }
    return Object.assign({}, base, {
      wordReplace,
      phraseReplace,
      allow: dedupe([...(base.allow || []), ...parseLines(this.settings.voiceKeep)]),
      blockWords: dedupe([...(base.blockWords || []), ...parseLines(this.settings.voiceAvoid)]),
    });
  }

  // chop runs the engine over text and returns the cleaned output with before and after
  // scores, or an error. A boot that failed at load time is retried here, so a transient
  // failure does not brick the plugin until restart.
  async chop(text) {
    if (!this.engineReady) {
      try {
        await this.bootEngine();
      } catch (err) {
        return { error: "engine not loaded: " + ((err && err.message) || err) };
      }
    }
    const req = JSON.stringify({ text, profile: this.voiceProfile(), presets: this.presets() });
    const res = JSON.parse(globalThis.slopChop(req));
    if (res.error) return { error: res.error };
    return {
      output: res.output,
      before: res.score ? res.score.value : null,
      after: res.scoreAfter ? res.scoreAfter.value : null,
    };
  }

  // chopActiveFile chops the whole active note in place.
  async chopActiveFile() {
    const view = this.app.workspace.getActiveViewOfType(MarkdownView);
    if (!view) {
      new Notice("slop-chop: open a note first");
      return;
    }
    const editor = view.editor;
    const text = editor.getValue();
    if (!text.trim()) return;
    const res = await this.chop(text);
    if (res.error) {
      new Notice("slop-chop: " + res.error);
      return;
    }
    if (editor.getValue() !== text) {
      new Notice("slop-chop: the note changed while chopping; nothing applied");
      return;
    }
    replaceAll(editor, res.output);
    new Notice("Chopped · slop " + res.before + " → " + res.after);
  }

  // chopSelection chops the selection in place, or the whole note when nothing is selected.
  async chopSelection(editor) {
    const sel = editor.getSelection();
    if (!sel) {
      await this.chopActiveFile();
      return;
    }
    if (!sel.trim()) return;
    // Pin the exact range that was chopped. Guarding only on the whole-note text would let a
    // selection that moved during the async chop, to identical text elsewhere, take the swap
    // in the wrong place; replacing the pinned range instead of the live selection avoids it.
    const from = editor.getCursor("from");
    const to = editor.getCursor("to");
    const snapshot = editor.getValue();
    const res = await this.chop(sel);
    if (res.error) {
      new Notice("slop-chop: " + res.error);
      return;
    }
    if (editor.getValue() !== snapshot) {
      new Notice("slop-chop: the note changed while chopping; nothing applied");
      return;
    }
    editor.replaceRange(res.output, from, to);
    new Notice("Chopped · slop " + res.before + " → " + res.after);
  }

  // saveSettings persists the settings.
  async saveSettings() {
    await this.saveData(this.settings);
  }
}

// SlopChopSettingTab is the settings pane for the preset and the voice.
class SlopChopSettingTab extends PluginSettingTab {
  // constructor keeps a handle to the plugin so controls can save.
  constructor(app, plugin) {
    super(app, plugin);
    this.plugin = plugin;
  }

  // display builds the settings controls.
  display() {
    const { containerEl } = this;
    containerEl.empty();

    new Setting(containerEl)
      .setName("Preset")
      .setDesc("Which built-in preset to apply. cleaver is the aggressive one.")
      .addText((t) =>
        t.setValue(this.plugin.settings.preset).onChange(async (v) => {
          this.plugin.settings.preset = v.trim();
          await this.plugin.saveSettings();
        }),
      );

    const voiceField = (name, desc, key) =>
      new Setting(containerEl)
        .setName(name)
        .setDesc(desc)
        .addTextArea((t) =>
          t.setValue(this.plugin.settings[key]).onChange(async (v) => {
            this.plugin.settings[key] = v;
            await this.plugin.saveSettings();
          }),
        );

    voiceField("Keep", "One per line. Words and phrases to never flag or cut.", "voiceKeep");
    voiceField("Prefer", "One per line, from => to. Your swap wins. An empty to drops the word.", "voicePrefer");
    voiceField("Avoid", "One per line. Your own words to flag wherever they appear.", "voiceAvoid");
  }
}

module.exports = SlopChopPlugin;
