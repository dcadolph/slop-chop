---
hide:
  - navigation
  - toc
---

<div class="sc-hero" markdown>

![slop-chop](assets/icon.png){ .hero-logo }

# slop-chop

<p class="tagline">Chop the slop.</p>

<p class="sc-artifacts">Emails. Resumes. Blog posts. Docs. LinkedIn. READMEs.</p>

<p class="subtitle">Paste text that sounds like a bot, get back text that sounds like you. Em-dashes, buzzwords, and stock phrases all get chopped in one pass, right in your browser. Plug in a model when you want a deeper rewrite.</p>

[Get started](quickstart.md){ .md-button .md-button--primary }
[View on GitHub](https://github.com/dcadolph/slop-chop){ .md-button }

</div>

<div id="sc-app" class="sc-app">
<div class="sc-app-head">
<div class="sc-app-title"><strong>Chop it right here</strong><span class="sc-app-note">Runs in your browser. Your text never leaves the page.</span></div>
<div class="sc-app-actions">
<button id="sc-score" class="sc-score" type="button" title="Click for the breakdown." aria-expanded="false" aria-controls="sc-score-pop" hidden></button>
<span id="sc-score-after" class="sc-score sc-score-after" title="Slop score after the chop." hidden></span>
<button id="sc-settings-btn" class="sc-iconbtn" type="button" aria-label="Settings" aria-expanded="false" aria-controls="sc-drawer"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" aria-hidden="true"><path d="M4 6h16M4 12h16M4 18h16"/></svg></button>
<div id="sc-score-pop" class="sc-score-pop" hidden>
<div class="sc-score-pop-head"><strong>Slop score: <span id="sc-pop-value"></span> of 100</strong></div>
<p class="sc-score-pop-what">How much the input reads like AI wrote it. It weighs the density of tells against how flat the sentence rhythm is.</p>
<div class="sc-score-legend">
<span><i class="dot low"></i>under 25 reads clean</span>
<span><i class="dot mid"></i>25 to 54 mixed</span>
<span><i class="dot high"></i>55 and up heavy slop</span>
</div>
<dl class="sc-score-stats">
<div><dt>Tells</dt><dd id="sc-pop-tells"></dd></div>
<div><dt>Words</dt><dd id="sc-pop-words"></dd></div>
<div><dt>Density</dt><dd id="sc-pop-density"></dd></div>
<div><dt>Rhythm</dt><dd id="sc-pop-cadence"></dd></div>
</dl>
</div>
</div>
</div>
<div class="sc-panes">
<div class="sc-pane">
<div class="sc-pane-bar"><span>Slop in</span><button id="sc-clear" type="button">Clear</button></div>
<div class="sc-editor">
<div id="sc-marks" class="sc-marks" aria-hidden="true"></div>
<textarea id="sc-in" spellcheck="false" placeholder="Paste your slop or drop a file..."></textarea>
<div class="sc-drop-hint" aria-hidden="true">Drop to chop</div>
</div>
</div>
<div class="sc-pane">
<div class="sc-pane-bar"><span>Chopped</span><span class="sc-pane-actions"><button id="sc-restore" type="button" hidden title="Put the rules output back in the pane.">Restore</button><button id="sc-rewrite" type="button" hidden>Rewrite</button><button id="sc-download" type="button">Download</button><button id="sc-copy" type="button">Copy</button></span></div>
<div class="sc-editor">
<div id="sc-out-marks" class="sc-marks" aria-hidden="true"></div>
<textarea id="sc-out" readonly spellcheck="false" placeholder="Clean text lands here."></textarea>
</div>
</div>
</div>
<p class="sc-legend" aria-hidden="true"><span class="sc-swatch sc-swatch-slop"></span>Amber is slop found.<span class="sc-swatch sc-swatch-fix"></span>Green is what changed.</p>
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
<h3>Your voice</h3>
<label class="sc-field">Keep<small>One per line. Words and phrases to never flag or cut. Wins over the presets.</small><textarea id="sc-voice-keep" placeholder="gnarly&#10;ship it"></textarea></label>
<label class="sc-field">Prefer<small>One per line, from =&gt; to. Your swap wins over a preset. An empty to drops the word.</small><textarea id="sc-voice-prefer" placeholder="utilize =&gt; use&#10;a myriad of =&gt; a bunch of"></textarea></label>
<label class="sc-field">Avoid<small>One per line. Your own words to flag wherever they appear.</small><textarea id="sc-voice-avoid" placeholder="synergy&#10;circle back"></textarea></label>
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
<section>
<h3>Model rewrite</h3>
<label class="sc-field">Provider<small>Adds a Rewrite button that sends the chopped text to a model for the work rules cannot do. Off by default.</small>
<select id="sc-rw-provider">
<option value="" selected>Off</option>
<option value="anthropic">Anthropic API</option>
<option value="openai">OpenAI-compatible (Ollama, LM Studio, vLLM)</option>
</select></label>
<div id="sc-rw-anthropic" hidden>
<label class="sc-field">API key<small>Stays in this browser. Sent only to api.anthropic.com.</small><input type="password" id="sc-rw-key" placeholder="sk-ant-..." autocomplete="off"></label>
<label class="sc-field">Model<input type="text" id="sc-rw-model" placeholder="claude-opus-4-8"></label>
</div>
<div id="sc-rw-openai" hidden>
<label class="sc-field">Base URL<small>Ollama runs at http://localhost:11434 and needs OLLAMA_ORIGINS set to this site's origin.</small><input type="text" id="sc-rw-url" placeholder="http://localhost:11434"></label>
<label class="sc-field">Model<input type="text" id="sc-rw-omodel" placeholder="llama3.3"></label>
<label class="sc-field">API key<small>Optional. Sent as a bearer token to the base URL only.</small><input type="password" id="sc-rw-okey" autocomplete="off"></label>
</div>
<div id="sc-rw-tone-wrap" hidden>
<label class="sc-field">Tone notes<small>One per line. Steers the model's voice. The rules pass ignores them.</small><textarea id="sc-rw-tone" placeholder="dry and direct&#10;no marketing voice"></textarea></label>
<label><input type="checkbox" id="sc-rw-verify" checked> Check the rewrite kept your meaning</label>
</div>
</section>
<div class="sc-drawer-foot">
<button id="sc-reset" type="button">Reset</button>
<button id="sc-export" type="button">Copy profile JSON</button>
<button id="sc-share" type="button" title="Copy a link that opens this page with these settings. API keys stay out of it.">Copy link</button>
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

## "Can't my AI just do this?"

A model has the brains for it. Point one at your draft and it will spot the buzzwords and fix a clumsy line better than any fixed list can. What it will not do is behave the same way twice. It cuts a phrase on this run and keeps it on the next, forgets half your rules by the third paragraph, and nudges your meaning while you look away. Ask the model that wrote the slop to take it back out and you get the same model guessing a second time. Nothing to pin down, nothing to diff.

slop-chop is a fixed list, not a mood. The same text gives the same result every run, with or without a model in the loop. On its own it is a deterministic clean that costs nothing. Paired with your AI it becomes the rails: it runs after the model to catch what drifted, gates a build on the score, and bosses the agent into cleaning its own work to your standard, not its whim. You keep the model's brains and add the bumpers that hold it honest.

## Why slop-chop

<div class="grid cards" markdown>

-   :material-flash:{ .lg .middle } __Deterministic and free__

    ---

    The rules pass runs with no model and no cost, and gives the exact same output on every run. Predictable enough to diff in review and trust in a pipeline. It knows markdown, so code, links, and front matter come through untouched while your prose gets cleaned.

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

<div class="sc-start" markdown>

| Guide                        | What's inside                                                        |
| ---------------------------- | -------------------------------------------------------------------- |
| [Quickstart](quickstart.md)  | Install and clean your first file in a couple of minutes.            |
| [Profiles](PROFILE.md)       | Every field, the presets, the spelling dialects, and the allow list. |
| [Engine](ENGINE.md)          | How the rules pass works under the hood.                             |
| [The tells](TELLS.md)        | The full catalog of what gets chopped or flagged, and why.           |
| [Claude plugin](PLUGIN.md)   | Install, the skill, the command, backends, and troubleshooting.      |

</div>
