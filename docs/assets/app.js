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

  let wasmReady = null;
  let defaults = null;
  let presetNames = [];

  /* loadWasm fetches the engine once and resolves when its globals are registered. */
  function loadWasm() {
    if (wasmReady) return wasmReady;
    wasmReady = (async () => {
      if (!globalThis.Go) {
        await new Promise((resolve, reject) => {
          const s = document.createElement("script");
          s.src = new URL("assets/wasm_exec.js", document.baseURI).href;
          s.onload = resolve;
          s.onerror = () => reject(new Error("wasm_exec.js failed to load"));
          document.head.appendChild(s);
        });
      }
      const go = new Go();
      const url = new URL("assets/slop-chop.wasm", document.baseURI).href;
      let result;
      try {
        result = await WebAssembly.instantiateStreaming(fetch(url), go.importObject);
      } catch {
        const buf = await (await fetch(url)).arrayBuffer();
        result = await WebAssembly.instantiate(buf, go.importObject);
      }
      go.run(result.instance);
      await new Promise((r) => setTimeout(r, 0));
      defaults = JSON.parse(globalThis.slopDefaults());
      presetNames = JSON.parse(globalThis.slopPresets());
    })();
    return wasmReady;
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

  /* App wires one #sc-app element to the engine. */
  function boot(root) {
    const $ = (id) => root.querySelector("#" + id);
    const input = $("sc-in");
    const output = $("sc-out");
    const score = $("sc-score");
    const status = $("sc-status");
    const copyBtn = $("sc-copy");
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
    };

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
      const radio = root.querySelector('input[name="sc-dialect"][value="' + (s.dialect || "") + '"]');
      if (radio) radio.checked = true;
      for (const el of root.querySelectorAll(".sc-preset")) {
        el.checked = (s.presets || []).includes(el.value);
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
      };
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

    function renderScore(res) {
      score.textContent = "slop " + res.score.value;
      score.className = "sc-score " + scoreClass(res.score.value);
      score.hidden = false;
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

    /* chop runs the engine over the current input and paints the result. */
    function chop() {
      if (!globalThis.slopChop) return;
      const text = input.value;
      if (!text.trim()) {
        output.value = "";
        score.hidden = true;
        findingsBox.hidden = true;
        setStatus("");
        return;
      }
      const req = { text, profile: buildProfile(), presets: selectedPresets() };
      const res = JSON.parse(globalThis.slopChop(JSON.stringify(req)));
      if (res.error) {
        setStatus(res.error, true);
        return;
      }
      setStatus("");
      output.value = res.output;
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
    clearBtn.addEventListener("click", () => {
      input.value = "";
      chop();
      input.focus();
    });
    copyBtn.addEventListener("click", async () => {
      if (await toClipboard(output.value)) flash(copyBtn, "Copied");
    });
    burger.addEventListener("click", () => setDrawer(drawer.hidden));
    closeBtn.addEventListener("click", () => setDrawer(false));
    root.addEventListener("keydown", (e) => {
      if (e.key === "Escape" && !drawer.hidden) setDrawer(false);
    });
    drawer.addEventListener("change", chopNow);
    drawer.addEventListener("input", (e) => {
      if (e.target.tagName === "TEXTAREA") {
        saveSettings();
        chopSoon();
      }
    });
    $("sc-export").addEventListener("click", async () => {
      const json = JSON.stringify(buildProfile(), null, 2);
      if (await toClipboard(json)) flash($("sc-export"), "Copied");
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

    loadWasm()
      .then(() => {
        renderPresets();
        const saved = loadSettings();
        if (saved) applySettings(saved);
        if (engineTag && globalThis.slopVersion) {
          engineTag.textContent = "engine " + globalThis.slopVersion();
        }
        setStatus("");
        chop();
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
