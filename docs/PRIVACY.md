# Privacy policy

slop-chop is built so your text stays yours. This policy covers the slop-chop web app, browser
extension, editor integrations, Obsidian plugin, command line tool, and hosted API.

## The short version

slop-chop processes your text on your own device. The rules engine that cleans your writing runs
locally. slop-chop does not collect, store, sell, or transmit your text or any personal data.
There are no accounts and no tracking.

## What runs where

The web app, the browser extension, the Obsidian plugin, the editor plugins, and the command line
tool run the engine entirely on your device. In the browser it is compiled to WebAssembly. On your
machine it is a local binary. Your text never leaves your device.

Settings you choose, such as your keep, prefer, and avoid word lists and the selected preset, are
stored locally on your device, in browser storage or a local file. They are not sent anywhere.

## Optional model rewrite

The tools offer an optional rewrite step that sends your text to an AI model for a deeper edit. It
is off by default. If you turn it on, you supply your own API key, and your text is sent directly
from your device to the provider you pick, such as Anthropic, OpenAI, or a local model server. That
transfer is governed by the chosen provider's privacy policy. slop-chop does not receive, proxy, or
store that text.

## Hosted API

If you use the optional hosted API at api.slop-chop.com, the text you send is processed in memory to
produce the response and is not stored or logged. Nothing is retained after the response returns.

## No tracking

slop-chop does not use analytics, advertising, or third-party trackers to profile you. The website's
built-in search runs in your browser.

## Contact

Questions or concerns: open an issue at
[github.com/dcadolph/slop-chop](https://github.com/dcadolph/slop-chop).

## Changes

If this policy changes, the updated version is posted on this page. Last updated 2026-07-13.
