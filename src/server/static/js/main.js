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
function showToast(message, type = 'info', duration = 3000) {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;

    container.appendChild(toast);

    setTimeout(() => {
        toast.style.animation = 'slideIn 0.3s ease reverse';
        setTimeout(() => toast.remove(), 300);
    }, duration);
}

// Modal Functions
function showModal(title, content) {
    const container = document.getElementById('modal-container');

    const modal = document.createElement('div');
    modal.className = 'modal active';
    modal.innerHTML = `
        <div class="modal-backdrop" onclick="closeModal()"></div>
        <div class="modal-content">
            <div class="modal-header">
                <h2>${title}</h2>
                <button class="modal-close" onclick="closeModal()">×</button>
            </div>
            <div class="modal-body">
                ${content}
            </div>
        </div>
    `;

    container.innerHTML = '';
    container.appendChild(modal);
}

function closeModal() {
    const container = document.getElementById('modal-container');
    container.innerHTML = '';
}

// API Helper Functions
async function apiGet(endpoint) {
    try {
        const response = await fetch(endpoint);
        const data = await response.json();

        if (!data.success) {
            throw new Error(data.error?.message || 'API request failed');
        }

        return data.data;
    } catch (error) {
        showToast(error.message, 'error');
        throw error;
    }
}

// Utility Functions
function formatDistance(km, units = 'imperial') {
    if (units === 'metric') {
        return `${km.toFixed(2)} km`;
    }
    const miles = km * 0.621371;
    return `${miles.toFixed(2)} mi`;
}

function formatElevation(feet, units = 'imperial') {
    if (units === 'metric') {
        const meters = feet * 0.3048;
        return `${meters.toFixed(0)} m`;
    }
    return `${feet} ft`;
}

function formatCoordinates(lat, lon) {
    const latDir = lat >= 0 ? 'N' : 'S';
    const lonDir = lon >= 0 ? 'E' : 'W';
    return `${Math.abs(lat).toFixed(4)}°${latDir}, ${Math.abs(lon).toFixed(4)}°${lonDir}`;
}

// Close mobile menu when clicking outside
document.addEventListener('click', (e) => {
    const nav = document.getElementById('main-nav');
    const toggle = document.querySelector('.mobile-menu-toggle');

    if (nav && toggle && !nav.contains(e.target) && !toggle.contains(e.target)) {
        nav.classList.remove('active');
    }
});

// Close modal on Escape key
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        closeModal();
    }
});
