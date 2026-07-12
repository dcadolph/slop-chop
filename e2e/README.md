# Browser test suite

Drives the built site in real browsers: Chromium for the full flows, Firefox and WebKit
for a core smoke. Model providers are mocked at the network layer, so no keys and no
cost.

Run it locally:

```sh
make wasm
mkdocs build
python3 -m http.server 4173 --bind 127.0.0.1 --directory site &
cd e2e
npm ci
npx playwright install chromium firefox webkit
npm test
```

Point the suite somewhere else with `E2E_BASE_URL`, which defaults to
`http://127.0.0.1:4173/index.html`.

| Suite             | Covers                                                              |
| ----------------- | -------------------------------------------------------------------- |
| `base.e2e.js`     | Boot, chop, drawer, presets, dialect, errors, persistence, big paste.&nbsp; |
| `rewrite.e2e.js`  | Model connectors, meaning check, error recovery, gating.              |
| `features.e2e.js` | Worker responsiveness, share links, fold, output diff, score panel.   |
| `cross.e2e.js`    | The core flows again in Firefox and WebKit.                           |
