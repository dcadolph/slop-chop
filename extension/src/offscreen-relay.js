/* Chrome only. Creates and talks to the offscreen document that hosts the wasm engine, so the
   engine runs off the service worker Chrome may stop between events. background.js loads this
   with importScripts on Chrome; Firefox has no offscreen API and never loads it, which keeps
   the Chrome-only offscreen and getContexts APIs out of the Firefox build. */
"use strict";

// OFFSCREEN_URL is the hidden page that loads and runs the wasm engine.
const OFFSCREEN_URL = "src/offscreen.html";

// creating holds the in-flight offscreen creation, so concurrent callers share one create
// instead of racing chrome.offscreen.createDocument, which allows a single document only.
let creating = null;

// ensureOffscreen creates the offscreen document once, so the engine loads a single time. A
// failed creation clears the cached attempt, so the next call retries instead of failing
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

// callOffscreenEngine makes sure the engine page exists, then forwards a message to it.
self.callOffscreenEngine = async function (message) {
  await ensureOffscreen();
  return chrome.runtime.sendMessage({ ...message, target: "offscreen" });
};
