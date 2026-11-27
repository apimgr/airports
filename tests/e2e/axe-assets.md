# Browser accessibility test asset

`axe.min.js` is the pinned axe-core browser audit used by the Go/chromedp end-to-end suite.

- Version: 4.14.0
- Source: [`axe-core` on npm](https://www.npmjs.com/package/axe-core), `axe-core-4.14.0.tgz`
- License: Mozilla Public License 2.0; see [LICENSE.axe-core](vendor/LICENSE.axe-core)
- Retrieval date: 2026-11-27
- SHA-256: `20c09fe157a8a34a30e241aaa1fcdade657734f08ab379ecfbeb7d45cc46e878`

The test injects this file with `chromedp.Evaluate` and runs the color-contrast rule. The asset is committed under `tests/e2e/vendor/` because AI.md requires a pinned vendored script and the repository-wide `vendor/` ignore otherwise excludes it.
