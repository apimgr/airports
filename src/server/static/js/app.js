// UI Helper Functions

// escapeHtml escapes a string for safe interpolation into an innerHTML
// template literal. Used wherever user-supplied input (as opposed to
// hardcoded markup) is inserted via innerHTML instead of textContent.
function escapeHtml(value) {
    const div = document.createElement('div');
    div.textContent = String(value);
    return div.innerHTML;
}

// Theme is server-rendered (AI.md PART 16: the server reads the "theme"
// cookie and renders theme-light/theme-dark/theme-auto on <html> with no
// init JS and no FOUC). This function only updates the *current* page for
// instant feedback and persists the choice in the server-readable cookie
// so the next navigation/render picks it up.
function applyTheme(theme) {
    const html = document.documentElement;
    html.classList.remove('theme-dark', 'theme-light', 'theme-auto');
    html.classList.add('theme-' + theme);
    document.cookie = 'theme=' + theme + '; path=/; max-age=31536000; SameSite=Lax';

    const icon = document.querySelector('.theme-icon');
    if (icon) {
        icon.textContent = theme === 'dark' ? '🌙' : '☀️';
    }
}

function toggleTheme() {
    const isLight = document.documentElement.classList.contains('theme-light');
    applyTheme(isLight ? 'dark' : 'light');
}

function toggleMobileMenu() {
    const nav = document.getElementById('main-nav');
    nav.classList.toggle('active');
}

// Sync the theme-toggle icon with the class the server already rendered
// on <html> (no class mutation here — that would risk a flash).
document.addEventListener('DOMContentLoaded', () => {
    const icon = document.querySelector('.theme-icon');
    if (icon) {
        icon.textContent = document.documentElement.classList.contains('theme-light') ? '☀️' : '🌙';
    }
});

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
    close.setAttribute('aria-label', 'Dismiss');
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
        '<h2 id="app-modal-title">' + title + '</h2>' +
        '<form method="dialog"><button class="modal-close" aria-label="Close">×</button></form>' +
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
                throw new Error((data.error && data.error.message) || data.message || 'API request failed');
            }
            return data.data !== undefined ? data.data : data;
        })
        .catch(function(error) {
            showToast(error.message, 'error');
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

// ── Swagger UI / GraphiQL bootstrap ─────────────────────────────────────────
// Both pages load their vendor bundles as external <script src> tags before
// this file, then embed the per-request spec/endpoint URL as a data
// attribute (never inline JS) so the single app.js file can initialize them
// under a strict script-src 'self' CSP.
document.addEventListener('DOMContentLoaded', function() {
    const swaggerEl = document.getElementById('swagger-ui');
    if (swaggerEl && typeof SwaggerUIBundle !== 'undefined') {
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
    }

    const graphiqlEl = document.getElementById('graphiql');
    if (graphiqlEl && typeof GraphiQL !== 'undefined') {
        const fetcher = GraphiQL.createFetcher({ url: graphiqlEl.getAttribute('data-endpoint-url') });
        const defaultQuery = '# Welcome to the Airports GraphQL API\n# Press the Play button to run a query\n\n' +
            'query GetAirport {\n  airport(code: "KJFK") {\n    icao\n    iata\n    name\n    city\n    country\n    coordinates { lat lon }\n  }\n}\n\n' +
            'query SearchNearby {\n  nearby(lat: 40.6398, lon: -73.7789, radius: 50) {\n    icao\n    name\n    distance\n  }\n}\n';
        ReactDOM.createRoot(graphiqlEl)
            .render(React.createElement(GraphiQL, { fetcher: fetcher, defaultQuery: defaultQuery }));
    }
});

// Service worker registration
if ('serviceWorker' in navigator) {
    window.addEventListener('load', function() {
        navigator.serviceWorker.register('/sw.js').catch(function() {});
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
        fetch('/api/v1/airports/search?q=' + encodeURIComponent(query) + '&limit=5')
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
                        '<div class="no-results">No airports found</div>';
                }
            });
    }, 300);
}

function detectLocation() {
    fetch('/api/v1/geoip/airports/nearby')
        .then(function(r) { return r.json(); })
        .then(function(data) {
            if (data.ok !== false && data.data) {
                const loc = data.data.location;
                const nearbyHref = '/nearby?lat=' + loc.latitude + '&lon=' + loc.longitude;
                showModal('Your Location',
                    '<p><strong>IP:</strong> ' + loc.ip + '</p>' +
                    '<p><strong>Location:</strong> ' + (loc.city || 'Unknown') + ', ' + (loc.country_name || loc.country) + '</p>' +
                    '<p><strong>Coordinates:</strong> ' + loc.latitude.toFixed(4) + ', ' + loc.longitude.toFixed(4) + '</p>' +
                    '<p><strong>Nearby Airports:</strong> ' + data.data.nearby_airports.length + '</p>' +
                    '<button data-action="navigate" data-href="' + nearbyHref + '">View Nearby Airports</button>'
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
        window.location = '/search?q=' + encodeURIComponent(query);
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
    window.location = '/nearby?lat=' + lat + '&lon=' + lon + '&radius=100';
}

// "View on Map" is now a plain <a href target="_blank"> in airport.html
// (AI.md PART 16 no-JS map fallback) so no JS is needed to open the map.

function copyJSON() {
    const jsonEl = document.getElementById('json-data');
    if (!jsonEl) return;
    navigator.clipboard.writeText(jsonEl.textContent).then(function() {
        showToast('JSON copied to clipboard!', 'success');
    });
}

// copyValue copies the element's data-copy attribute to the clipboard and
// swaps the trigger button's label to a translated "Copied!" state for
// visual feedback, falling back to the untranslated attribute value if the
// server did not render data-copied-text (i18n keys come from the template,
// never hardcoded here).
function copyValue(target) {
    const value = target.getAttribute('data-copy');
    if (!value) return;
    navigator.clipboard.writeText(value).then(function() {
        const copiedText = target.getAttribute('data-copied-text') || 'Copied!';
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
    window.location = '/search?q=' + encodeURIComponent(country);
}

// ── Nearby page ───────────────────────────────────────────────────────────────
function findNearbyAirports() {
    const lat    = document.getElementById('latitude').value;
    const lon    = document.getElementById('longitude').value;
    const radius = document.getElementById('radius').value;
    const units  = document.getElementById('units') && document.getElementById('units').value;
    const limit  = document.getElementById('limit') && document.getElementById('limit').value;

    if (!lat || !lon) {
        showToast('Please enter both latitude and longitude', 'error');
        return;
    }

    let url = '/nearby?lat=' + lat + '&lon=' + lon + '&radius=' + radius;
    if (units) url += '&units=' + units;
    if (limit) url += '&limit=' + limit;
    window.location = url;
}

function useMyLocation() {
    if (!navigator.geolocation) {
        showToast('Geolocation is not supported by your browser', 'error');
        return;
    }

    showToast('Getting your location...', 'info');

    navigator.geolocation.getCurrentPosition(
        function(position) {
            document.getElementById('latitude').value = position.coords.latitude.toFixed(4);
            document.getElementById('longitude').value = position.coords.longitude.toFixed(4);
            showToast('Location detected!', 'success');
        },
        function(error) {
            showToast('Unable to get your location: ' + error.message, 'error');
        }
    );
}

function viewAirport(code) {
    window.location = '/airports/' + code;
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

    fetch('/api/v1/geoip/' + encodeURIComponent(ip))
        .then(function(r) { return r.json(); })
        .then(function(data) {
            if (data.ok === false) {
                document.getElementById('error-message').textContent = data.message || 'Lookup failed';
                errBox.classList.remove('hidden');
                return;
            }
            const loc = document.getElementById('location-data');
            loc.innerHTML =
                '<dl>' +
                '<dt>IP</dt><dd>' + escapeHtml(data.ip || ip) + '</dd>' +
                '<dt>Country</dt><dd>' + (data.country_name || '-') + ' (' + (data.country_code || '-') + ')</dd>' +
                '<dt>City</dt><dd>' + (data.city || '-') + '</dd>' +
                '<dt>Latitude</dt><dd>' + (data.lat || '-') + '</dd>' +
                '<dt>Longitude</dt><dd>' + (data.lon || '-') + '</dd>' +
                '<dt>ASN</dt><dd>' + (data.asn || '-') + '</dd>' +
                '<dt>ISP</dt><dd>' + (data.isp || '-') + '</dd>' +
                '</dl>';
            result.classList.remove('hidden');

            if (data.lat && data.lon) {
                fetch('/api/v1/geoip/airports/nearby?ip=' + encodeURIComponent(ip))
                    .then(function(r) { return r.json(); })
                    .then(function(nearby) {
                        const airports = nearby.data || [];
                        const nearbyEl = document.getElementById('nearby-data');
                        if (!airports.length) {
                            nearbyEl.textContent = 'No airports found nearby.';
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
                        document.getElementById('nearby-data').textContent = 'Could not load nearby airports.';
                    });
            }
        })
        .catch(function(err) {
            document.getElementById('error-message').textContent = 'Request failed: ' + err.message;
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
        case 'toggle-theme':
            toggleTheme();
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
        default:
            break;
    }
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
