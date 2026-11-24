// Keeps the page itself, never what it shows: requests to the API are left alone.
const CACHE = 'trackferry-1', KEEP = 120;
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (e) => e.waitUntil(caches.keys().then((all) => Promise.all(all.filter((k) => k !== CACHE).map((k) => caches.delete(k)))).then(() => self.clients.claim())));

async function keep(key, res) {
  if (!res.ok || res.redirected) return;
  const c = await caches.open(CACHE);
  await c.put(key, res);
  const old = (await c.keys()).filter((r) => new URL(r.url).pathname.startsWith('/assets/'));
  for (const r of old.slice(0, Math.max(0, old.length - KEEP))) await c.delete(r);
}

self.addEventListener('fetch', (e) => {
  const r = e.request, u = new URL(r.url);
  if (r.method !== 'GET' || u.origin !== location.origin) return;
  if (r.mode === 'navigate' && u.pathname === '/') {
    e.respondWith(fetch(r).then((res) => { e.waitUntil(keep('/', res.clone()).catch(() => {})); return res; }, async (err) => (await caches.match('/')) ?? Promise.reject(err)));
  } else if (u.pathname.startsWith('/assets/')) {
    // Their names change with their content.
    e.respondWith(caches.match(r).then((hit) => hit ?? fetch(r).then((res) => { if (res.status === 200) e.waitUntil(keep(r, res.clone()).catch(() => {})); return res; })));
  }
});
