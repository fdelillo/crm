const OFFLINE_CACHE = 'crm-offline-v1'
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches
      .open(OFFLINE_CACHE)
      .then((cache) => cache.add('/offline.html'))
      .then(() => self.skipWaiting()),
  )
})
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys
            .filter(
              (key) => key.startsWith('crm-offline-') && key !== OFFLINE_CACHE,
            )
            .map((key) => caches.delete(key)),
        ),
      )
      .then(() => self.clients.claim()),
  )
})
self.addEventListener('fetch', (event) => {
  const path = new URL(event.request.url).pathname
  if (
    event.request.mode !== 'navigate' ||
    path === '/api' ||
    path.startsWith('/api/')
  )
    return
  event.respondWith(
    fetch(event.request).catch(() =>
      caches.open(OFFLINE_CACHE).then((cache) => cache.match('/offline.html')),
    ),
  )
})
