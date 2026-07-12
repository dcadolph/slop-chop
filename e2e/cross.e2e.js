// Core-flow smoke in Firefox and WebKit: worker boot, chop, both mark layers, the
// drawer, share link apply, and a mocked rewrite with its meaning check. Chromium is
// covered by the full suites.
"use strict";

const { firefox, webkit } = require("playwright");

const { BASE, launch, waitForApp } = require("./helpers");

async function smoke(name, browserType) {
  const browser = await launch(browserType);
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));

  await page.route("https://api.anthropic.com/v1/messages", (route) => {
    const isJudge = JSON.parse(route.request().postData()).messages[0].content.startsWith("ORIGINAL:");
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        stop_reason: "end_turn",
        content: [{ type: "text", text: isJudge ? '{"faithful": true, "issues": []}' : "Rewritten fine." }],
      }),
    });
  });

  await page.goto(BASE, { waitUntil: "load" });
  await waitForApp(page);
  const out = await page.inputValue("#sc-out");
  if (out.includes("—")) throw new Error(name + ": em-dash survived");
  const inMarks = await page.locator("#sc-marks mark").count();
  const outMarks = await page.locator("#sc-out-marks mark").count();
  if (inMarks < 5) throw new Error(name + ": input marks missing");
  if (outMarks < 3) throw new Error(name + ": output diff marks missing");

  await page.click("#sc-settings-btn");
  const presets = await page.locator(".sc-preset").count();
  if (presets !== 5) throw new Error(name + ": presets not rendered");
  await page.selectOption("#sc-rw-provider", "anthropic");
  await page.fill("#sc-rw-key", "sk-ant-test");
  await page.waitForTimeout(200);
  await page.click("#sc-drawer-close");
  await page.click("#sc-rewrite");
  await page.waitForFunction(
    () => document.getElementById("sc-status").textContent.includes("Meaning check passed"),
    { timeout: 10000 },
  );

  // Share hash applies in a fresh page.
  const href = await page.evaluate(() => {
    const s = JSON.parse(localStorage.getItem("slop-chop-settings-v1"));
    delete s.rwKey;
    delete s.rwOKey;
    s.blockWords = "crossbrowser";
    const bytes = new TextEncoder().encode(JSON.stringify(s));
    let bin = "";
    for (const c of bytes) bin += String.fromCharCode(c);
    return location.origin + location.pathname + "#s=" + btoa(bin).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
  });
  const p2 = await browser.newPage();
  await p2.goto(href, { waitUntil: "load" });
  await waitForApp(p2);
  if ((await p2.inputValue("#sc-block-words")) !== "crossbrowser") {
    throw new Error(name + ": share link not applied");
  }
  await p2.close();

  if (errors.length) throw new Error(name + " page errors: " + errors.join(" | "));
  await browser.close();
  console.log("[pass]", name, "| in-marks:", inMarks, "| out-marks:", outMarks);
}

(async () => {
  await smoke("firefox", firefox);
  await smoke("webkit", webkit);
  console.log("CROSS SUITE PASS");
})().catch((err) => {
  console.error("CROSS SUITE FAIL:", err.message);
  process.exit(1);
});
