/* The toolbar popup is a paste-and-chop mini tool. It sends text to the same engine the rest
   of the extension uses and shows the cleaned text with the before and after slop scores. */
"use strict";

const byId = (id) => document.getElementById(id);

byId("chop").addEventListener("click", async () => {
  const text = byId("in").value;
  if (!text.trim()) return;
  byId("score").textContent = "...";
  let res;
  try {
    res = await chrome.runtime.sendMessage({ type: "chop", text });
  } catch (err) {
    byId("score").textContent = String((err && err.message) || err);
    return;
  }
  if (!res || res.error) {
    byId("score").textContent = (res && res.error) || "no response";
    return;
  }
  byId("out").value = res.output;
  byId("score").textContent = res.before != null ? "slop " + res.before + " → " + res.after : "";
});

byId("opts").addEventListener("click", () => chrome.runtime.openOptionsPage());
