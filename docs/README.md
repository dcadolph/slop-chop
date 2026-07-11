---
hide:
  - navigation
  - toc
---

<div class="sc-hero" markdown>

![slop-chop](assets/icon.png){ .hero-logo }

# slop-chop

<p class="tagline">Chop the slop.</p>

<p class="subtitle">Paste in text and get back something that reads like a person wrote it. A fast, deterministic rules pass pulls the AI tells in one go, with an optional model rewrite for the work rules cannot do.</p>

[Get started](quickstart.md){ .md-button .md-button--primary }
[View on GitHub](https://github.com/dcadolph/slop-chop){ .md-button }

</div>

<div id="sc-app" class="sc-app">
<div class="sc-app-head">
<div class="sc-app-title"><strong>Chop it right here</strong><span class="sc-app-note">Runs in your browser. Your text never leaves the page.</span></div>
<div class="sc-app-actions">
<span id="sc-score" class="sc-score" title="How much the input reads like AI wrote it, from 0 for clean to 100 for heavy slop." hidden></span>
<button id="sc-settings-btn" class="sc-iconbtn" type="button" aria-label="Settings" aria-expanded="false" aria-controls="sc-drawer"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" aria-hidden="true"><path d="M4 6h16M4 12h16M4 18h16"/></svg></button>
</div>
</div>
<div class="sc-panes">
<div class="sc-pane">
<div class="sc-pane-bar"><span>Slop in</span><button id="sc-clear" type="button">Clear</button></div>
<div class="sc-editor">
<div id="sc-marks" class="sc-marks" aria-hidden="true"></div>
<textarea id="sc-in" spellcheck="false" placeholder="Paste your slop..."></textarea>
</div>
</div>
<div class="sc-pane">
<div class="sc-pane-bar"><span>Chopped</span><button id="sc-copy" type="button">Copy</button></div>
<textarea id="sc-out" readonly spellcheck="false" placeholder="Clean text lands here."></textarea>
</div>
</div>
<p id="sc-status" class="sc-status" hidden></p>
<details id="sc-findings" class="sc-findings" hidden>
<summary><span id="sc-findings-count"></span></summary>
<ul id="sc-findings-list"></ul>
</details>
<div id="sc-drawer" class="sc-drawer" hidden>
<div class="sc-drawer-head"><strong>Settings</strong><button id="sc-drawer-close" type="button">Close</button></div>
<section>
<h3>Rules</h3>
<label><input type="checkbox" id="sc-use-defaults" checked> Built-in default rules</label>
<label><input type="checkbox" id="sc-split-semicolons" checked> Split semicolons into sentences</label>
<label><input type="checkbox" id="sc-collapse-spaces" checked> Collapse doubled spaces</label>
</section>
<section>
<h3>Spelling dialect</h3>
<div class="sc-inline">
<label><input type="radio" name="sc-dialect" value="" checked> Off</label>
<label><input type="radio" name="sc-dialect" value="american"> American</label>
<label><input type="radio" name="sc-dialect" value="british"> British</label>
</div>
</section>
<section>
<h3>Presets</h3>
<div id="sc-presets" class="sc-inline"></div>
</section>
<section>
<h3>Your rules</h3>
<label class="sc-field">Block words<small>One per line. Flagged, never rewritten.</small><textarea id="sc-block-words" placeholder="synergy&#10;deep dive"></textarea></label>
<label class="sc-field">Word swaps<small>One per line, from =&gt; to.</small><textarea id="sc-word-swaps" placeholder="utilize =&gt; use"></textarea></label>
<label class="sc-field">Phrase rewrites<small>One per line, from =&gt; to. An empty to deletes the phrase.</small><textarea id="sc-phrase-swaps" placeholder="going forward, =&gt;"></textarea></label>
<label class="sc-field">Character swaps<small>One per line, from =&gt; to.</small><textarea id="sc-char-swaps" placeholder="&#8594; =&gt; -&gt;"></textarea></label>
<label class="sc-field">Regex rewrites<small>One per line, pattern =&gt; replacement. $1 expands.</small><textarea id="sc-regex-swaps" placeholder="\bvery (\w+) =&gt; $1"></textarea></label>
<label class="sc-field">Flag patterns<small>One per line, name =&gt; pattern. Flag only, never rewrite.</small><textarea id="sc-flag-patterns" placeholder="hedge =&gt; (?i)\bit seems\b"></textarea></label>
<label class="sc-field">Allow list<small>One per line. Never flag or rewrite these.</small><textarea id="sc-allow" placeholder="delve"></textarea></label>
</section>
<div class="sc-drawer-foot">
<button id="sc-reset" type="button">Reset</button>
<button id="sc-export" type="button">Copy profile JSON</button>
<span id="sc-engine" class="sc-engine"></span>
</div>
</div>
</div>

<div class="sc-terminal">
<div class="sc-terminal-bar"><span class="dot red"></span><span class="dot yellow"></span><span class="dot green"></span><span class="title">slop-chop</span></div>
<div class="sc-terminal-body"><span class="prompt">$</span> <span class="cmd">echo "In summary, a robust—and seamless—result." | slop-chop fix</span>
<span class="out">The result works.</span>
<span class="prompt">$</span> <span class="cmd">slop-chop check notes.md</span>
<span class="out">notes.md:1:1  opener   "In summary"
notes.md:1:14 word     "robust"
notes.md:1:22 char     em-dash
3 tells found</span>
<span class="prompt">$</span> <span class="cmd">slop-chop score notes.md</span>
<span class="out">7</span></div>
</div>

<div class="sc-install" markdown>

```sh
brew install dcadolph/tap/slop-chop
```

</div>

## How it works

<div class="sc-steps">
<div class="step"><span class="num">1</span><strong>Rules pass</strong>Fast and deterministic. Swaps characters, drops flagged words, rewrites stock phrases, fixes spelling to one dialect, tidies punctuation. No model, no cost, same output every run. Code blocks come through untouched.</div>
<div class="step"><span class="num">2</span><strong>Score</strong>One number from 0 for clean to 100 for heavy slop. It weighs rule tells against flat, machine-like sentence cadence. Pass <code>--max</code> to gate a build.</div>
<div class="step"><span class="num">3</span><strong>Rewrite</strong>Optional. Hands the text to a model for the things rules cannot manage, like reworking a sentence so it no longer needs a semicolon, or bending the writing toward your voice.</div>
</div>

## Why slop-chop

<div class="grid cards" markdown>

-   :material-flash:{ .lg .middle } __Deterministic and free__

    ---

    The rules pass runs with no model and no cost, and gives the same output on every run. It knows markdown, so fenced code and inline backticks come through untouched.

-   :material-source-branch:{ .lg .middle } __Drop it in CI__

    ---

    `check` fails a pull request when it finds slop. `fix` can push the cleanup back to the branch. There is a ready-made GitHub Action.

-   :material-robot-happy:{ .lg .middle } __Claude Code plugin__

    ---

    The repo is its own marketplace. Install the plugin to get a `slop-chop` skill and a `/slop-chop` command that drive the binary for you.

-   :material-tune:{ .lg .middle } __Reads like you__

    ---

    Profiles and presets say what to cut and what to put in its place. Bring your own cut list so the result sounds like you, not a chatbot.

</div>

## Start here

- [Quickstart](quickstart.md): install and clean your first file in a couple of minutes.
- [Profiles](PROFILE.md): every field, the presets, the spelling dialects, and the allow list.
- [Claude plugin](PLUGIN.md): install, the skill, the command, backends, and troubleshooting.
