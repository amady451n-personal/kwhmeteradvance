const CACHE = 'meteran-v2'
const SHELL = ['/', '/index.html', '/manifest.webmanifest', '/icon.svg']

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(SHELL)))
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
        const cached = (await caches.match(request)) || (await caches.match('/index.html'))
        const refresh = fetch(request)
          .then(async (response) => {
            if (response.ok) {
              const cache = await caches.open(CACHE)
              await cache.put('/index.html', response.clone())
            }
            return response
          })
          .catch(() => null)
        if (cached) {
          event.waitUntil(refresh)
          return cached
        }
        return (await refresh) || Response.error()
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
