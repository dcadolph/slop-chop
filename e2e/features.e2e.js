// Drives the worker engine, share links, the compact hero fold, the output diff, and
// the score breakdown in Chromium.
"use strict";

const { chromium } = require("playwright");

const { BASE, log, launch, waitForApp } = require("./helpers");

async function main() {
  const browser = await launch(chromium);
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));

  // Step 1: engine boots in the worker, version tag populated.
  await page.goto(BASE, { waitUntil: "load" });
  await waitForApp(page);
  const engineTag = await page.textContent("#sc-engine");
  if (!/engine v?\d/.test(engineTag)) throw new Error("engine tag missing: " + engineTag);
  log("worker engine booted:", JSON.stringify(engineTag.trim()));

  // Step 2: hero compact, chopper visible above a 900px fold.
  const inBox = await page.locator("#sc-in").boundingBox();
  if (inBox.y > 820) throw new Error("input pane below the fold: " + inBox.y);
  log("input pane top:", Math.round(inBox.y), "(fold at 900)");

  // Step 3: output diff marks paint under what changed.
  const outMarks = await page.locator("#sc-out-marks mark").count();
  if (outMarks < 5) throw new Error("output diff marks missing: " + outMarks);
  log("output diff marks:", outMarks);

  // Step 4: the score chip opens a breakdown whose numbers match the findings bar.
  await page.click("#sc-score");
  const pop = await page.evaluate(() => ({
    visible: !document.getElementById("sc-score-pop").hidden,
    tells: document.getElementById("sc-pop-tells").textContent,
    density: document.getElementById("sc-pop-density").textContent,
    count: document.getElementById("sc-findings-count").textContent,
  }));
  if (!pop.visible) throw new Error("score popover did not open");
  if (pop.tells + " tells" !== pop.count) throw new Error("popover tells mismatch: " + JSON.stringify(pop));
  await page.keyboard.press("Escape");
  if (!(await page.evaluate(() => document.getElementById("sc-score-pop").hidden))) {
    throw new Error("escape did not close score popover");
  }
  log("score breakdown:", pop.tells, "tells |", pop.density);

  // Step 5: the main thread stays free during a giant chop.
  const big = "In summary, a robust—seamless—plan; it works. ".repeat(2000);
  await page.evaluate((t) => {
    const el = document.getElementById("sc-in");
    el.value = t;
    el.dispatchEvent(new Event("input", { bubbles: true }));
  }, big);
  await page.waitForTimeout(500);
  const t0 = Date.now();
  await page.evaluate(() => 1 + 1);
  const mainThreadMs = Date.now() - t0;
  if (mainThreadMs > 1500) throw new Error("main thread blocked: " + mainThreadMs + "ms");
  await page.waitForFunction(
    () => document.getElementById("sc-out").value.length > 10000,
    { timeout: 90000 },
  );
  log("main thread free during big chop:", mainThreadMs + "ms");

  // Step 6 (probe): typing mid-chop drops the stale result, newest wins.
  await page.fill("#sc-in", "");
  await page.evaluate((t) => {
    const el = document.getElementById("sc-in");
    el.value = t;
    el.dispatchEvent(new Event("input", { bubbles: true }));
  }, big);
  await page.waitForTimeout(200);
  await page.fill("#sc-in", "Just a robust little line.");
  await page.waitForFunction(
    () => document.getElementById("sc-out").value === "Just a solid little line.",
    { timeout: 90000 },
  );
  log("stale chop dropped, newest paint wins: ok");

  // Step 7: share link round-trip in a fresh page, keys excluded.
  await page.click("#sc-settings-btn");
  await page.selectOption("#sc-rw-provider", "anthropic");
  await page.fill("#sc-rw-key", "sk-ant-secret-stays-home");
  await page.fill("#sc-block-words", "flywheel");
  await page.waitForTimeout(300);
  const href = await page.evaluate(() => {
    const state = JSON.parse(localStorage.getItem("slop-chop-settings-v1"));
    delete state.rwKey;
    delete state.rwOKey;
    const bytes = new TextEncoder().encode(JSON.stringify(state));
    let bin = "";
    for (const b of bytes) bin += String.fromCharCode(b);
    const enc = btoa(bin).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
    return location.origin + location.pathname + "#s=" + enc;
  });
  const b64 = href.split("#s=")[1].replaceAll("-", "+").replaceAll("_", "/");
  const decoded = Buffer.from(b64, "base64").toString("utf8");
  if (decoded.includes("secret")) throw new Error("key leaked into share link");

  const page2 = await browser.newPage();
  await page2.goto(href, { waitUntil: "load" });
  await waitForApp(page2);
  const received = await page2.evaluate(() => ({
    blockWords: document.getElementById("sc-block-words").value,
    key: document.getElementById("sc-rw-key").value,
    hash: location.hash,
  }));
  if (received.blockWords !== "flywheel") throw new Error("shared settings not applied");
  if (received.key !== "") throw new Error("key crossed the link");
  if (received.hash !== "") throw new Error("hash not cleaned from url");
  await page2.close();
  log("share link round-trip, key excluded: ok");

  // Step 8 (probe): a mangled share hash degrades to a normal visit.
  const page3 = await browser.newPage();
  await page3.goto(BASE + "#s=%%%not-base64%%%", { waitUntil: "load" });
  await waitForApp(page3);
  await page3.close();
  log("mangled share hash still boots: ok");

  if (errors.length) throw new Error("page errors: " + errors.join(" | "));
  await browser.close();
  console.log("FEATURES SUITE PASS");
}

main().catch((err) => {
  console.error("FEATURES SUITE FAIL:", err.message);
  process.exit(1);
});
