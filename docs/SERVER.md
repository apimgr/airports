# 🔐 Admin Authentication System

**For Airports API Server v1.0**

This project uses a simplified admin authentication system optimized for an API-focused server.

## Overview

Unlike the full Universal Server Template (which has user accounts and admin accounts), this project only implements **admin-only authentication** for server configuration.

**Why Simplified?**
- This is a public API server for airport data
- No user accounts needed (all airport data is public)
- Admin authentication only protects server configuration (`/config`)
- Simpler to deploy and maintain

## Authentication Methods

### 1. API Token (for programmatic access)
- **Header**: `Authorization: Bearer <token>`
- **Use**: API endpoints, automation, scripts
- **Format**: 64-character hex string

### 2. Basic Auth (for web UI access)
- **Header**: `Authorization: Basic <base64(username:password)>`
- **Use**: Browser access to `/config`
- **Browser**: Prompts automatically for credentials

## First Run Initialization

On first start, the server automatically:

1. **Generates admin credentials** (if not provided via environment)
   - Username: `administrator` (or ENV:ADMIN_USER)
   - Password: Random 16-char (or ENV:ADMIN_PASSWORD)
   - Token: Random 64-char hex (or ENV:ADMIN_TOKEN)

2. **Saves to database** (hashed with SHA-256)
   - `admin.username`
   - `admin.password_hash`
   - `admin.token_hash`

3. **Writes credentials file** (`./config/admin_credentials`)
   - Permissions: 0600 (owner read/write only)
   - Contains username, password (if auto-generated), and token
   - **Shown ONCE** - save securely!

## Environment Variables

Set these **before first run** to customize admin credentials:

```bash
# Admin username (default: administrator)
export ADMIN_USER="admin"

# Admin password (default: random 16-char)
export ADMIN_PASSWORD="your-secure-password"

# Admin API token (default: random 64-char hex)
export ADMIN_TOKEN="your-api-token-here"

# Config directory (default: ./config)
export CONFIG_DIR="./config"

# Database connection
export DATABASE_URL="sqlite:./data/airports.db"
# OR
export DB_TYPE="sqlite"
export DB_PATH="./data/airports.db"
```

**After first run**, credentials are stored in the database and environment variables are ignored.

## Database Connection Strings

Supported formats:

```bash
# SQLite (default)
DATABASE_URL="sqlite:./data/airports.db"
DATABASE_URL="sqlite:/var/lib/airports/db.sqlite"

# MySQL
DATABASE_URL="mysql://user:password@localhost:3306/airports"

# PostgreSQL
DATABASE_URL="postgres://user:password@localhost:5432/airports"
DATABASE_URL="postgresql://user:password@localhost:5432/airports?sslmode=disable"
```

## Protected Routes

### Web UI (requires Basic Auth)
- `GET /config` - Configuration management page
- `POST /config/update` - Update single setting
- `POST /config/reset` - Reset to defaults
- `GET /config/export` - Export settings as JSON

### API (requires Bearer Token)
- `GET /api/v1/config` - Get all settings (JSON)
- `GET /api/v1/config?category=server` - Get category settings

## Public Routes (No Auth)

All airport data endpoints are public:
- `/` - Homepage
- `/search` - Search airports
- `/nearby` - Find nearby airports
- `/airport/{code}` - Airport details
- `/stats` - Database statistics
- `/api/v1/airports/*` - All airport API endpoints
- `/api/v1/geoip/*` - All GeoIP endpoints
- `/healthz` - Health check

## Usage Examples

### Web UI Access

1. Open browser to `http://localhost:8080/config`
2. Browser prompts for credentials
3. Enter username and password from `./config/admin_credentials`
4. Access configuration page

### API Access with cURL

```bash
# Get token from credentials file
TOKEN=$(grep "Token:" config/admin_credentials | awk '{print $NF}')

# Get all settings
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/config

# Get server settings only
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/config?category=server

# Update a setting
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key":"server.title","value":"My Airport API","type":"string"}' \
  http://localhost:8080/config/update
```

### API Access with Python

```python
import requests

# Load token from file
with open('config/admin_credentials') as f:
    for line in f:
        if 'Token:' in line:
            token = line.split()[-1]
            break

headers = {'Authorization': f'Bearer {token}'}

# Get all settings
r = requests.get('http://localhost:8080/api/v1/config', headers=headers)
settings = r.json()

print(f"Server title: {settings['data']['server'][0]['value']}")
```

## Security Notes

1. **Credentials File**
   - Generated once on first run
   - Permissions: 0600 (readable only by owner)
   - Contains plaintext password and token
   - **Back up securely!**

2. **Database Storage**
   - Passwords and tokens are SHA-256 hashed
   - Hashes cannot be reversed
   - Lost credentials = delete database and restart

3. **Reset Credentials**
   ```bash
   # Stop server
   pkill airports

   # Delete database
   rm data/airports.db

   # Optional: set new credentials
   export ADMIN_USER="newadmin"
   export ADMIN_PASSWORD="newsecurepassword"

   # Restart (generates new credentials)
   ./airports
   ```

4. **Production Recommendations**
   - Use HTTPS (configure SSL certificates)
   - Set strong ADMIN_PASSWORD before first run
   - Rotate ADMIN_TOKEN periodically
   - Restrict /config routes to internal network
   - Monitor access logs

## Differences from Universal Server Template

The full Universal Server Template spec includes:
- User registration and accounts
- User/Admin separation
- Session management
- Password reset flows
- Email verification
- 2FA support

**This project simplifies to:**
- Admin-only authentication
- No user accounts (public API)
- Token + Basic Auth
- Configuration protection only

This approach is appropriate because:
- Airport data is public (no user accounts needed)
- Only server configuration needs protection
- Simpler deployment and maintenance
- Focused on API functionality

## Troubleshooting

### "Unauthorized" when accessing /config

**Web UI:**
- Check username/password in `./config/admin_credentials`
- Browser may cache old credentials (try incognito mode)
- Check database exists: `ls -lh data/airports.db`

**API:**
- Verify token: `grep "Token:" config/admin_credentials`
- Check header format: `Authorization: Bearer <token>` (no quotes around token)
- Ensure token is complete (64 hex characters)

### Lost Credentials

```bash
# Option 1: Check the credentials file
cat config/admin_credentials

# Option 2: Reset (deletes all data!)
rm data/airports.db
./airports  # Generates new credentials
```

### Change Password

Currently requires database reset. Future enhancement: add password change endpoint.

### Multiple Admin Users

Current implementation supports one admin account. For multiple admins, consider:
- Sharing the same credentials (less secure)
- Implementing full user system (use Universal Server Template)
- Using reverse proxy with authentication (nginx, Caddy)

## Files

```
./
├── config/
│   └── admin_credentials          # Generated on first run (0600)
├── data/
│   └── airports.db                # SQLite database with hashed credentials
└── airports                       # Server binary
```

## Related Documentation

- [CLAUDE.md](./CLAUDE.md) - Full server specification
- [README.md](./README.md) - Installation and usage
- Universal Server Template v1.0 - Full template specification (see CLAUDE.md section)
