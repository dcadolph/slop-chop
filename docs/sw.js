/* slop-chop service worker: caches the page and the engine so the chopper keeps
   working with no network. Same-origin GETs are served stale-while-revalidate, so a
   deploy reaches a returning visitor one load later. Cross-origin traffic, like the
   model connectors, is never touched. */
"use strict";

const NAME = "slop-chop-shell-v1";

/* CORE is everything the chopper itself needs. The theme's hashed bundles are
   discovered from the built page at install time, since their names change. */
const CORE = [
  "./",
  "index.html",
  "manifest.json",
  "assets/app.js",
  "assets/worker.js",
  "assets/wasm_exec.js",
  "assets/slop-chop.wasm",
  "assets/icon.png",
  "stylesheets/app.css",
  "stylesheets/extra.css",
  "search/search_index.json",
];

/* precacheShell stores the core files, then reads the cached page for the hashed
   stylesheet and script names the theme generated and stores those too. */
async function precacheShell(cache) {
  await cache.addAll(CORE);
  const res = await cache.match("index.html");
  const html = await res.text();
  const hashed = [...html.matchAll(/(?:href|src)="([^"]+\.(?:css|js))"/g)]
    .map((m) => m[1])
    .filter((u) => !u.startsWith("http"));
  await cache.addAll([...new Set(hashed)]);
}

self.addEventListener("install", (e) => {
  e.waitUntil(
    (async () => {
      const cache = await caches.open(NAME);
      await precacheShell(cache);
      await self.skipWaiting();
    })(),
  );
});

self.addEventListener("activate", (e) => {
  e.waitUntil(
    (async () => {
      const names = await caches.keys();
      await Promise.all(names.filter((n) => n !== NAME).map((n) => caches.delete(n)));
      await self.clients.claim();
    })(),
  );
});

/* respond serves from the cache first and refreshes the entry in the background. A
   miss goes to the network, and an offline navigation falls back to the app page. */
async function respond(req) {
  const cache = await caches.open(NAME);
  const cached = await cache.match(req);
  const refresh = fetch(req)
    .then((res) => {
      if (res && res.ok && res.type === "basic") cache.put(req, res.clone());
      return res;
    })
    .catch(() => null);
  if (cached) return cached;
  const res = await refresh;
  if (res) return res;
  if (req.mode === "navigate") {
    const home = (await cache.match("./")) || (await cache.match("index.html"));
    if (home) return home;
  }
  return Response.error();
}

self.addEventListener("fetch", (e) => {
  if (e.request.method !== "GET") return;
  if (new URL(e.request.url).origin !== location.origin) return;
  e.respondWith(respond(e.request));
});
