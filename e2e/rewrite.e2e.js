// Drives the model-rewrite connectors, the streamed reply, the Restore button, and
// the meaning check with both provider endpoints mocked at the network layer, so the
// full click-to-verdict flow runs without keys or a live model.
"use strict";

const { chromium } = require("playwright");

const { BASE, log, launch, waitForApp } = require("./helpers");

async function main() {
  const browser = await launch(chromium);
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });

  // anthropicSSE builds a streamed Messages reply, one text_delta per chunk, the way
  // the API answers when the page asks for stream: true.
  const anthropicSSE = (chunks) =>
    [
      'data: {"type":"message_start"}',
      ...chunks.map(
        (c) => "data: " + JSON.stringify({ type: "content_block_delta", delta: { type: "text_delta", text: c } }),
      ),
      'data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}',
      'data: {"type":"message_stop"}',
      "",
    ].join("\n\n");

  // openaiSSE builds a streamed chat completions reply ending in [DONE].
  const openaiSSE = (chunks) =>
    [
      ...chunks.map((c) => "data: " + JSON.stringify({ choices: [{ delta: { content: c } }] })),
      "data: [DONE]",
      "",
    ].join("\n\n");

  let anthropicReq = null;
  let judgeReq = null;
  await page.route("https://api.anthropic.com/v1/messages", async (route) => {
    const body = JSON.parse(route.request().postData());
    const isJudge = body.messages[0].content.startsWith("ORIGINAL:");
    if (isJudge) judgeReq = { body };
    else anthropicReq = { headers: route.request().headers(), body };
    if (body.stream) {
      await route.fulfill({
        status: 200,
        contentType: "text/event-stream",
        body: anthropicSSE(["The model rewrote ", "this cleanly. ", "Short. Human."]),
      });
      return;
    }
    const text = isJudge
      ? 'Here you go:\n{"faithful": true, "issues": []}'
      : "The model rewrote this cleanly. Short. Human.";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ stop_reason: "end_turn", content: [{ type: "text", text }] }),
    });
  });

  let openaiReq = null;
  await page.route("http://localhost:11434/v1/chat/completions", async (route) => {
    const body = JSON.parse(route.request().postData());
    const isJudge = body.messages[1].content.startsWith("ORIGINAL:");
    if (!isJudge) openaiReq = { headers: route.request().headers(), body };
    if (body.stream) {
      await route.fulfill({
        status: 200,
        contentType: "text/event-stream",
        body: openaiSSE(["Local model rewrite. ", "Also clean."]),
      });
      return;
    }
    const content = isJudge ? '{"faithful": true, "issues": []}' : "Local model rewrite. Also clean.";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ choices: [{ message: { content } }] }),
    });
  });

  await page.goto(BASE, { waitUntil: "load" });
  await waitForApp(page);

  // Step 1: no provider, no Rewrite button.
  if (!(await page.isHidden("#sc-rewrite"))) throw new Error("rewrite button visible with provider off");
  log("rewrite hidden with provider off: ok");

  // Step 2: configure Anthropic, button appears once a key exists.
  await page.click("#sc-settings-btn");
  await page.selectOption("#sc-rw-provider", "anthropic");
  const hiddenNoKey = await page.isHidden("#sc-rewrite");
  await page.fill("#sc-rw-key", "sk-ant-test-not-real");
  await page.fill("#sc-rw-tone", "dry and direct");
  await page.waitForTimeout(200);
  if (!hiddenNoKey || !(await page.isVisible("#sc-rewrite"))) throw new Error("button gating wrong");
  log("button gated on key: ok");

  // Step 3: click Rewrite, the mocked model answers, the meaning check passes.
  await page.click("#sc-drawer-close");
  await page.click("#sc-rewrite");
  await page.waitForFunction(
    () => document.getElementById("sc-out").value.includes("model rewrote"),
    { timeout: 8000 },
  );
  await page.waitForFunction(
    () => document.getElementById("sc-status").textContent.includes("Meaning check passed"),
    { timeout: 8000 },
  );
  if (anthropicReq.headers["anthropic-dangerous-direct-browser-access"] !== "true") {
    throw new Error("CORS opt-in header missing");
  }
  if (anthropicReq.body.stream !== true) throw new Error("rewrite did not ask for a stream");
  if (judgeReq.body.stream) throw new Error("judge asked for a stream");
  if (!anthropicReq.body.system.includes("dry and direct")) throw new Error("tone missing from prompt");
  if (anthropicReq.body.messages[0].content.includes("—")) throw new Error("sent unchopped text");
  if (!judgeReq.body.messages[0].content.includes("REWRITE:")) throw new Error("judge missing texts");
  if (!judgeReq.body.system.includes('"faithful"')) throw new Error("judge prompt wrong");
  const outMarkCount = await page.locator("#sc-out-marks mark").count();
  log("anthropic rewrite streamed + meaning check: ok | model:", anthropicReq.body.model, "| out-marks:", outMarkCount);

  // Step 4: OpenAI-compatible provider hits the mocked local endpoint.
  await page.click("#sc-settings-btn");
  await page.selectOption("#sc-rw-provider", "openai");
  await page.fill("#sc-rw-url", "http://localhost:11434");
  await page.fill("#sc-rw-omodel", "llama3.3");
  await page.waitForTimeout(200);
  await page.click("#sc-drawer-close");
  await page.fill("#sc-in", "In summary, we leverage robust synergy.");
  await page.waitForTimeout(400);
  const rulesOut = await page.inputValue("#sc-out");
  if (!(await page.isHidden("#sc-restore"))) throw new Error("restore visible before any rewrite");
  await page.click("#sc-rewrite");
  await page.waitForFunction(
    () => document.getElementById("sc-status").textContent.includes("Meaning check passed"),
    { timeout: 8000 },
  );
  const roles = openaiReq.body.messages.map((m) => m.role).join(",");
  if (roles !== "system,user") throw new Error("openai roles wrong: " + roles);
  if ("authorization" in openaiReq.headers) throw new Error("auth header sent without key");
  if (openaiReq.body.stream !== true) throw new Error("openai rewrite did not ask for a stream");
  log("openai rewrite streamed + meaning check: ok | model:", openaiReq.body.model);

  // Step 4b: Restore puts the rules output back and retires itself.
  if (!(await page.isVisible("#sc-restore"))) throw new Error("restore missing after rewrite");
  await page.click("#sc-restore");
  const restored = await page.inputValue("#sc-out");
  if (restored !== rulesOut) {
    throw new Error("restore mismatch: " + JSON.stringify({ restored, rulesOut }));
  }
  if (!(await page.isHidden("#sc-restore"))) throw new Error("restore still visible after restoring");
  log("restore returned rules output: ok");

  // Step 5 (probe): provider error surfaces inline and the app recovers.
  await page.unroute("http://localhost:11434/v1/chat/completions");
  await page.route("http://localhost:11434/v1/chat/completions", (route) =>
    route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({ error: { message: "invalid api key" } }),
    }),
  );
  await page.click("#sc-rewrite");
  await page.waitForFunction(
    () => document.getElementById("sc-status").textContent.startsWith("Rewrite failed"),
    { timeout: 8000 },
  );
  if (!(await page.isEnabled("#sc-rewrite"))) throw new Error("button stuck disabled");
  if ((await page.inputValue("#sc-out")) !== rulesOut) {
    throw new Error("failed rewrite did not roll the pane back");
  }
  log("provider error surfaced and rolled back: ok");

  // Step 6 (probe): rewrite settings persist, keys included, across a reload.
  await page.reload({ waitUntil: "load" });
  await waitForApp(page);
  const persisted = await page.evaluate(() => ({
    provider: document.getElementById("sc-rw-provider").value,
    url: document.getElementById("sc-rw-url").value,
    btnVisible: !document.getElementById("sc-rewrite").hidden,
  }));
  if (persisted.provider !== "openai" || !persisted.btnVisible) {
    throw new Error("rewrite settings did not persist: " + JSON.stringify(persisted));
  }
  log("rewrite settings persist: ok");

  await browser.close();
  console.log("REWRITE SUITE PASS");
}

main().catch((err) => {
  console.error("REWRITE SUITE FAIL:", err.message);
  process.exit(1);
});
