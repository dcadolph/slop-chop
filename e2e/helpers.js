// Shared plumbing for the drive scripts: where the site is served, one launch path,
// and a step logger. BASE comes from E2E_BASE_URL so CI and local runs share code.
"use strict";

const BASE = process.env.E2E_BASE_URL || "http://127.0.0.1:4173/index.html";

// log prints one verified step, so a failure log shows how far the drive got.
const log = (...a) => console.log("[step]", ...a);

// launch starts a headless browser of the given type, defaulting to bundled Chromium.
async function launch(browserType) {
  return browserType.launch({ headless: true });
}

// waitForApp waits until the engine booted and the sample text chopped.
async function waitForApp(page, timeout = 30000) {
  await page.waitForFunction(() => {
    const out = document.getElementById("sc-out");
    return out && out.value.length > 0;
  }, { timeout });
}

module.exports = { BASE, log, launch, waitForApp };
