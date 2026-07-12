// Drives the core chop flows in Chromium: boot, paste, settings, presets, errors,
// persistence, reset, a giant paste, and cleanup.
"use strict";

const { chromium } = require("playwright");

const { BASE, log, launch, waitForApp } = require("./helpers");

async function main() {
  const browser = await launch(chromium);
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  const consoleErrors = [];
  page.on("console", (m) => {
    if (m.type() === "error") consoleErrors.push(m.text());
  });
  page.on("pageerror", (e) => consoleErrors.push("pageerror: " + e.message));

  // Step 1: load the landing page, engine boots, sample chops on its own.
  await page.goto(BASE, { waitUntil: "load" });
  await waitForApp(page);
  const first = await page.evaluate(() => ({
    out: document.getElementById("sc-out").value,
    score: document.getElementById("sc-score").textContent,
    count: document.getElementById("sc-findings-count").textContent,
  }));
  log("boot:", JSON.stringify(first.score), "|", first.count);
  if (first.out.includes("—") || /in summary/i.test(first.out)) throw new Error("sample not chopped");

  // Step 2: paste fresh slop like a user would.
  await page.fill("#sc-in", "");
  await page.fill(
    "#sc-in",
    "Needless to say, we leverage cutting-edge synergy—seamlessly. It's not just fast, it's transformative; teams delve deeper.",
  );
  await page.waitForTimeout(400);
  const pasted = await page.inputValue("#sc-out");
  log("paste out:", JSON.stringify(pasted));
  if (pasted.includes("—") || pasted.includes(";")) throw new Error("paste not chopped");

  // Step 3: the settings drawer opens with presets rendered from the engine.
  await page.click("#sc-settings-btn");
  if (!(await page.isVisible("#sc-drawer"))) throw new Error("drawer did not open");
  const presetCount = await page.locator(".sc-preset").count();
  log("preset checkboxes:", presetCount);
  if (presetCount !== 5) throw new Error("expected five presets, got " + presetCount);

  // Step 4: dialect toggle changes behavior end to end.
  await page.fill("#sc-in", "We optimise the colour and behaviour of the system.");
  await page.check('input[name="sc-dialect"][value="american"]');
  await page.waitForTimeout(400);
  const dialect = await page.inputValue("#sc-out");
  log("dialect out:", JSON.stringify(dialect));
  if (!/optimize/.test(dialect) || !/color/.test(dialect) || !/behavior/.test(dialect)) {
    throw new Error("american dialect did not rewrite");
  }

  // Step 5: semicolon toggle off keeps the semicolon, back on splits it.
  await page.fill("#sc-in", "It works; it ships.");
  await page.uncheck("#sc-split-semicolons");
  await page.waitForTimeout(400);
  if (!(await page.inputValue("#sc-out")).includes(";")) throw new Error("semicolon should remain");
  await page.check("#sc-split-semicolons");
  await page.waitForTimeout(400);
  if ((await page.inputValue("#sc-out")).includes(";")) throw new Error("semicolon should split");
  log("semicolon toggle both ways: ok");

  // Step 6: a custom block word shows up as a finding.
  await page.fill("#sc-in", "Our workflow is a workflow of workflows.");
  await page.fill("#sc-block-words", "workflow");
  await page.waitForTimeout(400);
  await page.click("#sc-findings summary");
  const rules = await page.evaluate(() =>
    [...document.querySelectorAll("#sc-findings-list .sc-rule")].map((el) => el.textContent),
  );
  if (!rules.includes("word:workflow")) throw new Error("custom block word not flagged");
  log("custom block word flagged: ok");

  // Step 7 (probe): garbage regex surfaces an inline error, the app stays alive.
  await page.fill("#sc-regex-swaps", "(unclosed => x");
  await page.waitForTimeout(400);
  const err = await page.evaluate(() => ({
    text: document.getElementById("sc-status").textContent,
    shown: !document.getElementById("sc-status").hidden,
  }));
  if (!err.shown || !/regex/.test(err.text)) throw new Error("bad regex not surfaced");
  await page.fill("#sc-regex-swaps", "");
  await page.waitForTimeout(400);
  log("bad regex surfaced and recovered: ok");

  // Step 8 (probe): settings persist across a reload.
  await page.check('.sc-preset[value="corporate"]');
  await page.waitForTimeout(300);
  await page.reload({ waitUntil: "load" });
  await waitForApp(page);
  const persisted = await page.evaluate(() => ({
    blockWords: document.getElementById("sc-block-words").value,
    corporate: document.querySelector('.sc-preset[value="corporate"]')?.checked,
  }));
  if (persisted.blockWords !== "workflow" || !persisted.corporate) {
    throw new Error("settings did not persist: " + JSON.stringify(persisted));
  }
  log("settings persist across reload: ok");

  // Step 9 (probe): reset restores defaults.
  await page.click("#sc-settings-btn");
  await page.click("#sc-reset");
  await page.waitForTimeout(300);
  const afterReset = await page.evaluate(() => ({
    blockWords: document.getElementById("sc-block-words").value,
    corporate: document.querySelector('.sc-preset[value="corporate"]')?.checked,
    cleaver: document.querySelector('.sc-preset[value="cleaver"]')?.checked,
  }));
  if (afterReset.blockWords !== "" || afterReset.corporate || !afterReset.cleaver) {
    throw new Error("reset did not restore defaults: " + JSON.stringify(afterReset));
  }
  log("reset restores defaults: ok");

  // Step 10 (probe): a giant paste chops clean.
  const big = "In summary, a robust—seamless—plan; it works. ".repeat(2000);
  await page.evaluate((t) => {
    const el = document.getElementById("sc-in");
    el.value = t;
    el.dispatchEvent(new Event("input", { bubbles: true }));
  }, big);
  await page.waitForFunction(
    () => document.getElementById("sc-out").value.length > 10000,
    { timeout: 90000 },
  );
  const bigOut = await page.evaluate(() => document.getElementById("sc-out").value);
  if (bigOut.includes("—") || bigOut.includes(";")) throw new Error("big paste not chopped clean");
  log("big paste chopped clean: ok");

  // Step 11 (probe): Escape closes the drawer, empty input clears the panes.
  await page.click("#sc-settings-btn");
  await page.keyboard.press("Escape");
  if (!(await page.isHidden("#sc-drawer"))) throw new Error("escape did not close drawer");
  await page.fill("#sc-in", "");
  await page.waitForTimeout(300);
  const cleared = await page.evaluate(() => ({
    out: document.getElementById("sc-out").value,
    scoreHidden: document.getElementById("sc-score").hidden,
  }));
  if (cleared.out !== "" || !cleared.scoreHidden) throw new Error("clear did not reset panes");
  log("escape and clear: ok");

  if (consoleErrors.length) throw new Error("console errors: " + consoleErrors.join(" | "));
  await browser.close();
  console.log("BASE SUITE PASS");
}

main().catch((err) => {
  console.error("BASE SUITE FAIL:", err.message);
  process.exit(1);
});
