// UI Helper Functions
const API_BASE = document.documentElement.getAttribute('data-api-base');
const BASE_URL = document.documentElement.getAttribute('data-base-url') || '';
const CLIENT_TRANSLATIONS = JSON.parse(document.documentElement.getAttribute('data-client-translations') || '{}');

function translate(key) {
    return CLIENT_TRANSLATIONS[key] || key;
}

function appPath(path) {
    return BASE_URL + path;
}

let hasDirtyForm = false;
let buildReloadScheduled = false;
let serviceWorkerUpdateAccepted = false;
let serviceWorkerReloading = false;
const bootRecoveryKey = 'airports_recovered';
const buildRecoveryKey = 'airports_build_recovered';
let bootRecoveryTimer = setTimeout(function() {
    let recovered = false;
    try { recovered = sessionStorage.getItem(bootRecoveryKey) === '1'; } catch (_) { recovered = false; }
    if (recovered) {
        showBootRecoveryError();
        return;
    }
    try { sessionStorage.setItem(bootRecoveryKey, '1'); } catch (_) {}
    const clearSite = Promise.all([
        navigator.serviceWorker ? navigator.serviceWorker.getRegistrations().then(function(registrations) {
            return Promise.all(registrations.map(function(registration) { return registration.unregister(); }));
        }) : Promise.resolve(),
        window.caches ? caches.keys().then(function(keys) { return Promise.all(keys.map(function(key) { return caches.delete(key); })); }) : Promise.resolve()
    ]);
    let reloadStarted = false;
    const reload = function() {
        if (reloadStarted) return;
        reloadStarted = true;
        window.location.reload();
    };
    clearSite.catch(function() {}).finally(reload);
    setTimeout(reload, 2000);
}, 10000);
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function() { clearTimeout(bootRecoveryTimer); }, {once: true});
} else {
    clearTimeout(bootRecoveryTimer);
}
document.addEventListener('input', function(event) {
    if (event.target && event.target.closest('form')) hasDirtyForm = true;
}, true);
document.addEventListener('change', function(event) {
    if (event.target && event.target.closest('form')) hasDirtyForm = true;
}, true);

function metaText(name, fallback) {
    const element = document.querySelector('meta[name="' + name + '"]');
    return element && element.content ? element.content : fallback;
}

function showBootRecoveryError() {
    const defaults = {
        title: 'Offline',
        heading: 'Unable to load the application',
        message: 'The application could not start. Check your connection and try again.'
    };
    const strings = window.caches
        ? caches.open('airports-offline-strings').then(function(cache) {
            return cache.match('/offline-strings.json').then(function(response) {
                return response ? response.json() : defaults;
            });
        }).catch(function() { return defaults; })
        : Promise.resolve(defaults);
    strings.then(function(value) {
        const panel = document.createElement('main');
        panel.className = 'app-recovery-panel';
        panel.setAttribute('role', 'alert');
        const headingElement = document.createElement('h1');
        headingElement.textContent = value.heading || defaults.heading;
        const messageElement = document.createElement('p');
        messageElement.textContent = value.message || defaults.message;
        const reload = document.createElement('a');
        reload.href = window.location.href;
        reload.textContent = value.reload || 'Reload';
        panel.append(headingElement, messageElement, reload);
        document.title = value.title || defaults.title;
        document.body.replaceChildren(panel);
    });
}

function showConnectionBanner() {
    let banner = document.getElementById('connection-status-banner');
    if (banner) return;
    banner = document.createElement('div');
    banner.id = 'connection-status-banner';
    banner.className = 'app-status-banner';
    banner.setAttribute('role', 'status');
    banner.setAttribute('aria-live', 'polite');
    banner.textContent = metaText('connection-reconnecting', 'Reconnecting to the server…');
    document.body.appendChild(banner);
}

function clearConnectionBanner() {
    document.getElementById('connection-status-banner')?.remove();
}

window.addEventListener('offline', function() {
    let banner = document.getElementById('connection-status-banner');
    if (!banner) {
        banner = document.createElement('div');
        banner.id = 'connection-status-banner';
        banner.className = 'app-status-banner';
        banner.setAttribute('role', 'status');
        banner.setAttribute('aria-live', 'polite');
        document.body.appendChild(banner);
    }
    banner.textContent = metaText('offline-indicator', 'You are offline. Some features may be unavailable.');
});
window.addEventListener('online', clearConnectionBanner);

function showUpdateNotification() {
    if (document.getElementById('app-update-banner')) return;
    const banner = document.createElement('div');
    banner.id = 'app-update-banner';
    banner.className = 'app-status-banner';
    banner.setAttribute('role', 'status');
    const label = document.createElement('span');
    label.textContent = metaText('app-update-available', 'A new version is available');
    const actions = document.createElement('div');
    actions.className = 'app-status-actions';
    const update = document.createElement('button');
    update.type = 'button';
    update.className = 'btn btn-primary';
    update.textContent = metaText('app-update-now', 'Update Now');
    update.addEventListener('click', function() {
        serviceWorkerUpdateAccepted = true;
        applyServiceWorkerUpdate();
    });
    const later = document.createElement('button');
    later.type = 'button';
    later.className = 'btn btn-secondary';
    later.textContent = metaText('app-update-later', 'Later');
    later.addEventListener('click', function() { banner.remove(); });
    actions.append(update, later);
    banner.append(label, actions);
    document.body.appendChild(banner);
}

function reloadOnce() {
    if (serviceWorkerReloading) return;
    serviceWorkerReloading = true;
    window.location.reload();
}

function applyServiceWorkerUpdate() {
    navigator.serviceWorker.ready.then(function(registration) {
        if (registration.waiting) {
            registration.waiting.postMessage({type: 'SKIP_WAITING'});
            setTimeout(reloadOnce, 5000);
        } else {
            reloadOnce();
        }
    }).catch(reloadOnce);
}

function handleBuildStamp(response) {
    const nextStamp = response.headers.get('X-Build');
    const pageStamp = document.documentElement.dataset.buildStamp;
    if (!nextStamp || !pageStamp || nextStamp === pageStamp || buildReloadScheduled) return;
    if (hasDirtyForm) {
        showUpdateNotification();
        return;
    }
    let recovered = false;
    try { recovered = sessionStorage.getItem(buildRecoveryKey) === nextStamp; } catch (_) {}
    if (recovered) return;
    buildReloadScheduled = true;
    try { sessionStorage.setItem(buildRecoveryKey, nextStamp); } catch (_) {}
    setTimeout(function() { window.location.reload(); }, 250);
}

const nativeFetch = window.fetch.bind(window);
window.fetch = async function(input, init) {
    init = init || {};
    const method = String(init.method || (input instanceof Request ? input.method : 'GET')).toUpperCase();
    const safeToRetry = method === 'GET' || method === 'HEAD';
    for (let attempt = 0; ; attempt++) {
        const controller = new AbortController();
        const timeout = setTimeout(function() { controller.abort(); }, 30000);
        const parentSignal = init.signal || (input instanceof Request ? input.signal : null);
        const abortParent = function() { controller.abort(); };
        if (parentSignal) {
            if (parentSignal.aborted) controller.abort();
            else parentSignal.addEventListener('abort', abortParent, {once: true});
        }
        let response;
        try {
            response = await nativeFetch(input, Object.assign({}, init, {signal: controller.signal}));
        } catch (error) {
            showConnectionBanner();
            throw error;
        } finally {
            clearTimeout(timeout);
            if (parentSignal) parentSignal.removeEventListener('abort', abortParent);
        }
        if (response.status === 503) {
            showConnectionBanner();
            if (safeToRetry && attempt < 5) {
                const retryAfter = response.headers.get('Retry-After');
                const seconds = Number(retryAfter);
                const dateDelay = Number.isFinite(seconds) ? seconds * 1000 : Math.max(0, Date.parse(retryAfter || '') - Date.now());
                const backoff = Math.min(30000, 1000 * Math.pow(2, attempt));
                const delay = Math.min(30000, dateDelay || backoff) + Math.floor(Math.random() * 250);
                await new Promise(function(resolve) { setTimeout(resolve, delay); });
                continue;
            }
        } else if (response.ok) {
            clearConnectionBanner();
        }
        handleBuildStamp(response);
        return response;
    }
};

// escapeHtml escapes a string for safe interpolation into an innerHTML
// template literal. Used wherever user-supplied input (as opposed to
// hardcoded markup) is inserted via innerHTML instead of textContent.
function escapeHtml(value) {
    const div = document.createElement('div');
    div.textContent = String(value);
    return div.innerHTML;
}

function toggleMobileMenu() {
    const checkbox = document.getElementById('nav-toggle');
    if (checkbox) checkbox.checked = !checkbox.checked;
}

function initNavigation() {
    const checkbox = document.getElementById('nav-toggle');
    const panel = document.getElementById('nav-panel');
    const toggle = document.querySelector('.nav-toggle');
    const desktopNav = document.getElementById('main-navigation');
    if (!checkbox || !panel || !toggle || !desktopNav) return;

    function updateMenuState(focusPanel) {
        const open = checkbox.checked;
        panel.inert = !open;
        toggle.setAttribute('aria-expanded', String(open));
        if (focusPanel && open) panel.querySelector('a')?.focus();
    }

    function updateResponsiveMenu() {
        const collapsed = window.innerWidth >= 768 && desktopNav.scrollWidth > desktopNav.clientWidth;
        document.documentElement.classList.toggle('nav-collapsed', collapsed);
        if (!collapsed && window.innerWidth >= 768) checkbox.checked = false;
        updateMenuState(false);
    }

    panel.inert = true;
    toggle.setAttribute('aria-expanded', 'false');
    checkbox.addEventListener('change', function() { updateMenuState(true); });
    window.addEventListener('resize', updateResponsiveMenu);
    document.addEventListener('keydown', function(event) {
        if (!checkbox.checked) return;
        if (event.key === 'Escape') {
            checkbox.checked = false;
            updateMenuState(false);
            toggle.focus();
            return;
        }
        if (event.key !== 'Tab') return;
        const focusable = Array.from(panel.querySelectorAll('a[href], button:not([disabled]), [tabindex="0"]'));
        if (focusable.length === 0) return;
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        if (event.shiftKey && document.activeElement === first) {
            event.preventDefault();
            last.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault();
            first.focus();
        }
    });
    updateResponsiveMenu();
}

initNavigation();

// Toast Notifications (AI.md PART 16 "Toast/Notification Requirements": max 5
// stacked, success/info 3s, warning 5s, error never auto-dismisses, click or
// X to dismiss, pause-on-hover, progress bar, slide-in/fade-out).
const TOAST_DURATIONS = { success: 3000, info: 3000, warning: 5000, error: 0 };
const TOAST_ICONS = { success: '✓', error: '✗', warning: '⚠', info: 'ℹ' };
const TOAST_MAX_VISIBLE = 5;

function showToast(message, type) {
    type = type || 'info';
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = 'toast toast-' + type;
    toast.setAttribute('role', 'alert');
    toast.setAttribute('aria-live', 'polite');

    const icon = document.createElement('span');
    icon.className = 'toast-icon';
    icon.textContent = TOAST_ICONS[type] || TOAST_ICONS.info;

    const msg = document.createElement('span');
    msg.className = 'toast-message';
    msg.textContent = message;

    const close = document.createElement('button');
    close.className = 'toast-close';
    close.setAttribute('aria-label', translate('ui.toast_dismiss'));
    close.innerHTML = '&times;';

    toast.append(icon, msg, close);

    function dismiss() {
        const isRtl = document.documentElement.getAttribute('dir') === 'rtl';
        toast.style.animation = (isRtl ? 'slideOutRTL' : 'slideOut') + ' 0.3s ease forwards';
        setTimeout(function() { toast.remove(); }, 300);
    }

    // Click anywhere on the toast (or the X) dismisses it early.
    toast.addEventListener('click', dismiss);

    const duration = TOAST_DURATIONS[type] ?? TOAST_DURATIONS.info;
    let remaining = duration;
    let timerId = null;
    let startedAt = 0;
    let progress = null;

    function startTimer() {
        startedAt = Date.now();
        if (progress) {
            progress.style.animationPlayState = 'running';
        }
        timerId = setTimeout(dismiss, remaining);
    }

    function pauseTimer() {
        if (timerId === null) {
            return;
        }
        clearTimeout(timerId);
        timerId = null;
        remaining -= Date.now() - startedAt;
        if (progress) {
            progress.style.animationPlayState = 'paused';
        }
    }

    // duration 0 = no auto-dismiss (errors stay until clicked).
    if (duration > 0) {
        progress = document.createElement('div');
        progress.className = 'toast-progress';
        progress.style.animationDuration = duration + 'ms';
        toast.appendChild(progress);

        toast.addEventListener('mouseenter', pauseTimer);
        toast.addEventListener('mouseleave', startTimer);
        startTimer();
    }

    // Newest on top per the stacking spec; cap at max visible toasts.
    container.prepend(toast);
    while (container.children.length > TOAST_MAX_VISIBLE) {
        container.lastElementChild.remove();
    }
}

document.addEventListener('DOMContentLoaded', function() {
    const notice = document.getElementById('owner-token-notice');
    if (!notice) return;
    showToast(notice.textContent.trim(), 'success');
    notice.hidden = true;
});

// Modal Functions
// Uses the native <dialog> element (AI.md PART 16 "Modals"): showModal()
// provides the focus trap and ::backdrop automatically, and the close
// button's <form method="dialog"> closes it with zero JS. The "close" event
// (fired on both the close button and native Escape-key dismissal) tears
// down the container so a fresh dialog is created on next open.
function showModal(title, content) {
    const container = document.getElementById('modal-container');

    container.innerHTML =
        '<dialog id="app-modal" class="modal" aria-labelledby="app-modal-title">' +
        '<div class="modal-header">' +
        '<h2 id="app-modal-title">' + escapeHtml(title) + '</h2>' +
        '<form method="dialog"><button class="modal-close" aria-label="' + escapeHtml(translate('ui.close')) + '">×</button></form>' +
        '</div>' +
        '<div class="modal-body">' + content + '</div>' +
        '</dialog>';

    const dialog = document.getElementById('app-modal');
    dialog.addEventListener('close', function() { container.innerHTML = ''; });
    dialog.showModal();
}

function closeModal() {
    const dialog = document.getElementById('app-modal');
    if (dialog) {
        dialog.close();
    }
}

// ── Site banner ──────────────────────────────────────────────────────────────
// The dismiss form works as a plain POST without JavaScript (server appends
// the id to the dismissed_announcements cookie and redirects back). When JS
// is available, this intercepts the submit to skip the reload — the cookie
// stays server-readable so dismissed announcements are never rendered again.
// Dismissal is keyed on the announcement id; changing the id resets dismissals.
function dismissAnnouncement(form) {
    const banner = form.closest('.site-banner');
    if (!banner) return;
    const id = banner.getAttribute('data-announcement-id');
    const match = document.cookie.match(/(?:^|;\s*)dismissed_announcements=([^;]*)/);
    const ids = match ? decodeURIComponent(match[1]).split(',').filter(Boolean) : [];
    if (!ids.includes(id)) {
        ids.push(id);
    }
    document.cookie = 'dismissed_announcements=' + encodeURIComponent(ids.join(',')) +
        '; path=/; max-age=31536000; SameSite=Lax';
    banner.remove();
}

// API Helper Functions
function apiGet(endpoint) {
    return fetch(endpoint)
        .then(function(response) { return response.json(); })
        .then(function(data) {
            if (data.ok === false) {
                throw new Error((data.error && data.error.message) || data.message || translate('ui.api_request_failed'));
            }
            return data.data !== undefined ? data.data : data;
        })
        .catch(function(error) {
            showToast(error instanceof TypeError ? translate('ui.api_request_failed') : error.message, 'error');
            throw error;
        });
}

// Utility Functions
function formatDistance(km, units) {
    units = units || 'imperial';
    if (units === 'metric') {
        return km.toFixed(2) + ' km';
    }
    const miles = km * 0.621371;
    return miles.toFixed(2) + ' mi';
}

function formatElevation(feet, units) {
    units = units || 'imperial';
    if (units === 'metric') {
        const meters = feet * 0.3048;
        return meters.toFixed(0) + ' m';
    }
    return feet + ' ft';
}

function formatCoordinates(lat, lon) {
    const latDir = lat >= 0 ? 'N' : 'S';
    const lonDir = lon >= 0 ? 'E' : 'W';
    return Math.abs(lat).toFixed(4) + '°' + latDir + ', ' + Math.abs(lon).toFixed(4) + '°' + lonDir;
}

// Service worker registration
if ('caches' in window) {
    const metaValue = function(name) {
        const element = document.querySelector('meta[name="' + name + '"]');
        return element ? element.content : '';
    };
    const offlineStrings = {
        lang: document.documentElement.lang || 'en',
        dir: document.documentElement.dir || 'ltr',
        title: metaValue('offline-title'),
        heading: metaValue('offline-heading'),
        message: metaValue('offline-message'),
        reload: metaValue('app-update-now')
    };
    caches.open('airports-offline-strings').then(function(cache) {
        return cache.put('/offline-strings.json', new Response(JSON.stringify(offlineStrings), {
            headers: {'Content-Type': 'application/json'}
        }));
    }).catch(function() {});
}

if ('serviceWorker' in navigator) {
    navigator.serviceWorker.addEventListener('controllerchange', function() {
        if (serviceWorkerUpdateAccepted) reloadOnce();
    });
    window.addEventListener('load', function() {
        navigator.serviceWorker.register(appPath('/sw.js')).then(function(registration) {
            setInterval(function() { registration.update().catch(function() {}); }, 60 * 60 * 1000);
            registration.addEventListener('updatefound', function() {
                const worker = registration.installing;
                if (!worker) return;
                worker.addEventListener('statechange', function() {
                    if (worker.state === 'installed' && navigator.serviceWorker.controller) showUpdateNotification();
                });
            });
            if (registration.waiting && navigator.serviceWorker.controller) showUpdateNotification();
        }).catch(function() {});
    });
}

// ── Home page ────────────────────────────────────────────────────────────────
var searchTimeout;

function quickSearch(event) {
    clearTimeout(searchTimeout);
    const query = event.target.value.trim();

    if (query.length < 2) {
        document.getElementById('quick-results').innerHTML = '';
        return;
    }

    searchTimeout = setTimeout(function() {
        fetch(API_BASE + '/airports/search?q=' + encodeURIComponent(query) + '&limit=5')
            .then(function(r) { return r.json(); })
            .then(function(data) {
                const list = (data.data && Array.isArray(data.data)) ? data.data : [];
                if (list.length > 0) {
                    const html = list.map(function(a) {
                        return '<div class="quick-result-item" data-action="view-airport" data-code="' + a.icao + '">' +
                            '<strong>' + a.icao + (a.iata ? ' / ' + a.iata : '') + '</strong>' +
                            '<span>' + a.name + '</span>' +
                            '<span class="result-location">' + a.city + ', ' + a.country + '</span>' +
                            '</div>';
                    }).join('');
                    document.getElementById('quick-results').innerHTML = html;
                } else {
                    document.getElementById('quick-results').innerHTML =
                        '<div class="no-results">' + escapeHtml(translate('ui.search_no_results')) + '</div>';
                }
            });
    }, 300);
}

function detectLocation() {
    fetch(API_BASE + '/geoip/airports/nearby')
        .then(function(r) { return r.json(); })
        .then(function(data) {
            if (data.ok !== false && data.data) {
                const loc = data.data.location;
                const nearbyHref = appPath('/airports/nearby?lat=' + loc.latitude + '&lon=' + loc.longitude);
                showModal(translate('geoip.your_location'),
                    '<p><strong>' + escapeHtml(translate('geoip.ip_label')) + ':</strong> ' + escapeHtml(loc.ip) + '</p>' +
                    '<p><strong>' + escapeHtml(translate('ui.location')) + ':</strong> ' + escapeHtml(loc.city || translate('ui.unknown')) + ', ' + escapeHtml(loc.country_name || loc.country) + '</p>' +
                    '<p><strong>' + escapeHtml(translate('geoip.latitude_label')) + ' / ' + escapeHtml(translate('geoip.longitude_label')) + ':</strong> ' + escapeHtml(loc.latitude.toFixed(4)) + ', ' + escapeHtml(loc.longitude.toFixed(4)) + '</p>' +
                    '<p><strong>' + escapeHtml(translate('geoip.nearby_heading')) + ':</strong> ' + escapeHtml(data.data.nearby_airports.length) + '</p>' +
                    '<button data-action="navigate" data-href="' + escapeHtml(nearbyHref) + '">' + escapeHtml(translate('ui.view_nearby_airports')) + '</button>'
                );
            }
        });
}

// ── Search page ──────────────────────────────────────────────────────────────
// Enter-to-submit is handled natively by the <form method="get"> wrapper
// (AI.md PART 16) — no keyup listener needed.
function performSearch() {
    const input = document.getElementById('search-input') || document.getElementById('quick-search');
    if (!input) return;
    const query = input.value.trim();
    if (query) {
        window.location = appPath('/airports/search?q=' + encodeURIComponent(query));
    }
}

// ── Airport detail page ──────────────────────────────────────────────────────
// Coordinates are read from data-lat / data-lon attributes on .airport-detail
// to avoid inline script template variables.
function findNearby() {
    const el = document.querySelector('[data-lat][data-lon]');
    if (!el) return;
    const lat = el.getAttribute('data-lat');
    const lon = el.getAttribute('data-lon');
    window.location = appPath('/airports/nearby?lat=' + lat + '&lon=' + lon + '&radius=100');
}

// "View on Map" is now a plain <a href target="_blank"> in airport.html
// (AI.md PART 16 no-JS map fallback) so no JS is needed to open the map.

function copyJSON() {
    const jsonEl = document.getElementById('json-data');
    if (!jsonEl) return;
    navigator.clipboard.writeText(jsonEl.textContent).then(function() {
        showToast(translate('ui.copy_json_success'), 'success');
    });
}

// copyValue copies the element's data-copy attribute to the clipboard and
// swaps the trigger button's label to a translated "Copied!" state for
// visual feedback, using the active locale if the template omits its label.
function copyValue(target) {
    const value = target.getAttribute('data-copy');
    if (!value) return;
    navigator.clipboard.writeText(value).then(function() {
        const copiedText = target.getAttribute('data-copied-text') || translate('common.copied');
        const original = target.textContent;
        target.textContent = copiedText;
        target.disabled = true;
        setTimeout(function() {
            target.textContent = original;
            target.disabled = false;
        }, 2000);
    });
}

// ── Stats page ───────────────────────────────────────────────────────────────
function searchCountry(country) {
    window.location = appPath('/airports/search?q=' + encodeURIComponent(country));
}

// ── Nearby page ───────────────────────────────────────────────────────────────
function findNearbyAirports() {
    const lat    = document.getElementById('latitude').value;
    const lon    = document.getElementById('longitude').value;
    const radius = document.getElementById('radius').value;
    const units  = document.getElementById('units') && document.getElementById('units').value;
    const limit  = document.getElementById('limit') && document.getElementById('limit').value;

    if (!lat || !lon) {
        showToast(translate('ui.coordinates_required'), 'error');
        return;
    }

    let url = appPath('/airports/nearby?lat=' + lat + '&lon=' + lon + '&radius=' + radius);
    if (units) url += '&units=' + units;
    if (limit) url += '&limit=' + limit;
    window.location = url;
}

function useMyLocation() {
    if (!navigator.geolocation) {
        showToast(translate('ui.geolocation_unsupported'), 'error');
        return;
    }

    showToast(translate('ui.geolocation_loading'), 'info');

    navigator.geolocation.getCurrentPosition(
        function(position) {
            document.getElementById('latitude').value = position.coords.latitude.toFixed(4);
            document.getElementById('longitude').value = position.coords.longitude.toFixed(4);
            showToast(translate('ui.geolocation_detected'), 'success');
        },
        function(error) {
            showToast(translate('ui.geolocation_failed'), 'error');
        }
    );
}

function viewAirport(code) {
    window.location = appPath('/airports/' + code);
}

// ── GeoIP page ────────────────────────────────────────────────────────────────
function lookupIP(event) {
    event.preventDefault();
    const ip = document.getElementById('ip-input').value.trim();
    if (!ip) return;

    const result = document.getElementById('geoip-result');
    const errBox = document.getElementById('geoip-error');
    result.classList.add('hidden');
    errBox.classList.add('hidden');

    fetch(API_BASE + '/geoip/' + encodeURIComponent(ip))
        .then(function(r) { return r.json(); })
        .then(function(data) {
            if (data.ok === false) {
                document.getElementById('error-message').textContent = data.message || translate('ui.lookup_failed');
                errBox.classList.remove('hidden');
                return;
            }
            const loc = document.getElementById('location-data');
            loc.innerHTML =
                '<dl>' +
                '<dt>' + escapeHtml(translate('geoip.ip_label')) + '</dt><dd>' + escapeHtml(data.ip || ip) + '</dd>' +
                '<dt>' + escapeHtml(translate('geoip.country_label')) + '</dt><dd>' + escapeHtml(data.country_name || '-') + ' (' + escapeHtml(data.country_code || '-') + ')</dd>' +
                '<dt>' + escapeHtml(translate('geoip.city_label')) + '</dt><dd>' + escapeHtml(data.city || '-') + '</dd>' +
                '<dt>' + escapeHtml(translate('geoip.latitude_label')) + '</dt><dd>' + escapeHtml(data.lat || '-') + '</dd>' +
                '<dt>' + escapeHtml(translate('geoip.longitude_label')) + '</dt><dd>' + escapeHtml(data.lon || '-') + '</dd>' +
                '<dt>' + escapeHtml(translate('geoip.asn_label')) + '</dt><dd>' + escapeHtml(data.asn || '-') + '</dd>' +
                '<dt>' + escapeHtml(translate('geoip.isp_label')) + '</dt><dd>' + escapeHtml(data.isp || '-') + '</dd>' +
                '</dl>';
            result.classList.remove('hidden');

            if (data.lat && data.lon) {
                fetch(API_BASE + '/geoip/airports/nearby?ip=' + encodeURIComponent(ip))
                    .then(function(r) { return r.json(); })
                    .then(function(nearby) {
                        const airports = nearby.data || [];
                        const nearbyEl = document.getElementById('nearby-data');
                        if (!airports.length) {
                            nearbyEl.textContent = translate('ui.no_airports_nearby');
                            return;
                        }
                        nearbyEl.innerHTML = airports.slice(0, 10).map(function(a) {
                            return '<div class="airport-item">' +
                                '<strong>' + a.name + '</strong>' +
                                '<span class="airport-codes">' + (a.icao || '') + (a.iata ? ' / ' + a.iata : '') + '</span>' +
                                '<span class="airport-dist">' + (a.distance_km ? a.distance_km.toFixed(1) + ' km' : '') + '</span>' +
                                '</div>';
                        }).join('');
                    })
                    .catch(function() {
                        document.getElementById('nearby-data').textContent = translate('ui.nearby_load_failed');
                    });
            }
        })
        .catch(function(err) {
            document.getElementById('error-message').textContent = translate('ui.request_failed');
            errBox.classList.remove('hidden');
        });
}

// ── Centralized event delegation ────────────────────────────────────────────
// CSP script-src 'self' forbids inline event handler attributes, so every
// interactive element is bound via a single data-action attribute and
// dispatched here instead of scattering per-element listeners.
function dispatchAction(action, target) {
    switch (action) {
        case 'toggle-mobile-menu':
            toggleMobileMenu();
            break;
        case 'perform-search':
            performSearch();
            break;
        case 'navigate':
            const href = target.getAttribute('data-href');
            if (href) window.location = href;
            break;
        case 'detect-location':
            detectLocation();
            break;
        case 'view-airport':
            const code = target.getAttribute('data-code');
            if (code) viewAirport(code);
            break;
        case 'find-nearby':
            findNearby();
            break;
        case 'copy-json':
            copyJSON();
            break;
        case 'copy-value':
            copyValue(target);
            break;
        case 'search-country':
            const country = target.getAttribute('data-country');
            if (country) searchCountry(country);
            break;
        case 'use-my-location':
            useMyLocation();
            break;
        case 'close-modal':
            closeModal();
            break;
        case 'cookie-preferences':
            showCookiePreferences();
            break;
        default:
            break;
    }
}

function showCookiePreferences() {
    const dialog = document.getElementById('cookie-preferences-modal');
    if (dialog && typeof dialog.showModal === 'function') dialog.showModal();
}

document.addEventListener('click', function(e) {
    const actionTarget = e.target.closest('[data-action]');
    if (actionTarget) {
        dispatchAction(actionTarget.getAttribute('data-action'), actionTarget);
    }

    // Close mobile menu when clicking outside it.
    const nav = document.getElementById('main-nav');
    const toggle = document.querySelector('.mobile-menu-toggle');
    if (nav && toggle && !nav.contains(e.target) && !toggle.contains(e.target)) {
        nav.classList.remove('active');
    }
});

document.addEventListener('keyup', function(e) {
    const target = e.target.closest('[data-action]');
    if (!target) return;
    const action = target.getAttribute('data-action');
    if (action === 'quick-search') {
        quickSearch(e);
    }
});

document.addEventListener('change', function(e) {
    const target = e.target.closest('[data-action]');
    if (!target) return;
    if (target.getAttribute('data-action') === 'limit-change') {
        performSearch();
    } else if (target.getAttribute('data-action') === 'submit-lang-form') {
        target.form.submit();
    }
});

document.addEventListener('submit', function(e) {
    const target = e.target.closest('[data-action]');
    if (!target) return;
    const action = target.getAttribute('data-action');
    // Every form below works as a plain GET without JavaScript (AI.md
    // PART 16 "No JavaScript-Disabled Broken State"). When JS is available,
    // preventDefault() takes over the submission via AJAX/client-side
    // navigation so there is no double-submission.
    if (action === 'lookup-ip') {
        lookupIP(e);
    } else if (action === 'submit-search') {
        e.preventDefault();
        performSearch();
    } else if (action === 'submit-nearby') {
        e.preventDefault();
        findNearbyAirports();
    } else if (action === 'dismiss-announcement') {
        e.preventDefault();
        dismissAnnouncement(target);
    }
});


// Client-side OpenStreetMap tile enhancement. Server-rendered coordinates and
// external map links remain the complete fallback when JavaScript or tiles fail.
function initAirportMaps() {
    document.querySelectorAll('.airport-map').forEach(function(section) {
        const canvasHost = section.querySelector('.map-canvas');
        let points;
        try { points = JSON.parse(section.getAttribute('data-map-points') || '[]'); } catch (_) { return; }
        points = points.filter(function(p) { return Number.isFinite(Number(p.lat)) && Number.isFinite(Number(p.lon)); });
        if (!canvasHost || points.length === 0) return;
        const canvas = document.createElement('canvas');
        const ctx = canvas.getContext('2d');
        const markerLayer = document.createElement('div');
        markerLayer.className = 'map-marker-layer';
        canvasHost.append(canvas, markerLayer);
        section.hidden = false;
        const tileCache = new Map();
        const zoomMin = 2, zoomMax = 17;
        let zoom = points.length === 1 ? 10 : 12;
        let center = { x: 0, y: 0 };
        let dragging = null;
        let failedTiles = 0, loadedTiles = 0, mapFailed = false;

        function project(lat, lon, z) {
            const scale = 256 * Math.pow(2, z);
            const bounded = Math.max(-85.05112878, Math.min(85.05112878, lat));
            return { x: (lon + 180) / 360 * scale, y: (1 - Math.log(Math.tan(Math.PI / 4 + bounded * Math.PI / 360)) / Math.PI) / 2 * scale };
        }
        function unproject(x, y, z) {
            const scale = 256 * Math.pow(2, z);
            return { lon: x / scale * 360 - 180, lat: Math.atan(Math.sinh(Math.PI * (1 - 2 * y / scale))) * 180 / Math.PI };
        }
        function sizeCanvas() {
            const rect = canvasHost.getBoundingClientRect();
            const ratio = window.devicePixelRatio || 1;
            canvas.width = Math.max(1, Math.round(rect.width * ratio));
            canvas.height = Math.max(1, Math.round(rect.height * ratio));
            draw();
        }
        function getTile(x, y) {
            const n = Math.pow(2, zoom);
            x = ((x % n) + n) % n;
            if (y < 0 || y >= n) return null;
            const key = zoom + '/' + x + '/' + y;
            if (!tileCache.has(key)) {
                const img = new Image();
                tileCache.set(key, img);
                img.onload = function() { loadedTiles++; draw(); };
                img.onerror = function() { failedTiles++; if (loadedTiles === 0 && failedTiles >= Math.min(4, tileCache.size)) showMapError(); };
                img.src = 'https://tile.openstreetmap.org/' + key + '.png';
            }
            return tileCache.get(key);
        }
        function positionMarkers() {
            markerLayer.replaceChildren();
            const width = canvasHost.clientWidth, height = canvasHost.clientHeight;
            points.forEach(function(point) {
                const pos = project(Number(point.lat), Number(point.lon), zoom);
                const button = document.createElement('button');
                button.type = 'button'; button.className = 'map-marker';
                button.textContent = String(point.code || '•');
                button.setAttribute('aria-label', String(point.name || point.code || translate('ui.airport_label')));
                const screenX = pos.x - center.x + width / 2, screenY = pos.y - center.y + height / 2;
                button.style.left = screenX + 'px'; button.style.top = screenY + 'px';
                button.addEventListener('click', function() {
                    if (point.code) window.location.href = appPath('/airports/' + encodeURIComponent(point.code));
                });
                markerLayer.appendChild(button);
            });
        }
        function draw() {
            if (!ctx || !canvas.width || !canvas.height) return;
            const ratio = window.devicePixelRatio || 1;
            const width = canvas.width / ratio, height = canvas.height / ratio;
            ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
            ctx.fillStyle = '#e8edf0'; ctx.fillRect(0, 0, width, height);
            const left = center.x - width / 2, top = center.y - height / 2;
            const x0 = Math.floor(left / 256), y0 = Math.floor(top / 256);
            const x1 = Math.floor((left + width) / 256), y1 = Math.floor((top + height) / 256);
            for (let ty = y0; ty <= y1; ty++) {
                for (let tx = x0; tx <= x1; tx++) {
                    const img = getTile(tx, ty);
                    if (img && img.complete && img.naturalWidth) {
                        ctx.drawImage(img, tx * 256 - left, ty * 256 - top, 256, 256);
                    }
                }
            }
            positionMarkers();
        }
        function showMapError() {
            if (mapFailed) return;
            mapFailed = true;
            section.hidden = true;
            const fallback = document.createElement('p');
            fallback.className = 'map-error';
            fallback.textContent = section.getAttribute('data-map-error') || translate('airport.map_unavailable');
            section.parentNode.insertBefore(fallback, section.nextSibling);
        }
        function setZoom(next, anchorX, anchorY) {
            next = Math.max(zoomMin, Math.min(zoomMax, next));
            if (next === zoom) return;
            const width = canvasHost.clientWidth, height = canvasHost.clientHeight;
            const ax = anchorX === undefined ? width / 2 : anchorX, ay = anchorY === undefined ? height / 2 : anchorY;
            const geographic = unproject(center.x + ax - width / 2, center.y + ay - height / 2, zoom);
            zoom = next;
            const projected = project(geographic.lat, geographic.lon, zoom);
            center = { x: projected.x - (ax - width / 2), y: projected.y - (ay - height / 2) };
            draw();
        }
        function getExtent(level) {
            return points.reduce(function(acc, p) {
                const pos = project(Number(p.lat), Number(p.lon), level);
                acc.minX = Math.min(acc.minX, pos.x); acc.maxX = Math.max(acc.maxX, pos.x);
                acc.minY = Math.min(acc.minY, pos.y); acc.maxY = Math.max(acc.maxY, pos.y); return acc;
            }, { minX: Infinity, maxX: -Infinity, minY: Infinity, maxY: -Infinity });
        }
        const availableWidth = Math.max(1, canvasHost.clientWidth * 0.75);
        const availableHeight = Math.max(1, canvasHost.clientHeight * 0.75);
        let extent = getExtent(zoom);
        while (points.length > 1 && zoom > zoomMin &&
            (extent.maxX - extent.minX > availableWidth || extent.maxY - extent.minY > availableHeight)) {
            zoom--; extent = getExtent(zoom);
        }
        center = { x: (extent.minX + extent.maxX) / 2, y: (extent.minY + extent.maxY) / 2 };
        const zoomButtons = section.querySelectorAll('[data-map-zoom]');
        zoomButtons.forEach(function(button) { button.addEventListener('click', function() { setZoom(zoom + Number(button.getAttribute('data-map-zoom'))); }); });
        canvasHost.addEventListener('wheel', function(event) { event.preventDefault(); const rect = canvasHost.getBoundingClientRect(); setZoom(zoom + (event.deltaY < 0 ? 1 : -1), event.clientX - rect.left, event.clientY - rect.top); }, { passive: false });
        canvasHost.addEventListener('pointerdown', function(event) { dragging = { x: event.clientX, y: event.clientY, centerX: center.x, centerY: center.y }; canvasHost.setPointerCapture(event.pointerId); });
        canvasHost.addEventListener('pointermove', function(event) { if (!dragging) return; center.x = dragging.centerX - (event.clientX - dragging.x); center.y = dragging.centerY - (event.clientY - dragging.y); draw(); });
        canvasHost.addEventListener('pointerup', function() { dragging = null; });
        if ('ResizeObserver' in window) new ResizeObserver(sizeCanvas).observe(canvasHost); else window.addEventListener('resize', sizeCanvas);
        sizeCanvas();
    });
}

document.addEventListener('DOMContentLoaded', initAirportMaps);

(function () {
    const loader = document.currentScript;
    if (!loader) return;

    const provider = loader.dataset.provider;
    if (!provider) return;
    const id = loader.dataset.id || '';
    const baseURL = (loader.dataset.url || '').replace(/\/+$/, '');

    function addScript(src, attributes) {
        const script = document.createElement('script');
        script.async = true;
        script.src = src;
        Object.entries(attributes || {}).forEach(function (entry) {
            script.setAttribute(entry[0], entry[1]);
        });
        document.head.appendChild(script);
    }

    switch (provider) {
    case 'google':
        if (id.startsWith('G-')) {
            window.dataLayer = window.dataLayer || [];
            window.gtag = window.gtag || function () { window.dataLayer.push(arguments); };
            window.gtag('js', new Date());
            window.gtag('config', id);
            addScript('https://www.googletagmanager.com/gtag/js?id=' + encodeURIComponent(id));
        } else {
            window.ga = window.ga || function () { (window.ga.q = window.ga.q || []).push(arguments); };
            window.ga.l = Date.now();
            window.ga('create', id, 'auto');
            window.ga('send', 'pageview');
            addScript('https://www.google-analytics.com/analytics.js');
        }
        break;
    case 'matomo':
    case 'piwik':
        window._paq = window._paq || [];
        window._paq.push(['trackPageView']);
        window._paq.push(['enableLinkTracking']);
        window._paq.push(['setTrackerUrl', baseURL + '/matomo.php']);
        window._paq.push(['setSiteId', id]);
        addScript(baseURL + '/matomo.js');
        break;
    case 'owa':
        window.owa_cmds = window.owa_cmds || [];
        window.owa_cmds.push(['setSiteId', id]);
        window.owa_cmds.push(['trackPageView']);
        addScript(baseURL + '/modules/base/js/owa.tracker-combined-min.js');
        break;
    case 'fathom':
        addScript(baseURL ? baseURL + '/tracker.js' : 'https://cdn.usefathom.com/script.js', { 'data-site': id });
        break;
    case 'plausible':
        addScript(baseURL ? baseURL + '/js/script.js' : 'https://plausible.io/js/script.js', { 'data-domain': id });
        break;
    case 'umami':
        addScript(baseURL + '/script.js', { 'data-website-id': id });
        break;
    case 'simple':
        addScript('https://scripts.simpleanalyticscdn.com/latest.js');
        break;
    case 'cloudflare':
        addScript('https://static.cloudflareinsights.com/beacon.min.js', {
            'data-cf-beacon': JSON.stringify({ token: id })
        });
        break;
    }
}());

document.addEventListener('DOMContentLoaded', function() {
    const swaggerEl = document.getElementById('swagger-ui');
    if (!swaggerEl || typeof SwaggerUIBundle === 'undefined') return;
    window.ui = SwaggerUIBundle({
        url: swaggerEl.getAttribute('data-spec-url'),
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
            SwaggerUIBundle.presets.apis,
            SwaggerUIStandalonePreset
        ],
        plugins: [
            SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: 'StandaloneLayout'
    });
});

document.addEventListener('DOMContentLoaded', function() {
    const graphiqlEl = document.getElementById('graphiql');
    if (!graphiqlEl || typeof GraphiQL === 'undefined') return;
    const fetcher = GraphiQL.createFetcher({ url: graphiqlEl.getAttribute('data-endpoint-url') });
    const defaultQuery = '# ' + translate('graphql.editor_welcome') + '\n# ' + translate('graphql.editor_run_hint') + '\n\n' +
        'query GetAirport {\n  airport(code: "KJFK") {\n    icao\n    iata\n    name\n    city\n    country\n    coordinates { lat lon }\n  }\n}\n\n' +
        'query SearchNearby {\n  nearby(lat: 40.6398, lon: -73.7789, radius: 50) {\n    icao\n    name\n    distance\n  }\n}\n';
    ReactDOM.createRoot(graphiqlEl)
        .render(React.createElement(GraphiQL, { fetcher: fetcher, defaultQuery: defaultQuery }));
});
