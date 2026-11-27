# Web Frontend Rules (PART 16)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

> Quick reference only: read AI.md PART 16 completely before acting. AI.md is authoritative; only vendored documentation tools are exempt from the one-project-JavaScript-file rule.

## CRITICAL - NEVER DO

- Use a JS framework/library/bundler/transpiler (React, Vue, Alpine, jQuery, webpack, vite, TypeScript, Babel) — plain browser-native JS only, no npm/node for frontend
- Split JS across multiple files — ALL JavaScript lives in ONE file: `static/js/app.js`
- Use inline `onclick`/`onchange`/inline `<script>`/inline `style="..."` — CSP blocks inline handlers; bind via `data-action` + `addEventListener` in `app.js`, styles in CSS files only
- Use `alert()`, `confirm()`, or `prompt()` — use custom toasts, native `<dialog>` modals, custom input modals
- Put the theme class on `<body>` — it MUST go on `<html>` (`class="theme-{{.Theme}}"`)
- Build a broken no-JS experience — navigation, forms, and CRUD MUST work with JavaScript disabled; JS only enhances
- Link to any server-administration path from public nav/footer — there is no admin web UI; direct config-file/CLI access only
- Ship an unstyled/generic error page (404, 500, etc.) — every error page MUST use `error.tmpl` extending the themed `public.tmpl` layout
- Use desktop-first CSS (base styles for desktop, `max-width` media queries to shrink) — mobile-first only: base = mobile, `@media (min-width: …)` to enhance
- Use physical left/right properties for direction-sensitive layout — use logical properties such as `margin-inline-start`, `inset-inline-end`, `padding-inline`, `border-inline-end`, and `text-align: start`; never write a `max-width` breakpoint
- Shrink `-sm` interactive controls below the 44×44 px touch target — each sized component uses `--size-md` as the base for `-sm` and restores `--size-sm` only inside its named `@media (min-width: 768px) /* md */` rule
- Add page-feature CSS outside `components.css` or invent a second component palette — feature components follow the same class naming, token, logical-property, and breakpoint rules as shared components
- Pass unsanitized user/operator content through `template.HTML` (footer `custom_html`, markdown) — always sanitize with an allow-list (bluemonday) first
- Use `position: fixed`/`sticky` on header/nav/footer — nothing is pinned to the viewport except the cookie banner, toast container, and site banner
- Skip the CSRF double-submit token on mutating browser requests without a bearer header, or accept an Origin-header bypass — there is none
- Render tracking scripts, preference cookies, or third-party embeds before consent is given (or after decline) — `CheckTrackingAllowed()` gates every tracking injection

## CRITICAL - ALWAYS DO

- Give every user-facing feature both an HTML frontend route AND a matching `/api/{api_version}/...` JSON route; auto-detect client type (Accept header → User-Agent → default HTML) and serve HTML/text/JSON accordingly
- Use Go `html/template` (`.tmpl` files) for ALL HTML, embedded via `//go:embed`; structure as `layout/`, `partial/`, `page/`, `component/`
- Apply `URLNormalizeMiddleware` first in the chain (strip trailing slash, 301 redirect, root `/` and file requests excepted)
- Prioritize HTML5 → CSS → JavaScript in that order; every JS line must be justified (no JS for hover, simple toggles, collapsibles — use `<details>`, `:focus-within`, checkbox hack, native `<dialog>`)
- Make ALL error pages, Swagger UI, GraphiQL, CLI, TUI, and GUI share ONE unified theme palette (dark/light/auto); `common.css` is authoritative for web colors, and `src/common/theme/colors.go` mirrors the shared roles with parity tests; default theme is dark; persist via server-readable `theme` cookie (no FOUC, no init JS)
- Meet WCAG 2.1 AA: 4.5:1 contrast minimum, visible focus rings, `aria-label`/`aria-describedby`/`role`, alt text, labeled inputs, skip links, `prefers-reduced-motion` support
- Build full PWA support: `manifest.json`, service worker (install/activate/fetch), offline fallback, all icon sizes incl. maskable, HTTPS-only
- Keep `/server/docs/swagger` and `/server/docs/graphql` inside the shared page shell; self-host their scripts/styles and startup code, use the shared theme tokens, and provide `<noscript>` links to the plain API endpoints
- Word-break/scroll any long unbreakable string (IPv6, onion address, tokens, hashes, UUIDs) — never let it overflow mobile layout
- Give every list/table/data view a proper empty state (icon + title + message + action button) — never blank space
- Validate forms with HTML5 attributes first (`required`, `pattern`, `type=email`), `:user-invalid` for zero-JS error styling, inline `aria-describedby` errors on blur
- Use toast for non-blocking confirmations, modal (native `<dialog>`) for decisions/input/destructive confirmation — never interchange them
- Render the theme toggle, cookie-consent banner, and site announcement banner server-side so they work with zero JavaScript; JS only removes the reload round-trip
- Populate `/server/about` and `/server/help` from real IDEA.md content; source `/server/contact` from its `server.contact.*` and `server.pages.contact.*` configuration, and follow the AI.md-defined sources for privacy and terms pages
- Serve `/sitemap.xml` dynamically (homepage priority 1.0, public pages 0.8, never API routes or auth-gated pages)
- Validate/sanitize any remote-fetched branding image (HTTPS only, block private/loopback/internal IPs, size + MIME-type limits, ≤5 redirects)
- Keep shared and feature-component CSS tokenized. Use `--color-on-primary` for primary foregrounds and `--color-overlay` for overlays; component custom properties with documented fallbacks are allowed outside `:root`
- Treat `src/server/static/css/common.css` as the authoritative web palette; mirror shared semantic roles in Go `ThemePalette` and verify palette parity with tests. Keep page-only web roles in `common.css`.
- Put project CSS in cascade layers ordered `reset`, `tokens`, `base`, `components`, `pages`, `utilities`; the base layer permits the documented page-shell `body`/`main` flex rule only.
- Put feature components (cookie banner, navigation drawer, status, code/copy, stats/lists, admin shell, form feedback, skip link) in `components.css` and include each in `/debug/components`
- Keep class/token lint exemptions limited to third-party theme CSS, examples explicitly labelled `WRONG`, and component custom properties with fallbacks; never copy exempt examples into project stylesheets

- Use JavaScript only for a capability with no HTML5/CSS equivalent; every core flow works with JavaScript disabled.

## Key Rules Summary

| Area | Rule |
|------|------|
| Templates | `html/template`, `.tmpl` ext, embedded via `//go:embed`; `layout/public.tmpl` is the only layout (no admin web UI) |
| CSS | `common.css` → `components.css` → `public.css`; ordered cascade layers (`reset`, `tokens`, `base`, `components`, `pages`, `utilities`); common.css is authoritative for the web palette, Go `ThemePalette` roles are parity-tested; page-only roles stay in common.css; only the page-shell body/main flex rule is allowed in base; mobile-first, logical properties, named breakpoints, no inline styles; `-sm` controls stay at `--size-md` below 768px |
| JS | ONE file `static/js/app.js`; no frameworks/bundlers/transpilers/npm; bind via `data-action` + `addEventListener`, never inline |
| Theme | 3 modes: dark (default) / light / auto; class on `<html>`, `theme` cookie (server-rendered, no FOUC); shared palette across Web/Swagger/GraphQL/CLI/TUI/GUI |
| Mobile breakpoints | Base = mobile (<768px); `min-width:768px` tablet; `min-width:1024px` desktop; touch targets ≥44x44px |
| Modals | Native `<dialog>` (`showModal()`, `<form method="dialog">` to close) — provides focus trap + `::backdrop` free |
| Toasts | `.toast-container` at the top inline-end, max 5 stacked, success/info 3s, warning 5s, error no auto-dismiss, pause on hover |
| CSRF | Stateless double-submit cookie (`csrf_token`, `SameSite=Strict`, not HttpOnly) + `X-CSRF-Token` header/form field; bypassed only for Bearer auth, safe methods, WS upgrade, or explicit exempt paths |
| CORS | Default `allowed_origins: ["*"]`; credentials only sent with an explicit (non-`*`) origin list |
| Cookie consent | Always shown (we use cookies) until `cookie_consent` cookie set; categories: essential (always) / preferences / analytics; decline blocks all tracking/third-party embeds |
| PWA | `manifest.json` + `sw.js` + `offline.html`; icons 72–512px + maskable; cache-first static, network-first HTML, network-only+queue for API |
| Response format | `{"ok":true,"data":{}}` / `{"ok":false,"error":"CODE","message":"..."}`; text responses `OK: ...` / `ERROR: CODE: message`; all non-HTML responses end with one `\n` |
| Standard pages | `/server/about` and `/server/help` use IDEA.md; `/server/contact` uses its config; privacy and terms follow their AI.md content-source rules |

For complete details, see AI.md PART 16.
