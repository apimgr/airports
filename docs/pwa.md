# Progressive web app

The Airports web interface can be installed from browsers that support web app installation. It includes a web manifest and service worker; HTTPS is required for service worker support outside localhost.

## Install

When the browser signals that installation is available, the page shows **Install app** in its footer. Choose it to open the browser's native confirmation. Airports does not install automatically. The control disappears after the browser returns its install choice or when the app is already running in standalone mode.

On iPhone or iPad, Safari does not provide the same install prompt. Open the Share menu, choose **Add to Home Screen**, then confirm with **Add**. The footer keeps these steps available without JavaScript.

## Updates

The service worker checks for updates. When a new worker is ready, the page shows an update notice with **Update Now** and **Later**. **Later** dismisses the notice. **Update Now** activates the waiting worker and reloads the page after the browser reports that it has taken control. The page does not silently activate an update while the visitor is using it.

## Offline behavior

The service worker caches static assets and public HTML pages marked `X-Offline-Cache: public`. Public pages use a network-first strategy and can fall back to a cached copy. Pages that are not marked public show the localized offline page when unavailable. API requests are network-only and are never cached; the page displays a connection status while offline and clears it when connectivity returns. Form submissions are not queued for later delivery.

Airport coordinates and the static OpenStreetMap link remain available in server-rendered HTML when JavaScript, the tile service, or network connectivity is unavailable. Interactive map tiles require a network connection.
