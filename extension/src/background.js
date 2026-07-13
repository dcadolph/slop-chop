/* Service worker. The hotkey fires here, so it asks the focused tab's content script to chop
   what has focus. The engine runs in an offscreen document, off the worker, which Chrome is
   free to stop between events. Content, popup, and options pages send engine calls here and
   this relays them to the offscreen document. */
"use strict";

// OFFSCREEN_URL is the hidden page that loads and runs the wasm engine.
const OFFSCREEN_URL = "src/offscreen.html";

// creating holds the in-flight offscreen creation, so concurrent callers share one create
// instead of racing chrome.offscreen.createDocument, which allows a single document only.
let creating = null;

// ensureOffscreen creates the offscreen document once, so the engine loads a single time.
// A failed creation clears the cached attempt, so the next call retries instead of failing
// forever on a stale rejection.
async function ensureOffscreen() {
  const existing = await chrome.runtime.getContexts({
    contextTypes: ["OFFSCREEN_DOCUMENT"],
  });
  if (existing.length > 0) return;
  if (!creating) {
    creating = chrome.offscreen
      .createDocument({
        url: OFFSCREEN_URL,
        reasons: ["BLOBS"],
        justification: "Run the slop-chop WebAssembly engine locally.",
      })
      .catch((err) => {
        creating = null;
        throw err;
      });
  }
  await creating;
}

// callOffscreen makes sure the engine page exists, then forwards a message to it.
async function callOffscreen(message) {
  await ensureOffscreen();
  return chrome.runtime.sendMessage({ ...message, target: "offscreen" });
}

// chopWithSettings reads the saved voice and presets here, where chrome.storage is available,
// and hands them to the offscreen engine, which cannot read storage itself.
async function chopWithSettings(text) {
  const settings = await chrome.storage.local.get({
    voice: { keep: [], prefer: {}, avoid: [] },
    presets: ["cleaver"],
  });
  return callOffscreen({ type: "chop", text, settings });
}

// The hotkey lands here. Tell the active tab's content script to chop what has focus.
chrome.commands.onCommand.addListener(async (command) => {
  if (command !== "chop-field") return;
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (tab && tab.id != null) {
    chrome.tabs.sendMessage(tab.id, { type: "do-chop" }).catch(() => {});
  }
});

// Content, popup, and options ask for engine work. A chop needs the saved settings; a
// presets query does not. Relay to the offscreen document either way.
chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || msg.target === "offscreen") return undefined;
  if (msg.type === "chop") {
    chopWithSettings(msg.text)
      .then((res) => sendResponse(res))
      .catch((err) => sendResponse({ error: String((err && err.message) || err) }));
    return true;
  }
  if (msg.type === "presets") {
    callOffscreen(msg)
      .then((res) => sendResponse(res))
      .catch((err) => sendResponse({ error: String((err && err.message) || err) }));
    return true;
  }
  return undefined;
});
