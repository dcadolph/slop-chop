/* A looping, self-contained demo for the extension page: a mock text field whose sloppy text
   is chopped to clean text while the slop score counts down. It runs only where the demo
   element exists and holds still when the visitor prefers reduced motion. Nothing here touches
   the real engine; it is a scripted illustration. */
(() => {
  "use strict";

  const root = document.getElementById("sc-ext-demo");
  if (!root) return;

  const BEFORE =
    "In today's fast-paced, digital-first landscape, teams leverage a myriad of tools to stay aligned.";
  const AFTER = "Teams use many tools to stay aligned.";

  const textEl = document.getElementById("sc-ext-text");
  const scoreEl = document.getElementById("sc-ext-score");
  const btn = document.getElementById("sc-ext-btn");
  const toast = document.getElementById("sc-ext-toast");

  // setScore paints the score chip and colors it by band.
  function setScore(v) {
    scoreEl.textContent = "slop " + v;
    scoreEl.className = "sc-ext-score " + (v < 25 ? "low" : v < 55 ? "mid" : "high");
  }

  const reduce =
    window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (reduce) {
    textEl.textContent = AFTER;
    textEl.classList.add("clean");
    setScore(2);
    return;
  }

  let raf = 0;

  // countTo animates the score from one value to another with an ease-out.
  function countTo(from, to, ms) {
    const start = performance.now();
    cancelAnimationFrame(raf);
    const step = (now) => {
      const t = Math.min(1, (now - start) / ms);
      const eased = 1 - Math.pow(1 - t, 3);
      setScore(Math.round(from + (to - from) * eased));
      if (t < 1) raf = requestAnimationFrame(step);
    };
    raf = requestAnimationFrame(step);
  }

  // cycle plays one full sloppy-to-clean pass and schedules the next.
  function cycle() {
    textEl.style.opacity = "1";
    textEl.textContent = BEFORE;
    textEl.classList.remove("clean");
    setScore(80);
    toast.classList.remove("show");
    btn.classList.remove("press");

    setTimeout(() => btn.classList.add("press"), 1200);
    setTimeout(() => (textEl.style.opacity = "0"), 1650);
    setTimeout(() => {
      textEl.textContent = AFTER;
      textEl.classList.add("clean");
      textEl.style.opacity = "1";
      countTo(80, 2, 900);
      toast.textContent = "Chopped · slop 80 → 2";
      toast.classList.add("show");
    }, 1950);
    setTimeout(() => {
      btn.classList.remove("press");
      toast.classList.remove("show");
    }, 4400);
    setTimeout(cycle, 6200);
  }

  cycle();
})();
