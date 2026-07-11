/* Runs the slop-chop engine in a Web Worker so a giant paste never freezes the page.
   The page sends {id, fn, arg} and gets {id, result} or {id, error} back. Paths are
   relative to this script, which lives next to the wasm binary. */
"use strict";

importScripts("wasm_exec.js");

const go = new Go();

WebAssembly.instantiateStreaming(fetch("slop-chop.wasm"), go.importObject)
  .catch(async () => {
    const res = await fetch("slop-chop.wasm");
    return WebAssembly.instantiate(await res.arrayBuffer(), go.importObject);
  })
  .then((result) => {
    go.run(result.instance);
    /* Give the Go runtime a tick to register its globals, then hand the page
       everything it needs to render the settings panel. */
    setTimeout(() => {
      postMessage({
        type: "ready",
        defaults: self.slopDefaults(),
        presets: self.slopPresets(),
        version: self.slopVersion(),
      });
    }, 0);
  })
  .catch((err) => {
    postMessage({ type: "fail", error: String((err && err.message) || err) });
  });

onmessage = (e) => {
  const { id, fn, arg } = e.data;
  try {
    if (fn !== "slopChop" && fn !== "slopRewritePrompt" && fn !== "slopJudgePrompt") {
      throw new Error("unknown engine call: " + fn);
    }
    postMessage({ id, result: self[fn](arg) });
  } catch (err) {
    postMessage({ id, error: String((err && err.message) || err) });
  }
};
