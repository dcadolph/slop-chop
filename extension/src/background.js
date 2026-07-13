/* Background script. The hotkey fires here, so it asks the focused tab's content script to
   chop what has focus. Content, popup, and options pages send engine calls here too. On Chrome
   the engine runs in an offscreen document (offscreen-relay.js relays to it) off the service
   worker Chrome may stop between events. Firefox has no offscreen API, so there the engine
   (engine.js) runs in this background page and the calls resolve in place. */
"use strict";

// hasOffscreen is true on Chrome. On Firefox it is false, and the Firefox manifest loads
// wasm_exec.js and engine.js into this background page so the engine runs here directly.
const hasOffscreen = typeof chrome.offscreen !== "undefined";

// On Chrome, load the offscreen relay, which holds the Chrome-only offscreen APIs. Firefox
// never takes this branch, so it neither loads the relay nor references those APIs.
if (hasOffscreen) {
  importScripts("offscreen-relay.js");
}

// runEngine answers an engine request. On Chrome it relays to the offscreen document; on
// Firefox it calls the in-page engine directly, since there is no offscreen to relay to.
async function runEngine(message) {
  if (hasOffscreen) return self.callOffscreenEngine(message);
  await self.slopEngine.boot();
  if (message.type === "chop") return self.slopEngine.chop(message.text, message.settings);
  if (message.type === "presets") return { ok: true, presets: self.slopEngine.presets() };
  return { error: "unknown request" };
}

// chopWithSettings reads the saved voice and presets here, where chrome.storage is available,
// and hands them to the engine, which does not read storage itself.
async function chopWithSettings(text) {
  const settings = await chrome.storage.local.get({
    voice: { keep: [], prefer: {}, avoid: [] },
    presets: ["cleaver"],
  });
  return runEngine({ type: "chop", text, settings });
}

// The hotkey lands here. Tell the active tab's content script to chop what has focus.
chrome.commands.onCommand.addListener(async (command) => {
  if (command !== "chop-field") return;
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (tab && tab.id != null) {
    chrome.tabs.sendMessage(tab.id, { type: "do-chop" }).catch(() => {});
  }
});

// Content, popup, and options ask for engine work. A chop needs the saved settings; a presets
// query does not. Answer both, on the offscreen document (Chrome) or in place (Firefox).
chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || msg.target === "offscreen") return undefined;
  if (msg.type === "chop") {
    chopWithSettings(msg.text)
      .then((res) => sendResponse(res))
      .catch((err) => sendResponse({ error: String((err && err.message) || err) }));
    return true;
  }
  if (msg.type === "presets") {
    runEngine(msg)
      .then((res) => sendResponse(res))
      .catch((err) => sendResponse({ error: String((err && err.message) || err) }));
    return true;
  }
  return undefined;
});
