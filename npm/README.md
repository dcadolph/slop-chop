# @slop-chop/wasm

Chop AI slop from text, in-process. This is the slop-chop rules engine compiled to
WebAssembly, wrapped for Node. No network, no model, no data leaves the machine, and the same
input always gives the same output.

## Use

```js
const slop = require("@slop-chop/wasm");

const res = await slop.chop("In summary, we leverage a myriad of robust tools.");
res.output;           // "We use many solid tools."
res.score.value;      // 80, the input
res.scoreAfter.value; // 0, the output
res.findings;         // every tell, with rule, match, and offsets
```

Rate without rewriting:

```js
await slop.score("Needless to say, synergy abounds."); // 0 to 100
```

## Options

```js
await slop.chop(text, {
  presets: ["cleaver"],                    // built-in preset names; cleaver is the default
  voice: {                                  // your voice, folded on top
    keep: ["gnarly"],                       // never flag or cut these
    prefer: { utilize: "use" },             // your swap wins
    avoid: ["synergy"],                     // flag these wherever they appear
  },
  profile: undefined,                       // full profile override; defaults to the built-in
});
```

`defaultProfile()` returns the built-in profile to tweak, `presetNames()` lists the packs, and
`version()` reports the engine build. `init()` front-loads the wasm startup, otherwise the
first call pays it.

## Build from source

The engine ships in the package under `engine/`. In the repo, `make npm-package` builds and
stages it.

MIT. From [slop-chop](https://slop-chop.com).
