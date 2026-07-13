/* Runs the shared slop-chop engine (engine.js) inside a hidden offscreen document on Chrome.
   The service worker relays messages tagged {target:"offscreen"}; this answers them off the
   worker, which Chrome is free to stop between events. On Firefox there is no offscreen API,
   so the same engine runs in the background page and this file is not loaded. */
"use strict";

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || msg.target !== "offscreen") return undefined;
  if (msg.type === "chop") {
    self.slopEngine
      .boot()
      .then(() => sendResponse(self.slopEngine.chop(msg.text, msg.settings)))
      .catch((err) => sendResponse({ error: String((err && err.message) || err) }));
    return true;
  }
  if (msg.type === "presets") {
    self.slopEngine
      .boot()
      .then(() => sendResponse({ ok: true, presets: self.slopEngine.presets() }))
      .catch((err) => sendResponse({ error: String((err && err.message) || err) }));
    return true;
  }
  return undefined;
});

self.slopEngine.boot();
