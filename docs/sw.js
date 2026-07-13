/* slop-chop service worker: caches the page and the engine so the chopper keeps
   working with no network. Pages and stylesheets are network-first so a deploy shows
   up on the next load, with the cache as the offline fallback. Everything else is
   stale-while-revalidate. Cross-origin traffic, like the model connectors, is never
   touched. */
"use strict";

const NAME = "slop-chop-shell-v3";

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

/* CORE_FRESH are the engine's fixed-name assets. Unlike the theme's content-hashed bundles,
   their names do not change between deploys, so a cache-first rule would serve the previous
   build's engine until a background revalidate, leaving the first load after a deploy stale.
   They are answered network-first instead, with the cache kept for offline. */
const CORE_FRESH = ["/assets/app.js", "/assets/worker.js", "/assets/wasm_exec.js", "/assets/slop-chop.wasm"];

/* freshFirst answers pages, stylesheets, and the fixed-name engine assets from the network
   so a deploy is visible on the next load, keeping the cached copy for offline. */
function freshFirst(req) {
  if (req.mode === "navigate") return true;
  const path = new URL(req.url).pathname;
  return path.endsWith(".css") || CORE_FRESH.some((p) => path.endsWith(p));
}

/* respond picks network-first or cache-first per request, refreshes the cache with
   whatever the network returns, and falls back to the cached app page when an
   offline navigation misses. */
async function respond(req) {
  const cache = await caches.open(NAME);
  const cached = await cache.match(req);
  const refresh = fetch(req)
    .then((res) => {
      if (res && res.ok && res.type === "basic") cache.put(req, res.clone());
      return res;
    })
    .catch(() => null);
  if (freshFirst(req)) {
    const res = await refresh;
    if (res) return res;
    if (cached) return cached;
  } else {
    if (cached) return cached;
    const res = await refresh;
    if (res) return res;
  }
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
