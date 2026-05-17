# Frontend Rules (PART 16)

Cheatsheet — see AI.md PART 16 (line 22102).

## Rendering Strategy

- **Server-side Go templates ONLY.** No client-side rendering frameworks (no React, Vue, Svelte, Next, Nuxt).
- Templates in `src/server/template/` — embedded via Go `embed`.
- Progressive enhancement with vanilla JS only.
- Static assets in `src/server/static/` — embedded via Go `embed`.

## PWA (Progressive Web App)

- `manifest.webmanifest` served at `/manifest.webmanifest`.
- Service worker at `/sw.js` — offline-capable.
- App-style icons for all common sizes (192, 256, 384, 512).
- `apple-touch-icon` for iOS.
- Theme color matches dark mode default.

## Themes

- **Dark mode is the DEFAULT.** Light and Auto (OS preference) toggles available.
- CSS custom properties for all colors — NEVER hardcode hex in component styles.
- Three CSS layers: base reset → theme variables → component styles.
- Theme stored in `localStorage` key `theme=dark|light|auto`; respects OS pref when `auto`.
- Theme toggle accessible from every page (header).

## Color Palette (Dracula-inspired default)

| Token | Dark | Light |
|-------|------|-------|
| `--bg` | `#282a36` | `#f8f8f2` |
| `--bg-alt` | `#44475a` | `#e6e6df` |
| `--text` | `#f8f8f2` | `#282a36` |
| `--accent-purple` | `#bd93f9` | `#6f42c1` |
| `--accent-pink` | `#ff79c6` | `#d63384` |
| `--accent-cyan` | `#8be9fd` | `#0891b2` |

## Mobile-First Responsive

- Base styles target mobile (320px+).
- Breakpoints: 640px (sm), 768px (md), 1024px (lg), 1280px (xl).
- Touch targets ≥ 44×44 px.
- Test on real mobile viewport — not just desktop window resize.

## Accessibility (also PART 30)

- Semantic HTML — use `<nav>`, `<main>`, `<article>`, `<button>` properly.
- All interactive elements keyboard-reachable; visible focus ring.
- Color contrast WCAG AA minimum (AAA for body text).
- All images have `alt`; all form fields have `<label>`.
- ARIA only when semantic HTML insufficient.
- `prefers-reduced-motion` respected.

## Required Server Pages

| Path | Purpose |
|------|---------|
| `/` | Home — nearby airports via GeoIP |
| `/server/about` | Version, build info |
| `/server/help` | User help |
| `/server/healthz` | Health status (HTML) |
| `/server/privacy` | Privacy policy |
| `/server/terms` | Terms |
| `/server/docs/swagger` | Swagger UI |
| `/server/docs/graphql` | GraphiQL UI |

## Forms

- All state-changing forms have CSRF token.
- Show inline validation; never lose user input on error.
- Server errors rendered inline with the offending field.
- Submit buttons disabled during in-flight request (prevent double-submit).

## Assets

- All CSS/JS bundled at build time — NO CDN scripts.
- Content-hash in filename for long-cache.
- `<script>` tags include `defer` or `async` where appropriate.
- No inline `<script>` with executable code (CSP-incompatible).

## Performance

- HTML pages ≤ 50 KB compressed.
- Critical CSS inlined for above-the-fold.
- Images served as WebP with fallback.
- Lazy-load below-the-fold images (`loading="lazy"`).
