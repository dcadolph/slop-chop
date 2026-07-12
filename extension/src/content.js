/* Runs on every page. When the hotkey fires, the service worker sends "do-chop"; this reads
   the focused text field, sends its text to the engine, writes the cleaned text back, and
   shows a short before/after badge. The engine runs inside the extension, so the text never
   leaves the browser. */
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
// change instead of snapping back to its old model value.
function writeText(el, text) {
  if (el.tagName === "TEXTAREA" || el.tagName === "INPUT") {
    const proto =
      el.tagName === "TEXTAREA" ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    const setter = Object.getOwnPropertyDescriptor(proto, "value").set;
    setter.call(el, text);
  } else {
    el.textContent = text;
  }
  el.dispatchEvent(new Event("input", { bubbles: true }));
}

// toast shows a short-lived badge in the corner.
function toast(message) {
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
  writeText(el, res.output);
  const drop =
    res.before != null && res.after != null ? " (slop " + res.before + " → " + res.after + ")" : "";
  toast("Chopped" + drop);
}

chrome.runtime.onMessage.addListener((msg) => {
  if (msg && msg.type === "do-chop") chopFocused();
});
