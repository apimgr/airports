-- Settings table (configuration stored in database)
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('string', 'number', 'boolean', 'json')),
    category TEXT NOT NULL,
    description TEXT,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert default settings
INSERT OR IGNORE INTO settings (key, value, type, category, description) VALUES
    ('server.title', 'Airports API', 'string', 'server', 'Application display name'),
    ('server.tagline', 'Global airport location information', 'string', 'server', 'Short subtitle/slogan'),
    ('server.description', 'A comprehensive API for accessing global airport location data with GeoIP integration. Search, locate, and explore 29,000+ airports worldwide.', 'string', 'server', 'Full application description'),
    ('server.http_port', '8080', 'number', 'server', 'HTTP port number'),
    ('server.timezone', 'UTC', 'string', 'server', 'Server timezone'),
    ('server.date_format', 'US', 'string', 'server', 'Date format (US/EU/ISO)'),
    ('server.time_format', '12-hour', 'string', 'server', 'Time format (12-hour/24-hour)'),
    ('server.default_units', 'imperial', 'string', 'server', 'Default unit system (imperial/metric)'),
    ('api.rate_limit_enabled', 'false', 'boolean', 'api', 'Enable API rate limiting'),
    ('api.rate_limit_requests', '100', 'number', 'api', 'Requests per minute per IP'),
    ('api.cors_enabled', 'true', 'boolean', 'api', 'Enable CORS'),
    ('api.cors_origin', '*', 'string', 'api', 'CORS allowed origins'),
    ('features.geoip_enabled', 'true', 'boolean', 'features', 'Enable GeoIP lookups'),
    ('features.nearby_max_radius', '500', 'number', 'features', 'Maximum radius for nearby searches (km)'),
    ('features.search_max_results', '1000', 'number', 'features', 'Maximum search results');

-- Index for faster lookups
CREATE INDEX IF NOT EXISTS idx_settings_category ON settings(category);
