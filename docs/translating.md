# Translation Guide

English is the source language for Airports. The locale files are embedded in
the server and CLI binaries; both use the same keys and supported languages.

## Current Languages

The repository contains seven locale files:

| Code | Language | Native name | Direction | Translator credit |
|------|----------|-------------|-----------|-------------------|
| `ar` | Arabic | العربية | RTL | Not provided in locale metadata |
| `de` | German | Deutsch | LTR | Not provided in locale metadata |
| `en` | English | English | LTR | Source language |
| `es` | Spanish | Español | LTR | Not provided in locale metadata |
| `fr` | French | Français | LTR | Not provided in locale metadata |
| `ja` | Japanese | 日本語 | LTR | Not provided in locale metadata |
| `zh` | Chinese | 中文 | LTR | Not provided in locale metadata |

Credits belong in the locale file's `meta.translators` field and in this
table. Use the name or handle the translator asks to publish. Update the
README language count and this page whenever a locale is added or retired.

## Files and Source of Truth

Locale files live in [`src/common/i18n/locales/`](../src/common/i18n/locales/).
`en.json` defines the keys and source text. Do not translate keys or edit the
English file while translating another language. Keep the same JSON structure,
key order, and two-space indentation in each locale file. Files use UTF-8 JSON
without comments or trailing commas.

## Translating an Existing Language

1. Choose one locale file, such as `fr.json`.
2. Translate its string values while preserving every key and nested object.
3. Keep every `{name}` placeholder exactly as written, including braces and
   spelling. Its position may change to fit natural grammar.
4. Preserve plural categories required by the language and complete sentences;
   do not translate fragments in isolation when the source can be clarified.
5. Check the glossary and keep product names, airport codes, protocol names,
   code, configuration keys, URLs, and literal values such as `dark` and `auto`
   unchanged.
6. Add or update your credit in the locale's `meta.translators` field and in
   the table above if you want public attribution.
7. Check the translated interface at 320, 768, and 1280 pixels. For RTL
   languages, also check that layout and navigation read correctly in RTL.

Submit a pull request containing the changed locale file and this page only
when translator credits or locale counts also change. A maintainer reviews key
and placeholder parity, glossary usage, and layout. Native-speaker review is
welcome, but it is not required for merging.

## Adding a Language

1. Copy `en.json` to `src/common/i18n/locales/{language-code}.json`, using a
   lowercase BCP 47 language code.
2. Translate every key and fill `meta.language`, `meta.name`,
   `meta.native_name`, `meta.direction`, and `meta.version`. Add optional
   `meta.translators` credits only with the translator's permission.
3. Register the language and add it to `server.i18n.available_languages` in
   the configuration defaults.
4. Rebuild the server and CLI so both binaries embed the new locale.
5. Update the language selector, README language list, locale table above, and
   the count in this guide.
6. Run the project test target in the configured Docker toolchain; the i18n
   tests check that locale keys and placeholders match the English source.

## Glossary

| Term | Translation guidance |
|------|----------------------|
| Airport | Use the standard local-language term for an airport. |
| ICAO / IATA | Keep the acronyms and airport codes unchanged. |
| GeoIP | Keep the technical name `GeoIP`; explain it in translated prose when useful. |
| API, JSON, CSV, GeoJSON, GraphQL | Keep protocol and format names unchanged. |
| OpenStreetMap | Keep the product name unchanged. |
| Tor, I2P, `.onion` | Keep network names and the literal `.onion` suffix unchanged. |
| `dark`, `light`, `auto` | Keep these literal configuration and preference values unchanged. |

## Translation Style

- Use a consistent, neutral register and natural phrasing for the language.
- Translate complete phrases; do not rely on concatenating fragments.
- Keep product and technology names, code, file names, configuration keys,
  URLs, and literal values unchanged.
- Preserve language-specific punctuation, spacing, decimal separators, and
  quotation marks.
- Avoid shipping uncertain machine translations. If the language cannot be
  translated reliably, wait for a human translator to contribute it.
- Keep all locale files synchronized whenever English strings are added,
  changed, or removed.
