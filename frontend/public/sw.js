/* global self, clients */
// Charon Web Push service worker.
//
// Served at the origin root (`/sw.js`) so `PushManager.subscribe` can
// register it with root scope, per the Web Push API's same-scope
// requirement (a service worker can only receive pushes for paths within
// its own registration scope).
//
// The payload JSON shape matches the notify_yourself render package's
// `minimal`/`detailed` templates (`title`, `message`) — see
// `dispatchWebPushViaNotify` on the backend, which renders through the same
// shared template engine used by every other notification provider.

self.addEventListener('push', (event) => {
  let data = {};
  if (event.data) {
    try {
      data = event.data.json();
    } catch {
      data = { message: event.data.text() };
    }
  }

  const title = data.title || 'Charon';
  const options = {
    body: data.message || data.body || '',
    icon: '/favicon.png',
    badge: '/favicon.png',
    data,
  };

  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();

  event.waitUntil(
    (async () => {
      const allClients = await clients.matchAll({ type: 'window', includeUncontrolled: true });
      for (const client of allClients) {
        if ('focus' in client) {
          return client.focus();
        }
      }
      if (clients.openWindow) {
        return clients.openWindow('/');
      }
      return undefined;
    })()
  );
});
