// UI Helper Functions

function toggleTheme() {
    const html = document.documentElement;
    const currentTheme = html.getAttribute('data-theme');
    const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
    html.setAttribute('data-theme', newTheme);
    document.body.setAttribute('data-theme', newTheme);
    localStorage.setItem('theme', newTheme);

    const icon = document.querySelector('.theme-icon');
    icon.textContent = newTheme === 'dark' ? '🌙' : '☀️';
}

function toggleMobileMenu() {
    const nav = document.getElementById('main-nav');
    nav.classList.toggle('active');
}

// Load saved theme
document.addEventListener('DOMContentLoaded', () => {
    const savedTheme = localStorage.getItem('theme') || 'dark';
    document.documentElement.setAttribute('data-theme', savedTheme);
    document.body.setAttribute('data-theme', savedTheme);

    const icon = document.querySelector('.theme-icon');
    if (icon) {
        icon.textContent = savedTheme === 'dark' ? '🌙' : '☀️';
    }
});

// Toast Notifications
function showToast(message, type, duration) {
    type = type || 'info';
    duration = duration || 3000;
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = 'toast ' + type;
    toast.textContent = message;

    container.appendChild(toast);

    setTimeout(function() {
        toast.style.animation = 'slideIn 0.3s ease reverse';
        setTimeout(function() { toast.remove(); }, 300);
    }, duration);
}

// Modal Functions
function showModal(title, content) {
    const container = document.getElementById('modal-container');

    const modal = document.createElement('div');
    modal.className = 'modal active';
    modal.innerHTML =
        '<div class="modal-backdrop" onclick="closeModal()"></div>' +
        '<div class="modal-content">' +
        '<div class="modal-header">' +
        '<h2>' + title + '</h2>' +
        '<button class="modal-close" onclick="closeModal()">×</button>' +
        '</div>' +
        '<div class="modal-body">' + content + '</div>' +
        '</div>';

    container.innerHTML = '';
    container.appendChild(modal);
}

function closeModal() {
    const container = document.getElementById('modal-container');
    container.innerHTML = '';
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

// Close mobile menu when clicking outside
document.addEventListener('click', function(e) {
    const nav = document.getElementById('main-nav');
    const toggle = document.querySelector('.mobile-menu-toggle');

    if (nav && toggle && !nav.contains(e.target) && !toggle.contains(e.target)) {
        nav.classList.remove('active');
    }
});

// Close modal on Escape key
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        closeModal();
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
                        return '<div class="quick-result-item" onclick="viewAirport(\'' + a.icao + '\')">' +
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
                showModal('Your Location',
                    '<p><strong>IP:</strong> ' + loc.ip + '</p>' +
                    '<p><strong>Location:</strong> ' + (loc.city || 'Unknown') + ', ' + (loc.country_name || loc.country) + '</p>' +
                    '<p><strong>Coordinates:</strong> ' + loc.latitude.toFixed(4) + ', ' + loc.longitude.toFixed(4) + '</p>' +
                    '<p><strong>Nearby Airports:</strong> ' + data.data.nearby_airports.length + '</p>' +
                    '<button onclick="window.location=\'/nearby?lat=' + loc.latitude + '&lon=' + loc.longitude + '\'">View Nearby Airports</button>'
                );
            }
        });
}

// ── Search page ──────────────────────────────────────────────────────────────
function handleSearchKey(event) {
    if (event.key === 'Enter') {
        performSearch();
    }
}

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

function viewOnMap() {
    const el = document.querySelector('[data-lat][data-lon]');
    if (!el) return;
    const lat = el.getAttribute('data-lat');
    const lon = el.getAttribute('data-lon');
    window.open('https://www.openstreetmap.org/?mlat=' + lat + '&mlon=' + lon + '&zoom=12', '_blank');
}

function copyJSON() {
    const jsonEl = document.getElementById('json-data');
    if (!jsonEl) return;
    navigator.clipboard.writeText(jsonEl.textContent).then(function() {
        showToast('JSON copied to clipboard!', 'success');
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
    result.style.display = 'none';
    errBox.style.display = 'none';

    fetch('/api/v1/geoip/' + encodeURIComponent(ip))
        .then(function(r) { return r.json(); })
        .then(function(data) {
            if (data.ok === false) {
                document.getElementById('error-message').textContent = data.message || 'Lookup failed';
                errBox.style.display = 'block';
                return;
            }
            const loc = document.getElementById('location-data');
            loc.innerHTML =
                '<dl>' +
                '<dt>IP</dt><dd>' + (data.ip || ip) + '</dd>' +
                '<dt>Country</dt><dd>' + (data.country_name || '-') + ' (' + (data.country_code || '-') + ')</dd>' +
                '<dt>City</dt><dd>' + (data.city || '-') + '</dd>' +
                '<dt>Latitude</dt><dd>' + (data.lat || '-') + '</dd>' +
                '<dt>Longitude</dt><dd>' + (data.lon || '-') + '</dd>' +
                '<dt>ASN</dt><dd>' + (data.asn || '-') + '</dd>' +
                '<dt>ISP</dt><dd>' + (data.isp || '-') + '</dd>' +
                '</dl>';
            result.style.display = 'block';

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
            errBox.style.display = 'block';
        });
}
