/* Runs on every page. It reads the focused text field, sends the text to the engine, writes
   the cleaned text back, and shows a short before/after badge. A small chop button floats by
   the focused field, and the hotkey (relayed by the service worker) does the same thing. The
   engine runs inside the extension, so the text never leaves the browser. */
"use strict";

// editableTarget returns the focused element when it is one we can rewrite, else null.
function editableTarget() {
  const el = document.activeElement;
  if (!el) return null;
  if (el.tagName === "TEXTAREA") return el;
  if (el.tagName === "INPUT" && /^(text|search|url|email|)$/i.test(el.type)) return el;
  if (el.isContentEditable) return el;
  return null;
}

// readText pulls the current text out of a field.
function readText(el) {
  if (el.tagName === "TEXTAREA" || el.tagName === "INPUT") return el.value;
  return el.innerText;
}

// writeText puts text back and fires an input event, so a framework-bound field notices the
// change instead of snapping back to its old model value. A contenteditable is rewritten
// through the editor's own insertText command: the host editor keeps its paragraph
// structure and its undo stack, instead of having its subtree flattened to one text node.
function writeText(el, text) {
  if (el.tagName === "TEXTAREA" || el.tagName === "INPUT") {
    const proto =
      el.tagName === "TEXTAREA" ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    const setter = Object.getOwnPropertyDescriptor(proto, "value").set;
    setter.call(el, text);
    el.dispatchEvent(new Event("input", { bubbles: true }));
    return;
  }
  el.focus();
  const range = document.createRange();
  range.selectNodeContents(el);
  const sel = getSelection();
  sel.removeAllRanges();
  sel.addRange(range);
  if (!document.execCommand("insertText", false, text)) {
    el.textContent = text;
    el.dispatchEvent(new Event("input", { bubbles: true }));
  }
}

// toast shows a short-lived badge in the corner. It no-ops on a document with no body,
// like a bare XML or SVG file the content script also runs on, where appendChild would throw.
function toast(message) {
  if (!document.body) return;
  const t = document.createElement("div");
  t.textContent = message;
  t.style.cssText =
    "position:fixed;z-index:2147483647;bottom:16px;right:16px;background:#111;color:#9bcf1a;" +
    "font:600 13px/1.4 system-ui,sans-serif;padding:8px 12px;border-radius:8px;" +
    "box-shadow:0 4px 16px rgba(0,0,0,.3);opacity:0;transition:opacity .15s";
  document.body.appendChild(t);
  requestAnimationFrame(() => (t.style.opacity = "1"));
  setTimeout(() => {
    t.style.opacity = "0";
    setTimeout(() => t.remove(), 200);
  }, 2200);
}

// chopFocused runs the whole flow for whatever field has focus.
async function chopFocused() {
  const el = editableTarget();
  if (!el) {
    toast("slop-chop: focus a text field first");
    return;
  }
  const text = readText(el);
  if (!text.trim()) return;
  let res;
  try {
    res = await chrome.runtime.sendMessage({ type: "chop", text });
  } catch (err) {
    toast("slop-chop: " + String((err && err.message) || err));
    return;
  }
  if (!res || res.error) {
    toast("slop-chop: " + ((res && res.error) || "no response"));
    return;
  }
  if (res.output === text) {
    toast("Already clean" + (res.before != null ? " (slop " + res.before + ")" : ""));
    return;
  }
  // The first chop can take seconds while the engine boots. Anything typed in the meantime
  // must not be clobbered by the stale snapshot's rewrite.
  if (readText(el) !== text) {
    toast("slop-chop: the field changed while chopping; nothing applied");
    return;
  }
  writeText(el, res.output);
  const drop =
    res.before != null && res.after != null ? " (slop " + res.before + " → " + res.after + ")" : "";
  toast("Chopped" + drop);
}

// The floating chop button sits at the corner of the focused field, so the feature is
// discoverable without the hotkey. mousedown is swallowed to keep focus on the field.
let chopBtn = null;

// ensureButton builds the floating button once and wires its click to a chop. A page that
// swaps its body (Turbo-style navigation) detaches the node, so a disconnected button is
// re-appended rather than styled invisibly off-tree.
function ensureButton() {
  if (!document.body) return null;
  if (chopBtn) {
    if (!chopBtn.isConnected) document.body.appendChild(chopBtn);
    return chopBtn;
  }
  chopBtn = document.createElement("button");
  chopBtn.type = "button";
  chopBtn.textContent = "✂"; // scissors
  chopBtn.setAttribute("aria-label", "Chop the slop");
  chopBtn.title = "Chop the slop";
  chopBtn.style.cssText =
    "position:fixed;z-index:2147483646;width:26px;height:26px;padding:0;border:none;" +
    "border-radius:7px;background:#111;color:#9bcf1a;font:700 14px/1 system-ui,sans-serif;" +
    "cursor:pointer;box-shadow:0 2px 8px rgba(0,0,0,.3);display:none";
  chopBtn.addEventListener("mousedown", (e) => e.preventDefault());
  chopBtn.addEventListener("click", () => chopFocused());
  document.body.appendChild(chopBtn);
  return chopBtn;
}

// placeButton shows the button at the bottom-right corner of a field, or hides it when the
// field is too small to be worth chopping.
function placeButton(el) {
  const b = ensureButton();
  if (!b) return;
  const r = el.getBoundingClientRect();
  if (r.width < 80 || r.height < 24 || r.bottom < 0 || r.top > innerHeight) {
    b.style.display = "none";
    return;
  }
  b.style.top = Math.min(innerHeight - 30, Math.max(4, r.bottom - 30)) + "px";
  b.style.left = Math.min(innerWidth - 30, Math.max(4, r.right - 30)) + "px";
  b.style.display = "block";
}

// hideButton hides the floating button unless a field still has focus.
function hideButton() {
  if (chopBtn && !editableTarget()) chopBtn.style.display = "none";
}

document.addEventListener("focusin", () => {
  const el = editableTarget();
  if (el) placeButton(el);
  else hideButton();
});
document.addEventListener("focusout", () => setTimeout(hideButton, 150));
document.addEventListener(
  "scroll",
  () => {
    const el = editableTarget();
    if (el) placeButton(el);
  },
  true,
);
addEventListener("resize", () => {
  const el = editableTarget();
  if (el) placeButton(el);
});

chrome.runtime.onMessage.addListener((msg) => {
  if (msg && msg.type === "do-chop") chopFocused();
});
