const CACHE = 'meteran-v5'
const SHELL = ['/', '/index.html', '/manifest.webmanifest', '/icon.svg']
const OFFLINE_HTML = `<!doctype html><html lang="id"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>meteran</title><body style="font-family:system-ui;background:#F6F3EB;color:#15201D;display:grid;place-items:center;min-height:100vh;margin:0"><main style="max-width:320px;text-align:center"><h1 style="color:#0D5C53;font-size:20px;letter-spacing:.14em">KWH</h1><p>Belum bisa memuat aplikasi. Periksa koneksi, lalu coba lagi.</p><button onclick="location.reload()" style="background:#0D5C53;color:#fff;border:0;border-radius:8px;padding:12px 20px;font-size:14px">Coba lagi</button></main></body></html>`

self.addEventListener('install', (event) => {
  // A flaky proxy must not block SW installation: keep whatever we can cache.
  event.waitUntil(
    caches.open(CACHE).then((cache) => Promise.allSettled(SHELL.map((url) => cache.add(url)))),
  )
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((key) => key !== CACHE).map((key) => caches.delete(key))))
      .then(() => self.clients.claim()),
  )
})

self.addEventListener('push', (event) => {
  let payload = {}
  try {
    payload = event.data ? event.data.json() : {}
  } catch {
    payload = {}
  }
  event.waitUntil(
    self.registration.showNotification(payload.title || 'meteran kWh', {
      body: payload.body || 'Waktunya catat meter.',
      icon: '/icon.svg',
      badge: '/icon.svg',
      lang: 'id',
      data: { url: payload.url || '/' },
    }),
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = (event.notification.data && event.notification.data.url) || '/'
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((list) => {
      for (const client of list) if ('focus' in client) return client.focus()
      return self.clients.openWindow(url)
    }),
  )
})

self.addEventListener('fetch', (event) => {
  const { request } = event
  if (request.method !== 'GET') return
  const url = new URL(request.url)
  if (url.origin !== self.location.origin) return
  // API data is cached by the app in encrypted localStorage, never here.
  if (url.pathname.startsWith('/api/')) return

  if (request.mode === 'navigate') {
    event.respondWith(
      (async () => {
        const cached = (await caches.match('/index.html')) || (await caches.match(request))
        const refresh = fetch(request, { cache: 'reload' })
          .then(async (response) => {
            if (!response.ok) return cached || response
            const html = await response.clone().text()
            const assets = [...html.matchAll(/(?:src|href)="(\/assets\/[^"]+)"/g)].map(
              (match) => match[1],
            )
            event.waitUntil(
              (async () => {
                const cache = await caches.open(CACHE)
                await cache.put('/index.html', response.clone())
                await Promise.allSettled(
                  assets.map(async (url) => {
                    if (!(await cache.match(url))) await cache.add(url)
                  }),
                )
              })(),
            )
            return response
          })
          .catch(() => null)
        const quick = await Promise.race([
          refresh,
          new Promise<null>((resolve) => setTimeout(() => resolve(null), 1200)),
        ])
        if (quick) return quick
        if (cached) {
          event.waitUntil(refresh)
          return cached
        }
        return (
          (await refresh) ||
          new Response(OFFLINE_HTML, {
            headers: { 'Content-Type': 'text/html; charset=utf-8' },
          })
        )
      })(),
    )
    return
  }

  event.respondWith(
    caches.match(request).then(
      (hit) =>
        hit ||
        fetch(request).then((response) => {
          if (response.ok) {
            caches.open(CACHE).then((cache) => cache.put(request, response.clone()))
          }
          return response
        }),
    ),
  )
})
