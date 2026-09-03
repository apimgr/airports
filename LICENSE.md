MIT License

Copyright (c) 2024-2026 apimgr

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

---

## Third-Party Licenses

### Airport Data

Source: [OurAirports](https://ourairports.com/)
License: Public Domain

Airport data is sourced from OurAirports and is in the public domain. No attribution is required.

### GeoIP Databases

Source: [sapics/ip-location-db](https://github.com/sapics/ip-location-db)
License: CC BY 4.0 (ASN, country, and city data)

| Database | License | Underlying Source(s) |
|----------|---------|-----------------------|
| ASN | CC BY 4.0 | RouteViews, NRO, DB-IP (merged) |
| Country | CC BY 4.0 | NRO (RIR whois + geofeed + ASN data, merged) |
| City | CC BY 4.0 | DB-IP |

All three databases are licensed CC BY 4.0, and attribution is a license
condition. Both notices below are required together — MaxMind GeoLite2 is
not used by this project:

[IP Geolocation by DB-IP](https://db-ip.com/)

Country and ASN data licensed CC BY 4.0 by the Number Resource Organization (NRO).

### Go Dependencies

This software includes the following third-party Go libraries:

| Library | Version | License | Copyright |
|---------|---------|---------|-----------|
| github.com/cretz/bine | v0.2.0 | MIT | 2017-present Chris Cretzman |
| github.com/go-acme/lego/v4 | v4.35.2 | MIT | 2015-2020 lego contributors |
| github.com/go-chi/chi/v5 | v5.3.0 | MIT | 2015-present Peter Kieltyka, Google Inc. |
| github.com/oschwald/maxminddb-golang | v1.13.0 | ISC | 2015 Gregory J. Oschwald |
| github.com/prometheus/client_golang | v1.23.2 | Apache-2.0 | 2012-2015 The Prometheus Authors |
| github.com/redis/go-redis/v9 | v9.21.0 | BSD-2-Clause | 2012-2024 The go-redis Authors |
| github.com/robfig/cron/v3 | v3.0.1 | MIT | 2012 Rob Figueiredo |
| golang.org/x/crypto | v0.54.0 | BSD-3-Clause | 2009 The Go Authors |
| golang.org/x/sys | v0.47.0 | BSD-3-Clause | 2009 The Go Authors |
| golang.org/x/term | v0.45.0 | BSD-3-Clause | 2009 The Go Authors |
| golang.org/x/text | v0.40.0 | BSD-3-Clause | 2009 The Go Authors |
| golang.org/x/time | v0.15.0 | BSD-3-Clause | 2009 The Go Authors |
| gopkg.in/yaml.v3 | v3.0.1 | MIT | 2006-2011 Kirill Simonov; 2011-2019 Canonical Ltd |
| modernc.org/sqlite | v1.54.0 | BSD-3-Clause | 2017 The Sqlite Authors |

`go-acme/lego` pulls in a large set of transitive DNS-provider SDKs (AWS,
Azure, Google Cloud, Alibaba, and others) used only for optional DNS-01
challenge providers selected via `server.tls.dns_provider`; each is licensed
under its own upstream terms (predominantly MIT/Apache-2.0/BSD). Regenerate
the full transitive list with `go-licenses csv ./...` (`casjaysdev/go:latest`)
when auditing a specific provider's license.

For BSD-3-Clause libraries, the non-endorsement clause applies: neither the
name of the copyright holder nor the names of its contributors may be used to
endorse or promote products derived from this software without specific prior
written permission.

Full license texts available at: https://spdx.org/licenses/
