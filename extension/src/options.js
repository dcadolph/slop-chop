/* The options page edits the voice (keep, prefer, avoid) and the active presets, saving them
   to chrome.storage.local where the offscreen engine reads them for every chop. */
"use strict";

const byId = (id) => document.getElementById(id);

// parseLines splits a textarea into trimmed, non-empty lines.
function parseLines(text) {
  return text
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);
}

// parsePairs reads "from => to" lines into a map, with an empty to meaning drop.
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

// pairLines renders a prefer map back into editable "from => to" lines.
function pairLines(map) {
  return Object.entries(map || {})
    .map(([k, v]) => k + " => " + v)
    .join("\n");
}

// renderPresets draws a checkbox per built-in preset, asking the engine for the names.
async function renderPresets(selected) {
  let names = ["cleaver"];
  try {
    const res = await chrome.runtime.sendMessage({ type: "presets" });
    if (res && res.presets) names = res.presets;
  } catch {
    /* Fall back to the default name. */
  }
  const box = byId("presets");
  box.textContent = "";
  for (const name of names) {
    const label = document.createElement("label");
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.value = name;
    cb.checked = selected.includes(name);
    label.appendChild(cb);
    label.appendChild(document.createTextNode(" " + name));
    box.appendChild(label);
  }
}

// selectedPresets returns the checked preset names.
function selectedPresets() {
  return [...document.querySelectorAll("#presets input:checked")].map((el) => el.value);
}

// load fills the form from storage.
async function load() {
  const s = await chrome.storage.local.get({
    voice: { keep: [], prefer: {}, avoid: [] },
    presets: ["cleaver"],
  });
  byId("keep").value = (s.voice.keep || []).join("\n");
  byId("prefer").value = pairLines(s.voice.prefer);
  byId("avoid").value = (s.voice.avoid || []).join("\n");
  await renderPresets(s.presets || ["cleaver"]);
}

// save writes the form back to storage.
async function save() {
  const voice = {
    keep: parseLines(byId("keep").value),
    prefer: parsePairs(byId("prefer").value),
    avoid: parseLines(byId("avoid").value),
  };
  await chrome.storage.local.set({ voice, presets: selectedPresets() });
  const status = byId("status");
  status.textContent = "Saved";
  setTimeout(() => (status.textContent = ""), 1500);
}

byId("save").addEventListener("click", save);
byId("import").addEventListener("change", async (e) => {
  const file = e.target.files[0];
  if (!file) return;
  try {
    const v = JSON.parse(await file.text());
    byId("keep").value = (v.keep || []).join("\n");
    byId("prefer").value = pairLines(v.prefer);
    byId("avoid").value = (v.avoid || []).join("\n");
    byId("status").textContent = "Imported. Click Save to apply.";
  } catch {
    byId("status").textContent = "That file is not valid JSON.";
  }
});

load();
