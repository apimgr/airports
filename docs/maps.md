# Airport maps

Airport detail, search, nearby, and bounding-box result pages include server-rendered coordinates and direct OpenStreetMap links. These remain available when JavaScript is disabled or map tiles cannot load. With JavaScript enabled, the page progressively enhances the map area with an interactive OpenStreetMap tile view, airport markers, drag panning, and zoom controls.

Map tiles are requested by each visitor’s browser from `tile.openstreetmap.org`; the Airports server does not proxy or cache them. The map displays OpenStreetMap attribution and links to the copyright and usage information. Tile availability depends on the public tile service and browser network policy.

The nearby page accepts the same bounds as the read-only API (`minLat`, `maxLat`, `minLon`, `maxLon`) and renders up to 100 matching airports with the map and static links. For example: `/nearby?minLat=40&maxLat=42&minLon=-75&maxLon=-73&limit=100`.
