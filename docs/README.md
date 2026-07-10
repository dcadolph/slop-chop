<div class="sc-hero" markdown>

![slop-chop](assets/icon.png){ .hero-logo }

# slop-chop

<p class="tagline">Chop the slop.</p>

<p class="subtitle">Paste in text and get back something that reads like a person wrote it. A fast, deterministic rules pass pulls the AI tells in one go, with an optional model rewrite for the work rules cannot do.</p>

[Get started](quickstart.md){ .md-button .md-button--primary }
[View on GitHub](https://github.com/dcadolph/slop-chop){ .md-button }

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

- [Quickstart](quickstart.md) — install and clean your first file in a couple of minutes.
- [Profiles](PROFILE.md) — every field, the presets, the spelling dialects, and the allow list.
- [Claude plugin](PLUGIN.md) — install, the skill, the command, backends, and troubleshooting.
