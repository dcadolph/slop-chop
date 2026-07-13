/* The slop-chop rules engine for Node. Loads the bundled WebAssembly build once and exposes
   chop, score, and the built-in profile and preset names. Everything runs in-process: no
   network, no model, no data leaves the machine. */
"use strict";

const fs = require("fs");
const path = require("path");

let ready = null;
let defaults = null;

// init loads the wasm engine once. Every exported call awaits it, so calling init directly
// is optional and only useful to front-load the startup cost.
function init() {
  if (ready) return ready;
  ready = (async () => {
    const dir = path.join(__dirname, "engine");
    // wasm_exec.js is Go's runtime shim; it defines Go on the global object.
    (0, eval)(fs.readFileSync(path.join(dir, "wasm_exec.js"), "utf8"));
    const go = new globalThis.Go();
    const bytes = fs.readFileSync(path.join(dir, "slop-chop.wasm"));
    const result = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(result.instance);
    // Give the Go runtime a tick to register its globals.
    await new Promise((r) => setTimeout(r, 0));
    defaults = JSON.parse(globalThis.slopDefaults());
  })();
  return ready;
}

// parseVoice folds a voice of keep, prefer, and avoid lists into a profile: keep into allow,
// avoid into blockWords, and each prefer entry into a word or phrase swap. The voice wins
// over the base profile, and a kept term survives every rule.
function parseVoice(base, voice) {
  if (!voice) return base;
  const wordReplace = { ...base.wordReplace };
  const phraseReplace = { ...base.phraseReplace };
  for (const [from, to] of Object.entries(voice.prefer || {})) {
    if (String(from).trim().split(/\s+/).length === 1) wordReplace[from] = to;
    else phraseReplace[from] = to;
  }
  return {
    ...base,
    wordReplace,
    phraseReplace,
    allow: [...new Set([...(base.allow || []), ...(voice.keep || [])])],
    blockWords: [...new Set([...(base.blockWords || []), ...(voice.avoid || [])])],
  };
}

// chop cleans text and returns {output, findings, score, scoreAfter}. Options: presets is a
// list of built-in preset names (default ["cleaver"]), profile overrides the built-in default
// profile, and voice is {keep, prefer, avoid} folded on top.
async function chop(text, opts = {}) {
  await init();
  const base = opts.profile || defaults;
  const req = JSON.stringify({
    text,
    profile: parseVoice(base, opts.voice),
    presets: opts.presets || ["cleaver"],
  });
  const res = JSON.parse(globalThis.slopChop(req));
  if (res.error) throw new Error(res.error);
  return res;
}

// score rates text from 0 for clean to 100 for heavy slop, using the same options as chop.
async function score(text, opts = {}) {
  const res = await chop(text, opts);
  return res.score.value;
}

// defaultProfile returns a copy of the built-in default profile.
async function defaultProfile() {
  await init();
  return JSON.parse(JSON.stringify(defaults));
}

// presetNames lists the built-in preset names.
async function presetNames() {
  await init();
  return JSON.parse(globalThis.slopPresets());
}

// version returns the engine build version.
async function version() {
  await init();
  return globalThis.slopVersion();
}

module.exports = { init, chop, score, defaultProfile, presetNames, version };
