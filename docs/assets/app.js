/* slop-chop web app: runs the rules engine in the browser via WASM. Text never
   leaves the page. Loaded on every page, boots only where #sc-app exists. */
(() => {
  "use strict";

  const STORE_KEY = "slop-chop-settings-v1";
  const SAMPLE =
    "In summary, this comprehensive guide will delve into our robust, cutting-edge " +
    "platform—a game-changer that seamlessly empowers your workflow. It's not just " +
    "a tool, it's a paradigm shift; teams leverage it to unlock the full potential of " +
    "their content. Needless to say, the results are unparalleled.";

  let engineReady = null;
  let engineVersion = "";
  let defaults = null;
  let presetNames = [];
  let worker = null;
  let callSeq = 0;
  const pendingCalls = new Map();

  /* callEngine sends one call to the worker and resolves with its raw result. */
  function callEngine(fn, arg) {
    return new Promise((resolve, reject) => {
      const id = ++callSeq;
      pendingCalls.set(id, { resolve, reject });
      worker.postMessage({ id, fn, arg });
    });
  }

  /* loadEngine boots the engine in a Web Worker, so a giant paste chops off the main
     thread and typing never freezes. Resolves once the worker reports its globals. */
  function loadEngine() {
    if (engineReady) return engineReady;
    engineReady = new Promise((resolve, reject) => {
      worker = new Worker(new URL("assets/worker.js", document.baseURI));
      worker.onmessage = (e) => {
        const m = e.data;
        if (m.type === "ready") {
          defaults = JSON.parse(m.defaults);
          presetNames = JSON.parse(m.presets);
          engineVersion = m.version;
          resolve();
          return;
        }
        if (m.type === "fail") {
          reject(new Error(m.error));
          return;
        }
        const call = pendingCalls.get(m.id);
        if (!call) return;
        pendingCalls.delete(m.id);
        if (m.error) call.reject(new Error(m.error));
        else call.resolve(m.result);
      };
      worker.onerror = (e) => reject(new Error(e.message || "engine worker failed to start"));
    });
    return engineReady;
  }

  /* emptyProfile is the shape used when the built-in defaults are switched off. */
  function emptyProfile() {
    return {
      charReplace: {},
      phraseReplace: {},
      wordReplace: {},
      regexReplace: {},
      flagPatterns: {},
      blockWords: [],
      allow: [],
    };
  }

  /* parseLines splits a textarea into trimmed, non-empty lines. */
  function parseLines(text) {
    return text
      .split("\n")
      .map((l) => l.trim())
      .filter(Boolean);
  }

  /* parsePairs turns "from => to" lines into a map. A line without the separator
     maps to an empty replacement, which the engine treats as a deletion. */
  function parsePairs(text) {
    const out = {};
    for (const line of parseLines(text)) {
      const i = line.indexOf("=>");
      const from = (i < 0 ? line : line.slice(0, i)).trim();
      const to = i < 0 ? "" : line.slice(i + 2).trim();
      if (from) out[from] = to;
    }
    return out;
  }

  function dedupe(arr) {
    return [...new Set(arr)];
  }

  /* encodeShare packs a settings object into a URL-safe base64 string. */
  function encodeShare(state) {
    const bytes = new TextEncoder().encode(JSON.stringify(state));
    let bin = "";
    for (const b of bytes) bin += String.fromCharCode(b);
    return btoa(bin).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
  }

  /* decodeShare unpacks a shared settings string, or returns null when it does not
     parse, so a mangled link degrades to a normal visit. */
  function decodeShare(encoded) {
    try {
      const bin = atob(encoded.replaceAll("-", "+").replaceAll("_", "/"));
      const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0));
      const state = JSON.parse(new TextDecoder().decode(bytes));
      return state && typeof state === "object" ? state : null;
    } catch {
      return null;
    }
  }

  /* readShareHash returns the settings carried in the page URL, if any. */
  function readShareHash() {
    const m = /[#&]s=([A-Za-z0-9_-]+)/.exec(location.hash);
    return m ? decodeShare(m[1]) : null;
  }

  /* diffOps runs a Myers diff over two token lists. It returns [op, token] pairs where
     0 keeps a token, 1 adds one, and -1 drops one, or null when the lists diverge past
     the cap and a diff would not help anyone. */
  function diffOps(a, b) {
    const N = a.length;
    const M = b.length;
    const MAX = Math.min(N + M, 1500);
    const OFF = MAX;
    const V = new Int32Array(2 * MAX + 2);
    const trace = [];
    let D = 0;
    found: {
      for (; D <= MAX; D++) {
        trace.push(V.slice());
        for (let k = -D; k <= D; k += 2) {
          let x;
          if (k === -D || (k !== D && V[OFF + k - 1] < V[OFF + k + 1])) x = V[OFF + k + 1];
          else x = V[OFF + k - 1] + 1;
          let y = x - k;
          while (x < N && y < M && a[x] === b[y]) {
            x++;
            y++;
          }
          V[OFF + k] = x;
          if (x >= N && y >= M) break found;
        }
      }
      return null;
    }
    const ops = [];
    let x = N;
    let y = M;
    for (let d = D; d > 0; d--) {
      const Vp = trace[d];
      const k = x - y;
      let prevK;
      if (k === -d || (k !== d && Vp[OFF + k - 1] < Vp[OFF + k + 1])) prevK = k + 1;
      else prevK = k - 1;
      const prevX = Vp[OFF + prevK];
      const prevY = prevX - prevK;
      while (x > prevX && y > prevY) {
        x--;
        y--;
        ops.push([0, b[y]]);
      }
      if (x === prevX) {
        y--;
        ops.push([1, b[y]]);
      } else {
        x--;
        ops.push([-1, a[x]]);
      }
    }
    while (x > 0 && y > 0) {
      x--;
      y--;
      ops.push([0, b[y]]);
    }
    while (y > 0) {
      y--;
      ops.push([1, b[y]]);
    }
    while (x > 0) {
      x--;
      ops.push([-1, a[x]]);
    }
    return ops.reverse();
  }

  /* tokens splits text into words and whitespace runs, both kept, so a diff rebuilds
     the text byte for byte. */
  function tokens(text) {
    return text.split(/(\s+)/).filter((t) => t !== "");
  }

  /* hasFiles reports whether a drag carries files, as opposed to dragged text, which
     the textarea already handles on its own. */
  function hasFiles(e) {
    return Boolean(e.dataTransfer && e.dataTransfer.types && e.dataTransfer.types.includes("Files"));
  }

  /* A file dropped outside the pane would replace the page with the file. Swallow the
     default anywhere the chopper is present, so a near miss costs nothing. */
  document.addEventListener("dragover", (e) => {
    if (hasFiles(e) && document.getElementById("sc-app")) e.preventDefault();
  });
  document.addEventListener("drop", (e) => {
    if (hasFiles(e) && document.getElementById("sc-app")) e.preventDefault();
  });

  /* App wires one #sc-app element to the engine. */
  function boot(root) {
    const $ = (id) => root.querySelector("#" + id);
    const input = $("sc-in");
    const marks = $("sc-marks");
    const output = $("sc-out");
    const outMarks = $("sc-out-marks");
    const score = $("sc-score");
    const status = $("sc-status");
    const copyBtn = $("sc-copy");
    const downloadBtn = $("sc-download");
    const clearBtn = $("sc-clear");
    const findingsBox = $("sc-findings");
    const findingsCount = $("sc-findings-count");
    const findingsList = $("sc-findings-list");
    const drawer = $("sc-drawer");
    const burger = $("sc-settings-btn");
    const closeBtn = $("sc-drawer-close");
    const engineTag = $("sc-engine");

    const controls = {
      useDefaults: $("sc-use-defaults"),
      splitSemicolons: $("sc-split-semicolons"),
      collapseSpaces: $("sc-collapse-spaces"),
      blockWords: $("sc-block-words"),
      wordSwaps: $("sc-word-swaps"),
      phraseSwaps: $("sc-phrase-swaps"),
      charSwaps: $("sc-char-swaps"),
      regexSwaps: $("sc-regex-swaps"),
      flagPatterns: $("sc-flag-patterns"),
      allow: $("sc-allow"),
      rwProvider: $("sc-rw-provider"),
      rwKey: $("sc-rw-key"),
      rwModel: $("sc-rw-model"),
      rwURL: $("sc-rw-url"),
      rwOModel: $("sc-rw-omodel"),
      rwOKey: $("sc-rw-okey"),
      rwTone: $("sc-rw-tone"),
      rwVerify: $("sc-rw-verify"),
    };
    const rewriteBtn = $("sc-rewrite");

    function dialectValue() {
      const checked = root.querySelector('input[name="sc-dialect"]:checked');
      return checked ? checked.value : "";
    }

    function selectedPresets() {
      return [...root.querySelectorAll(".sc-preset:checked")].map((el) => el.value);
    }

    /* settingsState snapshots the raw control values for storage. */
    function settingsState() {
      return {
        useDefaults: controls.useDefaults.checked,
        splitSemicolons: controls.splitSemicolons.checked,
        collapseSpaces: controls.collapseSpaces.checked,
        dialect: dialectValue(),
        presets: selectedPresets(),
        blockWords: controls.blockWords.value,
        wordSwaps: controls.wordSwaps.value,
        phraseSwaps: controls.phraseSwaps.value,
        charSwaps: controls.charSwaps.value,
        regexSwaps: controls.regexSwaps.value,
        flagPatterns: controls.flagPatterns.value,
        allow: controls.allow.value,
        rwProvider: controls.rwProvider.value,
        rwKey: controls.rwKey.value,
        rwModel: controls.rwModel.value,
        rwURL: controls.rwURL.value,
        rwOModel: controls.rwOModel.value,
        rwOKey: controls.rwOKey.value,
        rwTone: controls.rwTone.value,
        rwVerify: controls.rwVerify.checked,
      };
    }

    function saveSettings() {
      try {
        localStorage.setItem(STORE_KEY, JSON.stringify(settingsState()));
      } catch {
        /* Storage can be unavailable; settings then live for the page only. */
      }
    }

    function loadSettings() {
      try {
        return JSON.parse(localStorage.getItem(STORE_KEY)) || null;
      } catch {
        return null;
      }
    }

    function applySettings(s) {
      // A fresh visit gets the cleaver preset, the pack that rewrites the buzzwords the
      // default profile only flags. A saved presets array, even an empty one, wins.
      const presets = s.presets || ["cleaver"];
      controls.useDefaults.checked = s.useDefaults !== false;
      controls.splitSemicolons.checked = s.splitSemicolons !== false;
      controls.collapseSpaces.checked = s.collapseSpaces !== false;
      controls.blockWords.value = s.blockWords || "";
      controls.wordSwaps.value = s.wordSwaps || "";
      controls.phraseSwaps.value = s.phraseSwaps || "";
      controls.charSwaps.value = s.charSwaps || "";
      controls.regexSwaps.value = s.regexSwaps || "";
      controls.flagPatterns.value = s.flagPatterns || "";
      controls.allow.value = s.allow || "";
      controls.rwProvider.value = s.rwProvider || "";
      controls.rwKey.value = s.rwKey || "";
      controls.rwModel.value = s.rwModel || "";
      controls.rwURL.value = s.rwURL || "";
      controls.rwOModel.value = s.rwOModel || "";
      controls.rwOKey.value = s.rwOKey || "";
      controls.rwTone.value = s.rwTone || "";
      controls.rwVerify.checked = s.rwVerify !== false;
      syncRewriteUI();
      const radio = root.querySelector('input[name="sc-dialect"][value="' + (s.dialect || "") + '"]');
      if (radio) radio.checked = true;
      for (const el of root.querySelectorAll(".sc-preset")) {
        el.checked = presets.includes(el.value);
      }
    }

    /* buildProfile merges the defaults under the user's entries, so an entry in the
       panel always wins, matching how presets merge in the CLI. */
    function buildProfile() {
      const base = controls.useDefaults.checked && defaults ? defaults : emptyProfile();
      return {
        charReplace: { ...base.charReplace, ...parsePairs(controls.charSwaps.value) },
        phraseReplace: { ...base.phraseReplace, ...parsePairs(controls.phraseSwaps.value) },
        wordReplace: { ...(base.wordReplace || {}), ...parsePairs(controls.wordSwaps.value) },
        regexReplace: { ...(base.regexReplace || {}), ...parsePairs(controls.regexSwaps.value) },
        flagPatterns: { ...(base.flagPatterns || {}), ...parsePairs(controls.flagPatterns.value) },
        blockWords: dedupe([...(base.blockWords || []), ...parseLines(controls.blockWords.value)]),
        allow: dedupe([...(base.allow || []), ...parseLines(controls.allow.value)]),
        collapseSpaces: controls.collapseSpaces.checked,
        splitSemicolons: controls.splitSemicolons.checked,
        dialect: dialectValue(),
        tone: parseLines(controls.rwTone.value),
      };
    }

    /* syncRewriteUI shows the fields for the picked provider and the Rewrite button
       once the provider is usable. */
    function syncRewriteUI() {
      const p = controls.rwProvider.value;
      $("sc-rw-anthropic").hidden = p !== "anthropic";
      $("sc-rw-openai").hidden = p !== "openai";
      $("sc-rw-tone-wrap").hidden = p === "";
      rewriteBtn.hidden = !rewriteReady();
    }

    /* rewriteReady reports whether the picked provider has what it needs to be called. */
    function rewriteReady() {
      switch (controls.rwProvider.value) {
        case "anthropic":
          return controls.rwKey.value.trim() !== "";
        case "openai":
          return controls.rwURL.value.trim() !== "" && controls.rwOModel.value.trim() !== "";
        default:
          return false;
      }
    }

    /* rewriteAnthropic sends the text to the Anthropic Messages API straight from the
       browser. The dangerous-direct-browser-access header opts into CORS; the key is
       the user's own and goes nowhere but Anthropic. */
    async function rewriteAnthropic(text, system) {
      const res = await fetch("https://api.anthropic.com/v1/messages", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          "x-api-key": controls.rwKey.value.trim(),
          "anthropic-version": "2023-06-01",
          "anthropic-dangerous-direct-browser-access": "true",
        },
        body: JSON.stringify({
          model: controls.rwModel.value.trim() || "claude-opus-4-8",
          max_tokens: 16000,
          system,
          messages: [{ role: "user", content: text }],
        }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(data.error && data.error.message ? data.error.message : "HTTP " + res.status);
      }
      if (data.stop_reason === "max_tokens") throw new Error("reply hit the token cap and is truncated");
      if (data.stop_reason === "refusal") throw new Error("the model declined to rewrite the text");
      const out = (data.content || [])
        .filter((b) => b.type === "text")
        .map((b) => b.text)
        .join("");
      if (!out) throw new Error("reply had no text content");
      return out.trim();
    }

    /* rewriteOpenAI sends the text to any OpenAI-compatible chat completions endpoint,
       which covers Ollama, LM Studio, vLLM, and most gateways. */
    async function rewriteOpenAI(text, system) {
      const url = controls.rwURL.value.trim().replace(/\/+$/, "") + "/v1/chat/completions";
      const headers = { "content-type": "application/json" };
      const key = controls.rwOKey.value.trim();
      if (key) headers.authorization = "Bearer " + key;
      const res = await fetch(url, {
        method: "POST",
        headers,
        body: JSON.stringify({
          model: controls.rwOModel.value.trim(),
          stream: false,
          messages: [
            { role: "system", content: system },
            { role: "user", content: text },
          ],
        }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        const msg = data.error && data.error.message ? data.error.message : "HTTP " + res.status;
        throw new Error(msg);
      }
      const out = data.choices && data.choices[0] && data.choices[0].message && data.choices[0].message.content;
      if (!out) throw new Error("reply had no text content");
      return out.trim();
    }

    /* jsonObject cuts the first-to-last-brace span out of a model reply, tolerating
       code fences or stray prose around the verdict, like the CLI judge does. */
    function jsonObject(text) {
      const start = text.indexOf("{");
      const end = text.lastIndexOf("}");
      return start >= 0 && end > start ? text.slice(start, end + 1) : "";
    }

    /* verifyRewrite asks the same model whether the rewrite kept the meaning and turns
       the verdict into a status line. A broken check never takes the rewrite away. */
    async function verifyRewrite(original, rewritten, send) {
      setStatus("Checking the rewrite kept your meaning...");
      const judge = JSON.parse(await callEngine("slopJudgePrompt", ""));
      const user = "ORIGINAL:\n" + original + "\n\nREWRITE:\n" + rewritten;
      try {
        const reply = await send(user, judge.system);
        const verdict = JSON.parse(jsonObject(reply));
        if (verdict.faithful) {
          setStatus("Rewritten. Meaning check passed.");
          return;
        }
        const notes = (verdict.issues || [])
          .slice(0, 3)
          .map((i) => i.note || i.kind)
          .join("; ");
        setStatus("Rewritten, but the meaning check flagged: " + (notes || "unspecified changes"), true);
      } catch (err) {
        setStatus("Rewritten. Meaning check did not run: " + err.message, true);
      }
    }

    /* rewrite sends the chopped text through the configured model, mirroring the CLI:
       rules first, then the model pass on the rules output, then the optional check. */
    async function rewrite() {
      const text = output.value.trim();
      if (!text || !rewriteReady() || !defaults) return;
      const promptRes = JSON.parse(await callEngine("slopRewritePrompt", JSON.stringify(buildProfile())));
      if (promptRes.error) {
        setStatus(promptRes.error, true);
        return;
      }
      rewriteBtn.disabled = true;
      const old = rewriteBtn.textContent;
      rewriteBtn.textContent = "Rewriting...";
      setStatus("");
      try {
        const send = controls.rwProvider.value === "anthropic" ? rewriteAnthropic : rewriteOpenAI;
        const rewritten = await send(text, promptRes.system);
        output.value = rewritten;
        renderOutMarks(input.value, rewritten);
        if (controls.rwVerify.checked) {
          await verifyRewrite(text, rewritten, send);
        } else {
          setStatus("Rewritten. The panes now differ: left is your input, right is the model's rewrite.");
        }
      } catch (err) {
        setStatus("Rewrite failed: " + err.message, true);
      } finally {
        rewriteBtn.disabled = false;
        rewriteBtn.textContent = old;
      }
    }

    function setStatus(text, isError) {
      status.textContent = text || "";
      status.classList.toggle("sc-error", Boolean(isError));
      status.hidden = !text;
    }

    function scoreClass(v) {
      if (v < 25) return "sc-score-low";
      if (v < 55) return "sc-score-mid";
      return "sc-score-high";
    }

    const scorePop = $("sc-score-pop");

    /* renderScore paints the chip and refreshes the breakdown behind it. The engine
       hands over the raw ingredients, so the popover shows its work. */
    function renderScore(res) {
      const s = res.score;
      score.textContent = "slop " + s.value;
      score.className = "sc-score " + scoreClass(s.value);
      score.hidden = false;
      $("sc-pop-value").textContent = s.value;
      $("sc-pop-tells").textContent = String(s.tells);
      $("sc-pop-words").textContent = String(s.words);
      $("sc-pop-density").textContent = s.tellsPer100 + " tells per 100 words";
      $("sc-pop-cadence").textContent =
        s.cadenceCv === 0
          ? "too short to judge"
          : s.cadenceCv < 0.5
            ? "flat (cv " + s.cadenceCv + "), even sentence lengths read machine-written"
            : "varied (cv " + s.cadenceCv + ")";
    }

    function setScorePop(open) {
      scorePop.hidden = !open;
      score.setAttribute("aria-expanded", String(open));
    }

    const MAX_ROWS = 400;

    function renderFindings(findings) {
      findingsList.textContent = "";
      if (!findings.length) {
        findingsBox.hidden = true;
        return;
      }
      const word = findings.length === 1 ? "tell" : "tells";
      findingsCount.textContent = findings.length + " " + word;
      for (const f of findings.slice(0, MAX_ROWS)) {
        const li = document.createElement("li");
        const pos = document.createElement("code");
        pos.className = "sc-pos";
        pos.textContent = f.line + ":" + f.col;
        const rule = document.createElement("span");
        rule.className = "sc-rule";
        rule.textContent = f.rule;
        const match = document.createElement("code");
        match.className = "sc-match";
        match.textContent = f.match;
        li.append(pos, rule, match);
        if (f.replacement !== undefined && f.replacement !== null) {
          const arrow = document.createElement("span");
          arrow.className = "sc-arrow";
          arrow.textContent = "→";
          const repl = document.createElement("code");
          repl.className = "sc-repl";
          repl.textContent = f.replacement === "" ? "(removed)" : f.replacement;
          li.append(arrow, repl);
        } else {
          const flag = document.createElement("span");
          flag.className = "sc-flag";
          flag.textContent = "flagged";
          li.append(flag);
        }
        findingsList.appendChild(li);
      }
      if (findings.length > MAX_ROWS) {
        const li = document.createElement("li");
        li.className = "sc-more";
        li.textContent = "and " + (findings.length - MAX_ROWS) + " more";
        findingsList.appendChild(li);
      }
      findingsBox.hidden = false;
    }

    /* MAX_MARK_BYTES stops the highlight mirror from doubling very large pastes. */
    const MAX_MARK_BYTES = 200000;

    /* renderMarks paints the finding highlights behind the input text. Finding offsets
       are byte offsets from the engine, so segments are cut on the encoded text. */
    function renderMarks(text, findings) {
      // Pull the mirror in by the scrollbar width, if the platform draws one, so both
      // layers wrap at the same column. The two comes from the textarea borders.
      marks.style.right = Math.max(0, input.offsetWidth - input.clientWidth - 2) + "px";
      marks.textContent = "";
      const enc = new TextEncoder();
      const bytes = enc.encode(text);
      if (!findings.length || bytes.length > MAX_MARK_BYTES) {
        marks.textContent = text;
        return;
      }
      const dec = new TextDecoder();
      let prev = 0;
      for (const f of findings) {
        const end = f.offset + enc.encode(f.match).length;
        if (f.offset < prev || end > bytes.length) continue;
        if (f.offset > prev) {
          marks.appendChild(document.createTextNode(dec.decode(bytes.subarray(prev, f.offset))));
        }
        const m = document.createElement("mark");
        m.textContent = f.match;
        marks.appendChild(m);
        prev = end;
      }
      marks.appendChild(document.createTextNode(dec.decode(bytes.subarray(prev))));
    }

    /* renderOutMarks paints what changed behind the output text, diffing token by
       token against the input. Texts too far apart fall back to a plain mirror. */
    function renderOutMarks(from, to) {
      outMarks.style.right = Math.max(0, output.offsetWidth - output.clientWidth - 2) + "px";
      outMarks.textContent = "";
      if (!to || to.length > MAX_MARK_BYTES) {
        outMarks.textContent = to;
        return;
      }
      const ops = diffOps(tokens(from), tokens(to));
      if (!ops) {
        outMarks.textContent = to;
        return;
      }
      for (const [op, token] of ops) {
        if (op < 0) continue;
        if (op === 0 || /^\s+$/.test(token)) {
          outMarks.appendChild(document.createTextNode(token));
        } else {
          const m = document.createElement("mark");
          m.textContent = token;
          outMarks.appendChild(m);
        }
      }
    }

    /* chop runs the engine over the current input and paints the result. It runs in
       the worker, so only the newest call paints and slow chops show a status line
       instead of freezing the page. */
    let chopTicket = 0;
    async function chop() {
      if (!defaults) return;
      const text = input.value;
      if (!text.trim()) {
        output.value = "";
        marks.textContent = "";
        outMarks.textContent = "";
        score.hidden = true;
        setScorePop(false);
        findingsBox.hidden = true;
        setStatus("");
        return;
      }
      const req = { text, profile: buildProfile(), presets: selectedPresets() };
      const ticket = ++chopTicket;
      const slow = setTimeout(() => setStatus("Chopping..."), 300);
      let res;
      try {
        res = JSON.parse(await callEngine("slopChop", JSON.stringify(req)));
      } catch (err) {
        clearTimeout(slow);
        if (ticket === chopTicket) setStatus("Engine error: " + err.message, true);
        return;
      }
      clearTimeout(slow);
      if (ticket !== chopTicket) return;
      if (res.error) {
        setStatus(res.error, true);
        return;
      }
      setStatus("");
      output.value = res.output;
      renderMarks(text, res.findings);
      renderOutMarks(text, res.output);
      renderScore(res);
      renderFindings(res.findings);
    }

    let timer = 0;
    function chopSoon() {
      clearTimeout(timer);
      timer = setTimeout(chop, 120);
    }

    function chopNow() {
      saveSettings();
      chop();
    }

    /* flash swaps a button label briefly to confirm a click. */
    function flash(btn, label) {
      const old = btn.textContent;
      btn.textContent = label;
      setTimeout(() => {
        btn.textContent = old;
      }, 1200);
    }

    async function toClipboard(text) {
      try {
        await navigator.clipboard.writeText(text);
        return true;
      } catch {
        return false;
      }
    }

    /* MAX_FILE_BYTES caps a dropped file at what the panes hold comfortably. */
    const MAX_FILE_BYTES = 2 * 1024 * 1024;

    /* droppedName remembers the last loaded file, so Download saves the chopped text
       under the same name and the result drops back in place of the original. */
    let droppedName = "";

    /* loadFile reads a dropped file into the input pane and chops it. */
    async function loadFile(file) {
      if (!file) return;
      if (file.size > MAX_FILE_BYTES) {
        const mb = (file.size / 1048576).toFixed(1);
        setStatus(file.name + " is " + mb + " MB. The pane takes up to 2 MB.", true);
        return;
      }
      let text;
      try {
        text = await file.text();
      } catch (err) {
        setStatus("Could not read " + file.name + ": " + err.message, true);
        return;
      }
      if (text.includes("\u0000")) {
        setStatus(file.name + " looks like binary, not text.", true);
        return;
      }
      droppedName = file.name;
      clearTimeout(timer);
      input.value = text;
      await chop();
      setStatus("Loaded " + file.name + ". Download saves the chopped copy.");
    }

    function renderPresets() {
      const box = $("sc-presets");
      box.textContent = "";
      for (const name of presetNames) {
        const label = document.createElement("label");
        const cb = document.createElement("input");
        cb.type = "checkbox";
        cb.className = "sc-preset";
        cb.value = name;
        label.append(cb, " " + name);
        box.appendChild(label);
      }
    }

    function setDrawer(open) {
      drawer.hidden = !open;
      burger.setAttribute("aria-expanded", String(open));
    }

    /* Wire the static controls. The engine may still be loading; chop is a no-op
       until it lands, and the load path re-chops once ready. */
    input.addEventListener("input", chopSoon);
    input.addEventListener("scroll", () => {
      marks.scrollTop = input.scrollTop;
      marks.scrollLeft = input.scrollLeft;
    });
    output.addEventListener("scroll", () => {
      outMarks.scrollTop = output.scrollTop;
      outMarks.scrollLeft = output.scrollLeft;
    });
    clearBtn.addEventListener("click", () => {
      input.value = "";
      droppedName = "";
      chop();
      input.focus();
    });
    copyBtn.addEventListener("click", async () => {
      if (await toClipboard(output.value)) flash(copyBtn, "Copied");
    });
    downloadBtn.addEventListener("click", () => {
      if (!output.value) return;
      const blob = new Blob([output.value], { type: "text/plain;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = droppedName || "chopped.txt";
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
      flash(downloadBtn, "Saved");
    });

    /* File drop: the input pane takes a text file. Dragged text is not intercepted,
       so selection drops still land the way the textarea handles them natively. */
    const editor = input.closest(".sc-editor");
    let dragDepth = 0;
    editor.addEventListener("dragenter", (e) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      dragDepth++;
      editor.classList.add("sc-dropping");
    });
    editor.addEventListener("dragover", (e) => {
      if (hasFiles(e)) e.preventDefault();
    });
    editor.addEventListener("dragleave", () => {
      if (dragDepth > 0 && --dragDepth === 0) editor.classList.remove("sc-dropping");
    });
    editor.addEventListener("drop", (e) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      dragDepth = 0;
      editor.classList.remove("sc-dropping");
      loadFile(e.dataTransfer.files[0]);
    });
    rewriteBtn.addEventListener("click", rewrite);
    score.addEventListener("click", () => setScorePop(scorePop.hidden));
    document.addEventListener("click", (e) => {
      if (!scorePop.hidden && !scorePop.contains(e.target) && e.target !== score) {
        setScorePop(false);
      }
    });
    burger.addEventListener("click", () => setDrawer(drawer.hidden));
    closeBtn.addEventListener("click", () => setDrawer(false));
    root.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        if (!drawer.hidden) setDrawer(false);
        if (!scorePop.hidden) setScorePop(false);
      }
    });
    drawer.addEventListener("change", () => {
      syncRewriteUI();
      chopNow();
    });
    drawer.addEventListener("input", (e) => {
      if (e.target.tagName === "TEXTAREA" || e.target.tagName === "INPUT") {
        saveSettings();
        syncRewriteUI();
        chopSoon();
      }
    });
    $("sc-export").addEventListener("click", async () => {
      const json = JSON.stringify(buildProfile(), null, 2);
      if (await toClipboard(json)) flash($("sc-export"), "Copied");
    });
    $("sc-share").addEventListener("click", async () => {
      // Keys never ride in a link. Everything else about the setup does.
      const state = settingsState();
      delete state.rwKey;
      delete state.rwOKey;
      const url = new URL(document.baseURI);
      url.hash = "s=" + encodeShare(state);
      if (await toClipboard(url.href)) flash($("sc-share"), "Copied");
    });
    $("sc-reset").addEventListener("click", () => {
      try {
        localStorage.removeItem(STORE_KEY);
      } catch {
        /* Nothing to clear. */
      }
      applySettings({});
      chop();
    });

    if (!input.value) input.value = SAMPLE;
    setStatus("Loading the chopper...");

    loadEngine()
      .then(() => {
        renderPresets();
        const shared = readShareHash();
        applySettings(shared || loadSettings() || {});
        if (shared) {
          saveSettings();
          history.replaceState(null, "", location.pathname + location.search);
        }
        if (engineTag) engineTag.textContent = "engine " + engineVersion;
        setStatus("");
        chop().then(() => {
          if (shared) setStatus("Settings loaded from the shared link.");
        });
      })
      .catch((err) => {
        setStatus("Engine failed to load: " + err.message, true);
      });
  }

  function bootIfPresent() {
    const root = document.getElementById("sc-app");
    if (!root || root.dataset.booted) return;
    root.dataset.booted = "1";
    boot(root);
  }

  /* Material's instant navigation replaces the page body without a reload, so hook
     its document stream when present and fall back to the DOM event. */
  if (window.document$ && typeof window.document$.subscribe === "function") {
    window.document$.subscribe(bootIfPresent);
  } else {
    document.addEventListener("DOMContentLoaded", bootIfPresent);
    bootIfPresent();
  }
})();
