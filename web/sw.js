// The service worker: the app works offline after one visit. Every
// request goes to the network first, so a new version shows at once, and
// what came is kept for when there is no network.
const CACHE = "lexington-v1";

self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (e) => e.waitUntil(self.clients.claim()));

self.addEventListener("fetch", (e) => {
	const req = e.request;
	if (req.method !== "GET" || new URL(req.url).origin !== location.origin) return;
	e.respondWith(
		fetch(req)
			.then((res) => {
				if (res.ok) {
					const copy = res.clone();
					e.waitUntil(caches.open(CACHE).then((c) => c.put(req, copy)));
				}
				return res;
			})
			.catch(() => caches.match(req, { ignoreSearch: true }).then((r) => r || Response.error())),
	);
});
