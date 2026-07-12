/* Service worker. The hotkey fires here, so it asks the focused tab's content script to
   chop what has focus. The engine itself runs in an offscreen document, off the worker,
   which Chrome is free to stop between events. */
"use strict";

// OFFSCREEN_URL is the hidden page that loads and runs the wasm engine.
const OFFSCREEN_URL = "src/offscreen.html";

// ensureOffscreen creates the offscreen document once, so the engine loads a single time.
async function ensureOffscreen() {
  const existing = await chrome.runtime.getContexts({
    contextTypes: ["OFFSCREEN_DOCUMENT"],
  });
  if (existing.length > 0) return;
  await chrome.offscreen.createDocument({
    url: OFFSCREEN_URL,
    reasons: ["BLOBS"],
    justification: "Run the slop-chop WebAssembly engine locally.",
  });
}

// chopText hands text to the offscreen engine and resolves with its result.
async function chopText(text) {
  await ensureOffscreen();
  return chrome.runtime.sendMessage({ target: "offscreen", type: "chop", text });
}

// The hotkey lands here. Tell the active tab's content script to chop what has focus.
chrome.commands.onCommand.addListener(async (command) => {
  if (command !== "chop-field") return;
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (tab && tab.id != null) {
    chrome.tabs.sendMessage(tab.id, { type: "do-chop" }).catch(() => {});
  }
});

// The content script asks for a chop. Run it through the offscreen engine and answer.
chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (msg && msg.type === "chop" && msg.target !== "offscreen") {
    chopText(msg.text)
      .then((res) => sendResponse(res))
      .catch((err) => sendResponse({ error: String((err && err.message) || err) }));
    return true;
  }
  return undefined;
});
