# AIRPORTS Specification

**Name**: airports

---

# HOW TO USE THIS TEMPLATE

## CRITICAL: Template File is READ-ONLY

**The template file (this file) is READ-ONLY. NEVER modify it directly.**

The template file may be named:
- `TEMPLATE.md` (standard)
- `SPEC.md` (legacy)
- `CLAUDE.md` (legacy)
- Or any other name containing specification content

**How to identify the template file:**
- Contains `airports`, `apimgr` variables (not replaced)
- Contains "HOW TO USE THIS TEMPLATE" section
- Contains multiple "PART X:" numbered sections
- Much larger than a typical project AI.md

**Template file location is flexible:**
- Project root
- Organization root
- Different organization entirely
- Home directory (`~/`)
- Any other location the user specifies

## File Types

| File | Location | Purpose | Modify? |
|------|----------|---------|---------|
| **Template file** | Organization/shared | Master specification | **NEVER** |
| **AI.md** | Each project repo | Project specification | **YES** |
| **TODO.AI.md** | Each project repo | Task tracking | **YES** |

## File Workflow

```
Template File (read-only, organization-level)
     │
     ├── AI.md missing? ────► Copy template to AI.md, replace variables
     │
     ├── AI.md exists? ─────► Update AI.md to reflect template changes
     │
     └── Old files exist? ──► Merge into AI.md, DELETE old files
           (CLAUDE.md, SPEC.md in project repo)
```

## For New Projects

1. **Copy template file** to your project as `AI.md`
2. **Replace all variables** with your project values:
   - `airports` → your project name (e.g., `jokes`)
   - `apimgr` → your organization (e.g., `apimgr`)
   - `AIRPORTS` → uppercase project name (e.g., `JOKES`)
   - `{gitprovider}` → your git host (e.g., `github`)
3. **Fill in project-specific sections**:
   - PART 2: Project Description and Features
   - PART 30: Project-Specific API Endpoints, Data Files, Configuration
4. **Delete this "HOW TO USE" section** from AI.md after setup
5. **Create required files**: AI.md, TODO.AI.md, README.md, LICENSE.md

## For Existing Projects

1. **Check for AI.md** - if missing, create from template
2. **Migrate old spec files** (if they exist in project repo):
   - `CLAUDE.md` → merge into `AI.md`, **DELETE**
   - `SPEC.md` → merge into `AI.md`, **DELETE**
3. **Audit existing code** against AI.md specification
4. **Document gaps** in TODO.AI.md

---

# AI ASSISTANT INSTRUCTIONS (READ THIS ENTIRE SECTION)

## CRITICAL: Specification Drift Prevention

**AI assistants have a tendency to "drift" from specifications over time.** This means gradually deviating from documented requirements, inventing new patterns, or forgetting constraints. **This is unacceptable.**

### The Problem

| Drift Type | Example | Impact |
|------------|---------|--------|
| **Pattern drift** | Using different file structure than specified | Inconsistency across projects |
| **Naming drift** | Using different variable/function names | Code doesn't match spec |
| **Feature drift** | Adding unrequested features | Over-engineering, bugs |
| **Constraint drift** | Forgetting NON-NEGOTIABLE rules | Specification violations |
| **Format drift** | Using different date/version formats | Integration failures |

### The Solution: Constant Re-verification

**You MUST re-read relevant spec sections before EVERY implementation.**

```
BEFORE writing ANY code:
1. Identify which PART(s) of the spec apply
2. Re-read those sections completely
3. Verify your planned implementation matches EXACTLY
4. If ANY deviation is needed, ASK first
5. NEVER assume you remember correctly - ALWAYS re-check
```

## Mandatory Compliance Checks

**Perform these checks at regular intervals:**

| When | Action |
|------|--------|
| **Start of session** | Read AI.md completely, identify applicable sections |
| **Before each task** | Re-read relevant PART(s) of the spec |
| **Every 3-5 changes** | Stop and verify against spec - are you drifting? |
| **Before completing task** | Full compliance check against relevant sections |
| **If uncertain** | STOP and re-read spec, or ASK user |

## What "NON-NEGOTIABLE" Means

**NON-NEGOTIABLE sections MUST be implemented EXACTLY as specified.**

| Allowed | NOT Allowed |
|---------|-------------|
| Copy spec exactly | "Improve" or "optimize" the spec |
| Ask for clarification | Assume you know better |
| Report conflicts | Silently deviate |
| Request exceptions | Make exceptions yourself |

**If you think a NON-NEGOTIABLE section is wrong:**
1. STOP implementation
2. Tell the user specifically what you think is wrong
3. Ask for explicit permission to deviate
4. Document the deviation in AI.md if approved

## Template File Rules

| Rule | Description |
|------|-------------|
| **NEVER modify template** | Template file is read-only (location varies) |
| **Identify by content** | Has unreplaced `{variables}`, "HOW TO USE" section |
| **AI.md is project spec** | Copy template → AI.md, customize for project |
| **Keep AI.md current** | Update when template changes or project changes |
| **Delete old files** | Merge CLAUDE.md/SPEC.md (in project) → AI.md |

## Before Starting ANY Work

1. **Locate AI.md** - this is the project specification
2. **If AI.md missing** - create from template, replace variables
3. **Read AI.md COMPLETELY** - not just relevant parts
4. **Identify applicable sections** - which PARTs apply to this task?
5. **Check TODO.AI.md** - see pending tasks
6. **Verify understanding** - if ANYTHING is unclear, ASK

## During Work

1. **Re-read spec before each implementation** - EVERY time
2. **Follow spec EXACTLY** - no "improvements" without asking
3. **Check yourself frequently** - am I drifting from spec?
4. **Update TODO.AI.md** as tasks complete
5. **If stuck or uncertain** - re-read spec or ASK, never guess

## Common Drift Mistakes to Avoid

| Mistake | Correct Approach |
|---------|------------------|
| "I'll use a better pattern" | Use the pattern in the spec |
| "This format makes more sense" | Use the format in the spec |
| "I remember the spec says..." | Re-read the spec, don't rely on memory |
| "This is obviously what they want" | Check the spec, ask if not covered |
| "I'll add this helpful feature" | Only implement what's requested |
| "The spec is outdated here" | Ask before deviating |

## Quick Reference

| File | Location | Purpose | Modify? |
|------|----------|---------|---------|
| Template | Varies (see below) | Master spec | **NEVER** |
| AI.md | Project repo | Project spec | Yes |
| TODO.AI.md | Project repo | Task tracking | Yes |
| README.md | Project repo | User docs | Yes |

---

# PART 1: CORE RULES (READ FIRST - NON-NEGOTIABLE)

## Working Roles

When working on this project, the following roles are assumed based on the task:

- **Senior Go Developer** - Writing production-quality Go code, making architectural decisions, following best practices, optimizing performance
- **UI/UX Designer** - Creating professional, functional, visually appealing interfaces with excellent user experience
- **Beta Tester** - Testing applications, finding bugs, edge cases, and issues before they reach users
- **User** - Thinking from the end-user perspective, ensuring things are intuitive and work as expected

These are not roleplay - they ARE these roles when the work requires it. Each project gets the full expertise of all four perspectives.

---

## CRITICAL: Specification Compliance

**STOP AND READ THIS SECTION COMPLETELY BEFORE PROCEEDING.**

### The Golden Rules

1. **Re-read this spec periodically** during work to ensure accuracy and no deviation
2. **When in doubt, check the spec** - the spec is the source of truth
3. **Never assume or guess** - ask questions if unclear
4. **Every NON-NEGOTIABLE section MUST be implemented exactly as specified**
5. **Keep AI.md in sync with the project** - always update after changes

### Required Documentation Files

| File | Location | Purpose | Modify? |
|------|----------|---------|---------|
| **Template file** | Varies (anywhere) | Master specification | **NEVER** |
| **AI.md** | Project repository | Project-specific specification | **YES** |
| **TODO.AI.md** | Project repository | Task tracking (>2 tasks) | **YES** |

**Note:** Template file may be named `TEMPLATE.md`, `SPEC.md`, `CLAUDE.md`, or other. Location varies (project root, org root, ~/, etc.). Identify by content: unreplaced `{variables}`, "HOW TO USE" section, multiple PART X: sections.

### Documentation Rules (NON-NEGOTIABLE)

| Rule | Description |
|------|-------------|
| **Template is READ-ONLY** | Never modify - it is the master specification |
| **AI.md is the project spec** | Copy from template, customize for project |
| **Keep AI.md current** | Update when template or project changes |
| **Migrate old files** | Old spec files in project repo → merge into `AI.md`, DELETE |
| **TODO.AI.md for >2 tasks** | Required when doing more than 2 tasks |

**File Hierarchy:**
```
Template File (master, read-only, organization-level)
    └── AI.md (project-specific, each repository)
            └── TODO.AI.md (task tracking, each repository)
```

### README.md (NON-NEGOTIABLE)

**README.md MUST always be kept updated. Update after ANY feature change, bug fix, or configuration change.**

#### Section Order (MUST follow this order)

1. **Title & Badges** - Project name, build status, version badges
2. **About** - Brief description of what the project does
3. **Official Site** - Link to official site (if defined, e.g., `https://airports.apimgr.us`)
4. **Features** - Key features list
5. **Production** - Production deployment instructions (Docker, binary)
6. **Configuration** - Key configuration options
7. **API** - API endpoints summary (if applicable)
8. **Other** - Additional info (troubleshooting, FAQ, etc.)
9. **Development** - Development setup (ALWAYS LAST)
10. **License** - License info

#### README Template

```markdown
# airports

[![Build Status](https://jenkins.casjay.cc/buildStatus/icon?job=apimgr/airports)](https://jenkins.casjay.cc/job/apimgr/job/airports/)
[![GitHub release](https://img.shields.io/github/v/release/apimgr/airports)](https://github.com/apimgr/airports/releases)
[![License](https://img.shields.io/github/license/apimgr/airports)](LICENSE.md)

## About

{Brief description of what the project does - 1-3 sentences}

## Official Site

https://airports.apimgr.us

## Features

- Feature 1
- Feature 2
- Feature 3

## Production

### Docker (Recommended)

```bash
docker run -d \
  --name airports \
  -p 64580:80 \
  -v ./rootfs/config/airports:/config \
  -v ./rootfs/data/airports:/data \
  ghcr.io/apimgr/airports:latest
```

### Docker Compose

```bash
curl -O https://raw.githubusercontent.com/apimgr/airports/main/docker-compose.yml
docker compose up -d
```

### Binary

```bash
# Download latest release
curl -LO https://github.com/apimgr/airports/releases/latest/download/airports-linux-amd64

# Make executable and run
chmod +x airports-linux-amd64
./airports-linux-amd64
```

## Configuration

Configuration is auto-generated on first run. Edit via admin panel at `http://{host}:{port}/admin`.

Key settings:
- `server.port` - Listen port (default: random 64xxx)
- `server.mode` - production or development

## API

API documentation available at `/api/v1/` when running.

| Endpoint | Description |
|----------|-------------|
| `GET /healthz` | Health check |
| `GET /api/v1/...` | API endpoints |

## Other

### Troubleshooting

- Check logs: `docker logs airports`
- Health check: `curl http://{host}:{port}/healthz`

## Development

**Development instructions are for contributors only.**

### Prerequisites

- Go (latest stable)
- Docker (for containerized builds)

### Build

```bash
# Clone
git clone https://github.com/apimgr/airports
cd airports

# Build (containerized - ALWAYS use Docker)
docker run --rm -v $(pwd):/build -w /build -e CGO_ENABLED=0 golang:alpine go build -o /tmp/airports ./src

# Test
make test
```

### Project Structure

```
src/           # Source code
tests/         # Test files
binaries/      # Built binaries (gitignored)
```

## License

MIT - See [LICENSE.md](LICENSE.md)
```

#### README Update Rules

| When | Update README |
|------|---------------|
| New feature added | Yes - add to Features, update API if applicable |
| Bug fix | Only if it affects usage |
| Configuration change | Yes - update Configuration section |
| New CLI flag | Yes - document in appropriate section |
| Docker/deployment change | Yes - update Production section |
| Breaking change | Yes - add notice at top |

**NEVER let README become outdated. It is the first thing users see.**

---

## Development Principles (NON-NEGOTIABLE)

**EVERY principle below MUST be followed. No exceptions.**

| Principle | Description |
|-----------|-------------|
| **Validate Everything** | All input must be validated before processing |
| **Sanitize Appropriately** | Clean data where needed |
| **Save Only Valid Data** | Never persist invalid data |
| **Clear Only Invalid Data** | Don't destroy valid data |
| **Test Everything** | Comprehensive testing where applicable |
| **Show Tooltips/Docs** | Help users understand the interface |
| **Security First** | But security should never block usability |
| **Mobile First** | Responsive design for all screen sizes |
| **Sane Defaults** | Everything has sensible default values |
| **No AI/ML** | Smart logic only, no machine learning |
| **Concise Responses** | Short, descriptive, and helpful |
| **Everything Configurable** | ALL settings MUST be configurable via admin web UI |
| **Live Reload** | Configuration changes apply immediately without restart |

### Admin Web UI Configuration (NON-NEGOTIABLE)

**EVERY setting in the configuration file MUST be editable via the admin web UI.**

| Rule | Description |
|------|-------------|
| **No SSH/CLI required** | Users should NEVER need to edit config files manually |
| **Complete coverage** | 100% of `server.yml` settings available in admin panel |
| **Grouped logically** | Settings organized into intuitive sections |
| **Tooltips/help** | Every setting has a description explaining what it does |
| **Validation** | Real-time validation with clear error messages |
| **Defaults shown** | Show default values and current values clearly |

### Live Reload (NON-NEGOTIABLE)

**Configuration changes MUST apply immediately without server restart.**

| Rule | Description |
|------|-------------|
| **No restart required** | Changes take effect immediately after saving |
| **Hot reload** | Application watches for config changes and reloads |
| **Graceful** | In-flight requests complete before new config applies |
| **Feedback** | User sees confirmation that changes are active |
| **Exceptions** | Only port/address changes require restart (with clear warning) |

**What MUST live reload:**
- Branding (title, tagline, description)
- SEO settings
- Theme changes
- Email/notification settings
- Rate limiting rules
- robots.txt / security.txt
- Scheduler settings
- SSL settings (except port)
- Tor enable/disable
- All feature toggles

**What MAY require restart (with warning):**
- Listen address
- Port number
- Database driver change

### Sensitive Information Handling (NON-NEGOTIABLE)

**NEVER expose sensitive information unless absolutely necessary:**

- Tokens/passwords shown ONLY ONCE on generation (must be copied immediately)
- Show only on: first run, password changes, token regeneration
- Show in difficult environments: Docker, headless servers
- **NEVER log sensitive data**
- **NEVER in error messages or stack traces**
- Mask in UI: show `••••••••` or last 4 chars only

---

## Target Audience

- Self-hosted users
- SMB (Small/Medium Business)
- Enterprise
- **IMPORTANT: Assume self-hosted and SMB users are NOT tech-savvy**

---

# CHECKPOINT 1: CORE RULES VERIFICATION

Before proceeding, confirm you understand:
- [ ] All NON-NEGOTIABLE sections must be implemented exactly
- [ ] AI.md must be kept in sync with project state
- [ ] TODO.AI.md required for more than 2 tasks
- [ ] Sensitive data handling rules
- [ ] Target audience includes non-tech-savvy users

---

# PART 2: PROJECT STRUCTURE

## Project Information

| Field | Value |
|-------|-------|
| **Name** | airports |
| **Organization** | apimgr |
| **Official Site** | https://airports.apimgr.us |
| **Repository** | https://github.com/apimgr/airports |
| **README** | README.md |
| **License** | MIT > LICENSE.md |
| **Embedded Licenses** | Added to bottom of LICENSE.md |

## Project Description

{Brief description of what this project does}

## Project-Specific Features

{List features unique to this project}

---

## Variables (NON-NEGOTIABLE)

| Variable | Description | Example |
|----------|-------------|---------|
| `airports` | Project name | `jokes` |
| `apimgr` | Organization name | `apimgr` |
| `{gitprovider}` | Git hosting provider | `github`, `gitlab`, `private` |
| **Rule** | Anything in `{}` is a variable | |
| **Rule** | Anything NOT in `{}` is literal | `/etc/letsencrypt/live` is a real path |

## Local Project Path Structure (NON-NEGOTIABLE)

**Format:** `~/Projects/{gitprovider}/apimgr/airports`

| Component | Description | Examples |
|-----------|-------------|----------|
| `~/Projects/` | Base projects directory | Always `~/Projects/` |
| `{gitprovider}` | Git hosting provider or `local` | `github`, `gitlab`, `bitbucket`, `private`, `local` |
| `apimgr` | Organization/username | `apimgr`, `casjay`, `myorg` |
| `airports` | Project name | `jokes`, `icons`, `myproject` |

**Examples:**
```
~/Projects/github/apimgr/jokes        # GitHub, apimgr org, jokes project
~/Projects/gitlab/casjay/icons        # GitLab, casjay user, icons project
~/Projects/private/myorg/myproject    # Private/self-hosted git, myorg, myproject
~/Projects/bitbucket/company/app      # Bitbucket, company org, app project
~/Projects/local/apimgr/prototype     # Local only, no VCS, prototyping
```

### Special: `local` Provider

`~/Projects/local/apimgr/airports` is used for:
- **Prototyping** - Quick experiments and proof-of-concept
- **Bootstrapping** - Initial project setup before pushing to VCS
- **Local-only development** - Projects not intended for remote hosting
- **No VCS required** - May not have git initialized
- **No Docker registry** - May not push to container registries

**This is the LOCAL development path, not the deployed path.**

---

## Directory Structure (NON-NEGOTIABLE)

**The root Project directory is**: `./`

```
./                          # Root project directory
├── .github/                # GitHub Actions (if using GitHub)
│   └── workflows/
│       ├── release.yml     # Stable releases
│       ├── beta.yml        # Beta releases
│       ├── daily.yml       # Daily builds
│       └── docker.yml      # Docker images
├── .gitea/                 # Gitea Actions (if using Gitea)
│   └── workflows/
│       ├── release.yml     # Stable releases
│       ├── beta.yml        # Beta releases
│       ├── daily.yml       # Daily builds
│       └── docker.yml      # Docker images
├── src/                    # All source files
├── scripts/                # All production/install scripts
├── tests/                  # All development/test scripts and files
├── docker/                 # Docker files
│   ├── Dockerfile          # Production Dockerfile
│   ├── Dockerfile.dev      # Development Dockerfile (optional)
│   ├── docker-compose.yml  # Production compose
│   ├── docker-compose.dev.yml  # Development compose (optional)
│   └── rootfs/             # Container filesystem overlay
│       └── usr/
│           └── local/
│               └── bin/
│                   └── entrypoint.sh  # Container entrypoint
├── rootfs/                 # Runtime volume data (gitignored)
│   ├── config/             # Config volumes per service
│   ├── data/               # Data volumes per service
│   └── db/                 # Database volumes per type
├── binaries/               # Built binaries (gitignored) - ALL binaries here
├── releases/               # Release binaries (gitignored)
├── README.md               # Production first, dev last
├── LICENSE.md              # MIT + embedded licenses
├── AI.md                   # Project specification (from TEMPLATE.md)
├── TODO.AI.md              # Task tracking for >2 tasks
├── Jenkinsfile             # Jenkins pipeline
└── release.txt             # Version tracking
```

**Gitignored directories:**
- `binaries/` - All build output (host + all platforms)
- `releases/` - Release output
- `rootfs/` - Runtime volume data

### .gitignore (REQUIRED)

```gitignore
# Build output
binaries/
releases/

# Runtime volume data (NEVER commit)
rootfs/

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
```

### .dockerignore (REQUIRED)

```dockerignore
# Git
.git/
.gitignore

# GitHub/Gitea workflows
.github/
.gitea/

# Entire docker directory
docker/

# Runtime volume data (NEVER include in image)
rootfs/

# Build output
binaries/
releases/

# IDE
.idea/
.vscode/

# Docs
*.md
```

**Note:** `./docker/` is ignored but `COPY docker/rootfs/ /` still works because we build using host docker - files are copied from host filesystem during build, not from build context.

**RULE: Keep the base directory organized and clean - no clutter!**

## Path Convention (NON-NEGOTIABLE)

**ALL paths are ALWAYS relative to PROJECT ROOT, never the current working directory.**

| Path | Means | NOT |
|------|-------|-----|
| `./docker/` | `{project_root}/docker/` | `{cwd}/docker/` |
| `./binaries/` | `{project_root}/binaries/` | `{cwd}/binaries/` |
| `./src/` | `{project_root}/src/` | `{cwd}/src/` |

**Rules:**
- Commands MUST be run from project root, OR
- Commands MUST use absolute paths, OR
- Commands MUST `cd` to project root first
- NEVER assume current directory is project root
- Scripts should determine project root programmatically

```bash
# Determine project root (in scripts)
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Or for Go binaries, use os.Executable()
```

```go
// Determine project root in Go
func getProjectRoot() string {
    exe, _ := os.Executable()
    return filepath.Dir(exe)
}
```

**Example - WRONG vs RIGHT:**
```bash
# WRONG: Assumes cwd is project root
cd binaries && docker build -f ../docker/Dockerfile .

# RIGHT: Explicit project root
docker build -f ./docker/Dockerfile -t myapp ./

# RIGHT: cd to project root first
cd /path/to/project && docker build -f ./docker/Dockerfile .
```

## Runtime Directory Usage (NON-NEGOTIABLE)

**Be smart about which directory to use for what purpose.**

| Directory | Purpose | Examples |
|-----------|---------|----------|
| `{config_dir}` | User-editable configuration | `server.yml`, email templates, custom themes, SSL certs |
| `{data_dir}` | Application-managed data | databases, Tor keys, caches, GeoIP databases |
| `{log_dir}` | Log files | `access.log`, `error.log`, `audit.log` |
| `{backup_dir}` | Backup files | `.tar.gz` backup archives |

**Rules:**
- If a user might edit it → `{config_dir}`
- If the application manages it → `{data_dir}`
- If it's a log → `{log_dir}`
- If it's a backup → `{backup_dir}`
- **NEVER mix purposes** - don't put user config in data_dir or vice versa

---

## Platform Support (NON-NEGOTIABLE)

### Operating Systems

| OS | Required |
|----|----------|
| Linux | YES |
| BSD (FreeBSD, OpenBSD, etc.) | YES |
| macOS (Intel and Apple Silicon) | YES |
| Windows | YES |

### Architectures

| Architecture | Required |
|--------------|----------|
| AMD64 | YES |
| ARM64 | YES |

**IMPORTANT: Be smart about implementations - code must work on ALL platforms.**

---

## Go Version (NON-NEGOTIABLE)

| Rule | Description |
|------|-------------|
| **Always Latest Stable** | Use latest stable Go version |
| **Build Only** | Go is only for building, not runtime (single static binary) |
| **go.mod** | Always use latest stable version |
| **Docker** | Use `golang:alpine` for build/test/debug |
| **No Pinning** | Don't pin to minor versions unless compatibility issue |

## Required Go Libraries (NON-NEGOTIABLE)

**All libraries MUST be pure Go and work with `CGO_ENABLED=0`.**

| Purpose | Library | Why |
|---------|---------|-----|
| **SQLite** | `modernc.org/sqlite` | Pure Go, no CGO required |
| **Tor** | `github.com/cretz/bine` | Pure Go Tor controller |
| **UUID** | `github.com/google/uuid` | Standard UUID generation |

### SQLite Driver (NON-NEGOTIABLE)

**MUST use `modernc.org/sqlite`. NEVER use `github.com/mattn/go-sqlite3`.**

| Driver | CGO Required | Use |
|--------|--------------|-----|
| `modernc.org/sqlite` | NO | **ALWAYS USE THIS** |
| `github.com/mattn/go-sqlite3` | YES | **NEVER USE** |

**Why `modernc.org/sqlite`?**
- Pure Go implementation - no C compiler needed
- Works with `CGO_ENABLED=0` for static binaries
- Cross-compilation works without toolchain setup
- Same SQLite functionality, just pure Go

**Usage:**
```go
import (
    "database/sql"
    _ "modernc.org/sqlite"
)

func openDB(path string) (*sql.DB, error) {
    // Driver name is "sqlite" (not "sqlite3")
    return sql.Open("sqlite", path)
}
```

**go.mod:**
```
require modernc.org/sqlite v1.29.1
```

### Forbidden Libraries

| Library | Reason | Alternative |
|---------|--------|-------------|
| `github.com/mattn/go-sqlite3` | Requires CGO | `modernc.org/sqlite` |
| `github.com/ooni/go-libtor` | Requires CGO | `github.com/cretz/bine` + external Tor |
| Any CGO library | Breaks static builds | Find pure Go alternative |

## Password Hashing (NON-NEGOTIABLE)

**ALL passwords MUST be hashed using Argon2id. NEVER store plaintext passwords.**

### Algorithm Requirements

| Setting | Value | Reason |
|---------|-------|--------|
| **Algorithm** | Argon2id | Winner of Password Hashing Competition, memory-hard |
| **Library** | `golang.org/x/crypto/argon2` | Pure Go, CGO_ENABLED=0 compatible |
| **Fallback** | Bcrypt (cost 12+) | For legacy password verification only |

### Argon2id Parameters (OWASP 2023)

```go
import "golang.org/x/crypto/argon2"

// Recommended parameters
const (
    ArgonTime    = 3         // iterations
    ArgonMemory  = 64 * 1024 // 64 MB
    ArgonThreads = 4         // parallelism
    ArgonKeyLen  = 32        // output length in bytes
    ArgonSaltLen = 16        // salt length in bytes
)

func HashPassword(password string) (string, error) {
    // Generate random salt
    salt := make([]byte, ArgonSaltLen)
    if _, err := rand.Read(salt); err != nil {
        return "", err
    }

    // Hash password
    hash := argon2.IDKey([]byte(password), salt, ArgonTime, ArgonMemory, ArgonThreads, ArgonKeyLen)

    // Encode as string: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
    return encodeArgon2Hash(salt, hash), nil
}
```

### Storage Format

Passwords stored in PHC string format:
```
$argon2id$v=19$m=65536,t=3,p=4$<base64-salt>$<base64-hash>
```

### Password Rules

| Rule | Description |
|------|-------------|
| **NEVER** | Store plaintext passwords anywhere |
| **NEVER** | Store passwords in config files (server.yml) |
| **NEVER** | Log passwords (even hashed) |
| **ALWAYS** | Use Argon2id for new passwords |
| **ALWAYS** | Store in database only |
| **ALWAYS** | Generate secure random salt per password |

### API Token Hashing

API tokens are also sensitive and MUST be hashed:

| Token Type | Storage | Hashing |
|------------|---------|---------|
| **API Token** | Database | SHA-256 hash (fast lookup needed) |
| **Session Token** | Database | SHA-256 hash |
| **Password** | Database | Argon2id (slow by design) |

```go
import "crypto/sha256"

func HashToken(token string) string {
    hash := sha256.Sum256([]byte(token))
    return hex.EncodeToString(hash[:])
}
```

**Note:** API tokens use SHA-256 (not Argon2id) because:
- Tokens are already high-entropy random strings
- Need fast lookup for every API request
- Argon2id's slowness is for weak human passwords

---

# CHECKPOINT 2: PROJECT STRUCTURE VERIFICATION

Before proceeding, confirm you understand:
- [ ] Project directory structure
- [ ] Variable syntax (`{}` = variable, no `{}` = literal)
- [ ] All 4 OSes must be supported
- [ ] Both AMD64 and ARM64 must be supported
- [ ] Always use latest stable Go

---

# PART 3: OS-SPECIFIC PATHS (NON-NEGOTIABLE)

## Linux

### Privileged (root/sudo)

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/airports` |
| Config | `/etc/apimgr/airports/` |
| Config File | `/etc/apimgr/airports/server.yml` |
| Data | `/var/lib/apimgr/airports/` |
| Logs | `/var/log/apimgr/airports/` |
| Backup | `/mnt/Backups/apimgr/airports/` |
| PID File | `/var/run/apimgr/airports.pid` |
| SSL | `/etc/apimgr/airports/ssl/` (letsencrypt/, local/) |
| SQLite DB | `/var/lib/apimgr/airports/db/` |
| GeoIP | `/var/lib/apimgr/airports/geoip/` |
| Service | `/etc/systemd/system/airports.service` |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `~/.local/bin/airports` |
| Config | `~/.config/apimgr/airports/` |
| Config File | `~/.config/apimgr/airports/server.yml` |
| Data | `~/.local/share/apimgr/airports/` |
| Logs | `~/.local/share/apimgr/airports/logs/` |
| Backup | `~/.local/backups/apimgr/airports/` |
| PID File | `~/.local/share/apimgr/airports/airports.pid` |
| SSL | `~/.config/apimgr/airports/ssl/` (letsencrypt/, local/) |
| SQLite DB | `~/.local/share/apimgr/airports/db/` |
| GeoIP | `~/.local/share/apimgr/airports/geoip/` |

---

## macOS

### Privileged (root/sudo)

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/airports` |
| Config | `/Library/Application Support/apimgr/airports/` |
| Config File | `/Library/Application Support/apimgr/airports/server.yml` |
| Data | `/Library/Application Support/apimgr/airports/data/` |
| Logs | `/Library/Logs/apimgr/airports/` |
| Backup | `/Library/Backups/apimgr/airports/` |
| PID File | `/var/run/apimgr/airports.pid` |
| SSL | `/Library/Application Support/apimgr/airports/ssl/` (letsencrypt/, local/) |
| SQLite DB | `/Library/Application Support/apimgr/airports/db/` |
| GeoIP | `/Library/Application Support/apimgr/airports/geoip/` |
| Service | `/Library/LaunchDaemons/com.apimgr.airports.plist` |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `~/bin/airports` or `/usr/local/bin/airports` |
| Config | `~/Library/Application Support/apimgr/airports/` |
| Config File | `~/Library/Application Support/apimgr/airports/server.yml` |
| Data | `~/Library/Application Support/apimgr/airports/` |
| Logs | `~/Library/Logs/apimgr/airports/` |
| Backup | `~/Library/Backups/apimgr/airports/` |
| PID File | `~/Library/Application Support/apimgr/airports/airports.pid` |
| SSL | `~/Library/Application Support/apimgr/airports/ssl/` (letsencrypt/, local/) |
| SQLite DB | `~/Library/Application Support/apimgr/airports/db/` |
| GeoIP | `~/Library/Application Support/apimgr/airports/geoip/` |
| Service | `~/Library/LaunchAgents/com.apimgr.airports.plist` |

---

## BSD (FreeBSD, OpenBSD, NetBSD)

### Privileged (root/sudo/doas)

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/airports` |
| Config | `/usr/local/etc/apimgr/airports/` |
| Config File | `/usr/local/etc/apimgr/airports/server.yml` |
| Data | `/var/db/apimgr/airports/` |
| Logs | `/var/log/apimgr/airports/` |
| Backup | `/var/backups/apimgr/airports/` |
| PID File | `/var/run/apimgr/airports.pid` |
| SSL | `/usr/local/etc/apimgr/airports/ssl/` (letsencrypt/, local/) |
| SQLite DB | `/var/db/apimgr/airports/db/` |
| GeoIP | `/var/db/apimgr/airports/geoip/` |
| Service | `/usr/local/etc/rc.d/airports` |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `~/.local/bin/airports` |
| Config | `~/.config/apimgr/airports/` |
| Config File | `~/.config/apimgr/airports/server.yml` |
| Data | `~/.local/share/apimgr/airports/` |
| Logs | `~/.local/share/apimgr/airports/logs/` |
| Backup | `~/.local/backups/apimgr/airports/` |
| PID File | `~/.local/share/apimgr/airports/airports.pid` |
| SSL | `~/.config/apimgr/airports/ssl/` (letsencrypt/, local/) |
| SQLite DB | `~/.local/share/apimgr/airports/db/` |
| GeoIP | `~/.local/share/apimgr/airports/geoip/` |

---

## Windows

### Privileged (Administrator)

| Type | Path |
|------|------|
| Binary | `C:\Program Files\apimgr\airports\airports.exe` |
| Config | `%ProgramData%\apimgr\airports\` |
| Config File | `%ProgramData%\apimgr\airports\server.yml` |
| Data | `%ProgramData%\apimgr\airports\data\` |
| Logs | `%ProgramData%\apimgr\airports\logs\` |
| Backup | `%ProgramData%\Backups\apimgr\airports\` |
| SSL | `%ProgramData%\apimgr\airports\ssl\` (letsencrypt\, local\) |
| SQLite DB | `%ProgramData%\apimgr\airports\db\` |
| GeoIP | `%ProgramData%\apimgr\airports\geoip\` |
| Service | Windows Service Manager |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `%LocalAppData%\apimgr\airports\airports.exe` |
| Config | `%AppData%\apimgr\airports\` |
| Config File | `%AppData%\apimgr\airports\server.yml` |
| Data | `%LocalAppData%\apimgr\airports\` |
| Logs | `%LocalAppData%\apimgr\airports\logs\` |
| Backup | `%LocalAppData%\Backups\apimgr\airports\` |
| SSL | `%AppData%\apimgr\airports\ssl\` (letsencrypt\, local\) |
| SQLite DB | `%LocalAppData%\apimgr\airports\db\` |
| GeoIP | `%LocalAppData%\apimgr\airports\geoip\` |

---

## Docker/Container

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/airports` |
| Config | `/config/` |
| Config File | `/config/server.yml` |
| Data | `/data/` |
| Logs | `/data/logs/` |
| SQLite DB | `/data/db/` |
| GeoIP | `/data/geoip/` |
| Internal Port | `80` |

---

# CHECKPOINT 3: PATH VERIFICATION

Before proceeding, confirm you understand:
- [ ] Each OS has specific paths for privileged and non-privileged users
- [ ] Config file is ALWAYS `server.yml` (not .yaml)
- [ ] Docker uses simplified paths (/config, /data)
- [ ] All paths follow the apimgr/airports pattern

---

# PART 4: PRIVILEGE ESCALATION & USER CREATION (NON-NEGOTIABLE)

## Overview

Application user creation **REQUIRES** privilege escalation. If the user cannot escalate privileges, the application runs as the current user with user-level directories.

## Escalation Detection by OS

### Linux
```
Escalation Methods (in order):
1. Already root (EUID == 0)
2. sudo (if user is in sudoers/wheel group)
3. su (if user knows root password)
4. pkexec (PolicyKit, if available)
5. doas (OpenBSD-style, if configured)
```

### macOS
```
Escalation Methods (in order):
1. Already root (EUID == 0)
2. sudo (user must be in admin group)
3. osascript with administrator privileges (GUI prompt)
```

### BSD
```
Escalation Methods (in order):
1. Already root (EUID == 0)
2. doas (OpenBSD default, others if configured)
3. sudo (if installed and configured)
4. su (if user knows root password)
```

### Windows
```
Escalation Methods (in order):
1. Already Administrator (elevated token)
2. UAC prompt (requires GUI)
3. runas (command line, requires admin password)
```

## User Creation Logic

```
ON --service --install:

1. Check if can escalate privileges
   ├─ YES: Continue with privileged installation
   │   ├─ Create system user/group (UID/GID 100-999)
   │   ├─ Use system directories (/etc, /var/lib, /var/log)
   │   ├─ Install service (systemd/launchd/rc.d/Windows Service)
   │   └─ Set ownership to created user
   │
   └─ NO: Fall back to user installation
       ├─ Skip user creation (run as current user)
       ├─ Use user directories (~/.config, ~/.local/share)
       ├─ Skip system service installation
       └─ Offer alternative (cron, user systemd, launchctl user agent)
```

## System User Requirements (NON-NEGOTIABLE)

| Requirement | Value |
|-------------|-------|
| Username | `airports` |
| Group | `airports` |
| UID/GID | **Must match** - same value for both UID and GID |
| UID/GID Range | 100-999 (system user range) |
| Shell | `/sbin/nologin` or `/usr/sbin/nologin` |
| Home | Config directory (`/etc/apimgr/airports`) or data directory (`/var/lib/apimgr/airports`) |
| Type | System user (no password, no login) |
| Gecos | `airports service account` |

### UID/GID Selection Logic

**The UID and GID MUST be the same value.** Find an unused ID where both UID and GID are available:

```
1. Start at 999 (top of system range)
2. Check if UID is unused (not in /etc/passwd)
3. Check if GID is unused (not in /etc/group)
4. If both available → use this value for both UID and GID
5. If either taken → decrement and repeat
6. Stop at 100 (bottom of system range)
7. If no ID found → error (system has no available IDs)
```

### Go Implementation

```go
func findAvailableSystemID() (int, error) {
    // Start from top of system range, work down
    for id := 999; id >= 100; id-- {
        // Check if UID is available
        if _, err := user.LookupId(strconv.Itoa(id)); err == nil {
            continue
            // UID exists, try next
        }

        // Check if GID is available
        if _, err := user.LookupGroupId(strconv.Itoa(id)); err == nil {
            continue
            // GID exists, try next
        }

        // Both available
        return id, nil
    }
    return 0, fmt.Errorf("no available UID/GID in system range 100-999")
}
```

### Platform-Specific Commands

**Linux:**
```bash
# Create group with specific GID
groupadd --system --gid {id} airports

# Create user with matching UID, same primary group
useradd --system --uid {id} --gid {id} \
  --home-dir /etc/apimgr/airports \
  --shell /sbin/nologin \
  --comment "airports service account" \
  airports
```

### macOS Service Account (NON-NEGOTIABLE)

**macOS services (launchd) MUST NOT run as logged-in user or root.**

| Option | Security | Recommendation |
|--------|----------|----------------|
| root | HIGH privileges - dangerous | NO |
| Logged-in User | User privileges - insecure | NO |
| `_www` | Web server account | NO (wrong purpose) |
| **Dedicated service user** | Minimal privileges, isolated | **RECOMMENDED** |

**Default: Dedicated system user with matching UID/GID**

macOS uses `dscl` (Directory Service Command Line) to create system users. The user is hidden from login screen and has no shell access.

**macOS UID/GID Ranges:**

| Range | Purpose |
|-------|---------|
| 0-99 | System accounts (reserved) |
| 100-499 | System services (use this range) |
| 500+ | Regular users |

**Create Service Account:**
```bash
# Find available ID in 100-499 range (same logic as Linux but different range)
# Start at 499, work down until both UID and GID are available

# Create group with specific GID
dscl . -create /Groups/airports
dscl . -create /Groups/airports PrimaryGroupID {id}
dscl . -create /Groups/airports Password "*"

# Create user with matching UID
dscl . -create /Users/airports
dscl . -create /Users/airports UniqueID {id}
dscl . -create /Users/airports PrimaryGroupID {id}
dscl . -create /Users/airports UserShell /usr/bin/false
dscl . -create /Users/airports RealName "airports service account"
dscl . -create /Users/airports NFSHomeDirectory /usr/local/var/apimgr/airports
dscl . -create /Users/airports Password "*"

# Hide user from login window
dscl . -create /Users/airports IsHidden 1
```

**launchd plist with UserName:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>apimgr.airports</string>

    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/airports</string>
    </array>

    <!-- Run as dedicated service user, NOT root -->
    <key>UserName</key>
    <string>airports</string>

    <key>GroupName</key>
    <string>airports</string>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>WorkingDirectory</key>
    <string>/usr/local/var/apimgr/airports</string>

    <key>StandardOutPath</key>
    <string>/usr/local/var/log/apimgr/airports/stdout.log</string>

    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/apimgr/airports/stderr.log</string>
</dict>
</plist>
```

**macOS Directory Structure:**

| Directory | Path | Purpose |
|-----------|------|---------|
| Binary | `/usr/local/bin/airports` | Executable |
| Config | `/usr/local/etc/apimgr/airports/` | Configuration files |
| Data | `/usr/local/var/apimgr/airports/` | Application data |
| Logs | `/usr/local/var/log/apimgr/airports/` | Log files |
| launchd plist | `/Library/LaunchDaemons/apimgr.airports.plist` | Service definition |

**Go Implementation (macOS):**
```go
// +build darwin

func findAvailableMacOSSystemID() (int, error) {
    // macOS system services use 100-499 range
    for id := 499; id >= 100; id-- {
        // Check if UID is available
        if _, err := user.LookupId(strconv.Itoa(id)); err == nil {
            continue
        }

        // Check if GID is available
        if _, err := user.LookupGroupId(strconv.Itoa(id)); err == nil {
            continue
        }

        return id, nil
    }
    return 0, fmt.Errorf("no available UID/GID in macOS system range 100-499")
}

func createMacOSServiceUser(name string, id int, homeDir string) error {
    commands := [][]string{
        // Create group
        {"dscl", ".", "-create", "/Groups/" + name},
        {"dscl", ".", "-create", "/Groups/" + name, "PrimaryGroupID", strconv.Itoa(id)},
        {"dscl", ".", "-create", "/Groups/" + name, "Password", "*"},
        // Create user
        {"dscl", ".", "-create", "/Users/" + name},
        {"dscl", ".", "-create", "/Users/" + name, "UniqueID", strconv.Itoa(id)},
        {"dscl", ".", "-create", "/Users/" + name, "PrimaryGroupID", strconv.Itoa(id)},
        {"dscl", ".", "-create", "/Users/" + name, "UserShell", "/usr/bin/false"},
        {"dscl", ".", "-create", "/Users/" + name, "RealName", name + " service account"},
        {"dscl", ".", "-create", "/Users/" + name, "NFSHomeDirectory", homeDir},
        {"dscl", ".", "-create", "/Users/" + name, "Password", "*"},
        {"dscl", ".", "-create", "/Users/" + name, "IsHidden", "1"},
    }

    for _, cmd := range commands {
        if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
            return fmt.Errorf("failed to run %v: %w", cmd, err)
        }
    }
    return nil
}
```

**FreeBSD:**
```bash
# Create user and group with matching ID
pw groupadd -n airports -g {id}
pw useradd -n airports -u {id} -g {id} \
  -d /var/lib/apimgr/airports \
  -s /usr/sbin/nologin \
  -c "airports service account"
```

### Windows Service Account (NON-NEGOTIABLE)

**Windows services MUST NOT run as logged-in user or Administrator.**

| Option | Security | Recommendation |
|--------|----------|----------------|
| Local System | HIGH privileges - dangerous | NO |
| Administrator | HIGH privileges - dangerous | NO |
| Logged-in User | User privileges - insecure | NO |
| Local Service | Limited privileges | OK for network-less services |
| Network Service | Limited + network access | OK for network services |
| **Virtual Service Account** | Minimal privileges, isolated | **RECOMMENDED** |
| Managed Service Account | Domain-managed, auto-password | Enterprise environments |

**Default: Virtual Service Account (VSA)**

Virtual Service Accounts are automatically managed by Windows, require no password management, and have minimal privileges. They are created automatically when the service is installed.

**Service Account Format:** `NT SERVICE\airports`

```powershell
# Create service with Virtual Service Account (automatic)
New-Service -Name "airports" `
  -BinaryPathName "C:\Program Files\apimgr\airports\airports.exe" `
  -DisplayName "airports" `
  -Description "airports service" `
  -StartupType Automatic

# Service automatically runs as NT SERVICE\airports
# No user creation needed - Windows manages it
```

**Directory Permissions:**
```powershell
# Grant Virtual Service Account access to config/data directories
$acl = Get-Acl "C:\ProgramData\apimgr\airports"
$rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
    "NT SERVICE\airports",
    "FullControl",
    "ContainerInherit,ObjectInherit",
    "None",
    "Allow"
)
$acl.SetAccessRule($rule)
Set-Acl "C:\ProgramData\apimgr\airports" $acl
```

**Go Implementation (Windows):**
```go
// +build windows

import "golang.org/x/sys/windows/svc/mgr"

func installWindowsService() error {
    m, err := mgr.Connect()
    if err != nil {
        return err
    }
    defer m.Disconnect()

    exePath, _ := os.Executable()

    // Create service - runs as Virtual Service Account by default
    // when ServiceStartName is empty or "NT SERVICE\{name}"
    s, err := m.CreateService(
        "airports",
        exePath,
        mgr.Config{
            DisplayName:     "airports",
            Description:     "airports service",
            StartType:       mgr.StartAutomatic,
            ServiceStartName: "", // Empty = Virtual Service Account
        },
    )
    if err != nil {
        return err
    }
    defer s.Close()

    return nil
}
```

**Windows Directory Structure:**

| Directory | Path | Purpose |
|-----------|------|---------|
| Binary | `C:\Program Files\apimgr\airports\` | Executable |
| Config | `C:\ProgramData\apimgr\airports\config\` | Configuration files |
| Data | `C:\ProgramData\apimgr\airports\data\` | Application data |
| Logs | `C:\ProgramData\apimgr\airports\logs\` | Log files |

### Home Directory Selection

| Directory | Use When |
|-----------|----------|
| Config dir (`/etc/apimgr/airports`) | Default - user needs access to config files |
| Data dir (`/var/lib/apimgr/airports`) | When data dir contains user-writable content |

**Note:** Home directory must exist before user creation. Create directories first, then user, then set ownership.

---

# PART 5: SERVICE SUPPORT (NON-NEGOTIABLE)

## Built-in Service Support

**ALL projects MUST have built-in service support for ALL service managers:**

| Service Manager | OS |
|-----------------|-----|
| systemd | Linux |
| runit | Linux |
| Windows Service Manager | Windows |
| launchd | macOS |
| rc.d | BSD |

---

# PART 6: CONFIGURATION (NON-NEGOTIABLE)

## Configuration Storage

**Configuration can be stored in `server.yml` OR database, depending on mode.**

### Terminology

| Term | Meaning |
|------|---------|
| **server.yml** | YAML configuration file on disk |
| **Configuration** | Settings (stored in server.yml OR database) |
| **Database** | SQLite (local) or PostgreSQL/MySQL (remote) |
| **Server Address** | The bind address for the server (e.g., `[::]`, `0.0.0.0`, `127.0.0.1`) |
| **FQDN** | Fully Qualified Domain Name (e.g., `api.example.com`) |
| **Node ID** | Unique identifier for a cluster node (default: hostname) |

**IMPORTANT:** We use "Server Address" (bind address), NOT "Server Name". The server address is where the server listens; the FQDN is how clients reach it.

### Configuration Source of Truth

| Mode | Source of Truth | server.yml Role |
|------|-----------------|-----------------|
| **Single Instance (SQLite)** | server.yml | Primary configuration |
| **Cluster Mode (Remote DB)** | Database | Bootstrap only (connection settings) |

### Single Instance Mode

```
server.yml (source of truth)
     │
     ▼
Application reads config
     │
     ▼
Admin panel writes to server.yml
```

- All settings stored in `server.yml`
- Admin panel edits `server.yml` directly
- SQLite databases for credentials/sessions only

### Cluster Mode

```
server.yml (cache + backup)
     │
     └─ Contains: database connection + cached config

Database (source of truth)
     │
     ▼
Application reads config from DB
     │
     ▼
Admin panel writes to database
     │
     ▼
Changes synced to server.yml cache
     │
     ▼
All nodes see changes immediately
```

- Database is source of truth
- `server.yml` is cache AND backup
- Admin panel writes to database
- Changes automatically synced to local `server.yml`
- If database unavailable → read-only mode using cached config

### server.yml as Cache/Backup (NON-NEGOTIABLE)

**When using external database, server.yml becomes a local cache and backup.**

```
Normal Operation:
┌──────────────┐         ┌──────────────┐
│   Database   │◄───────►│  server.yml  │
│ (source of   │  sync   │   (cache)    │
│   truth)     │         │              │
└──────────────┘         └──────────────┘
       │
       ▼
  Application
  reads from DB

Database Unavailable:
┌──────────────┐         ┌──────────────┐
│   Database   │    ✗    │  server.yml  │
│   (DOWN)     │         │   (backup)   │
└──────────────┘         └──────────────┘
                               │
                               ▼
                         Application
                         READ-ONLY MODE
                         uses cached config
```

### Config Sync (Database → server.yml)

**Every config change in database is synced to local server.yml:**

```go
func onConfigChange(db *sql.DB, key string, value interface{}) {
    // 1. Write to database (source of truth)
    writeToDatabase(db, key, value)

    // 2. Sync to local server.yml (cache)
    syncToLocalConfig(key, value)

    // 3. Update last_sync timestamp
    updateSyncTimestamp()
}
```

**Sync happens:**
- Immediately after any config change
- On application startup (DB → server.yml)
- Periodically (every 5 minutes) to catch any drift

### Maintenance Mode (NON-NEGOTIABLE)

**Only TWO types of errors are truly critical:**
1. Database connection error
2. Cannot write to files (disk full, permissions, etc.)

**All other errors are recoverable. The system ALWAYS attempts self-healing.**

### Critical Error Detection

| Error Type | Detection | Self-Healing Attempt |
|------------|-----------|---------------------|
| **Database connection** | Connection refused, timeout, auth failed | Retry with backoff, check credentials |
| **Database write** | Write failed, transaction error | Retry, check permissions, check disk |
| **File read** | Permission denied, file not found | Check permissions, recreate if possible |
| **File write** | Permission denied, disk full | Check permissions, check disk space, cleanup |

### Mode States

| State | Condition | Behavior |
|-------|-----------|----------|
| **Normal** | All systems healthy | Full functionality |
| **Maintenance** | Critical error detected | Read-only + admin guidance |
| **Starting** | Application starting up | Checking systems |

### Maintenance Mode

**When a critical error occurs, the application enters maintenance mode:**

| Aspect | Behavior |
|--------|----------|
| **Public API** | Read-only operations only |
| **Admin panel** | Accessible with fix instructions |
| **Writes** | Rejected with 503 |
| **Self-healing** | Continuously attempting in background |
| **Recovery** | Automatic when issue resolved |

### Self-Healing Process

```
Critical Error Detected
         │
         ▼
Enter Maintenance Mode
         │
         ├──────────────────────────────────────┐
         ▼                                      │
Attempt Self-Healing                            │
         │                                      │
         ├─► Database connection failed         │
         │   1. Retry connection (3x backoff)   │
         │   2. Check if host reachable         │
         │   3. Verify credentials              │
         │   4. Test with simple query          │
         │                                      │
         ├─► File write failed                  │
         │   1. Check disk space                │
         │   2. Check directory permissions     │
         │   3. Attempt to create test file     │
         │   4. Try alternate location          │
         │                                      │
         ├─► Disk full                          │
         │   1. Cleanup old logs                │
         │   2. Cleanup temp files              │
         │   3. Cleanup old backups             │
         │   4. Report space freed              │
         │                                      │
         ▼                                      │
Self-Healing Successful?                        │
    │                                           │
    ├─► YES: Exit Maintenance Mode ─────────────┘
    │        Resume Normal Operation
    │
    └─► NO: Continue in Maintenance Mode
            Retry every 30 seconds
            Show fix instructions in admin UI
```

### Admin Panel in Maintenance Mode

**The admin panel remains accessible and provides guidance for fixing issues.**

#### Maintenance Dashboard (`/admin`)

```
┌─────────────────────────────────────────────────────────────┐
│  ⚠️  MAINTENANCE MODE                                        │
│                                                             │
│  The application is running in read-only maintenance mode   │
│  due to a critical error. Self-healing is in progress.      │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Error: Database Connection Failed                          │
│  ─────────────────────────────────────────                  │
│  Host: db.example.com:5432                                  │
│  Error: connection refused                                  │
│  Last successful connection: 5 minutes ago                  │
│                                                             │
│  Self-Healing Status: Retrying... (attempt 15)              │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  📋 Suggested Actions:                                      │
│                                                             │
│  1. Check if database server is running                     │
│     └─ ssh db.example.com "systemctl status postgresql"     │
│                                                             │
│  2. Verify network connectivity                             │
│     └─ ping db.example.com                                  │
│     └─ telnet db.example.com 5432                           │
│                                                             │
│  3. Check database credentials                              │
│     └─ Verify password in server.yml or environment         │
│                                                             │
│  4. Check database logs                                     │
│     └─ ssh db.example.com "tail /var/log/postgresql/*.log"  │
│                                                             │
│  [Test Connection]  [View Full Diagnostics]                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### Error-Specific Guidance

**Database Connection Failed:**

| Check | Command/Action | What to Look For |
|-------|----------------|------------------|
| Server running | `systemctl status postgresql` | Active (running) |
| Network | `ping db.example.com` | Response |
| Port open | `telnet db.example.com 5432` | Connected |
| Credentials | Check server.yml | Correct password |
| Max connections | Check DB logs | "too many connections" |
| Firewall | Check iptables/ufw | Port 5432 allowed |

**Disk Full:**

| Check | Command/Action | What to Look For |
|-------|----------------|------------------|
| Disk space | `df -h` | Usage % |
| Large files | `du -sh /var/log/*` | Oversized logs |
| Old backups | Check backup directory | Old files to delete |
| Temp files | Check /tmp | Cleanup candidates |

**Auto-cleaned:**
- Logs older than retention policy
- Temp files older than 24 hours
- Old backup files (keeps last 5)

**Permission Denied:**

| Check | Command/Action | What to Look For |
|-------|----------------|------------------|
| Directory owner | `ls -la /path/to/dir` | Correct user:group |
| Directory perms | `stat /path/to/dir` | Write permission |
| SELinux/AppArmor | `getenforce` / `aa-status` | Blocking access |
| Disk mounted | `mount` | Read-only mount |

### API Responses in Maintenance Mode

**All write operations return:**

```json
{
  "error": "Service in maintenance mode",
  "code": "MAINTENANCE_MODE",
  "status": 503,
  "message": "System is in maintenance mode due to: Database connection failed",
  "reason": "database_connection",
  "self_healing": true,
  "retry_after": 30
}
```

**Headers:**
```
Retry-After: 30
X-Maintenance-Mode: true
X-Maintenance-Reason: database_connection
```

### /healthz in Maintenance Mode

```json
{
  "status": "maintenance",
  "version": "1.0.0",
  "mode": "maintenance",
  "uptime": "2d 5h 30m",
  "maintenance": {
    "reason": "database_connection",
    "message": "Database connection failed",
    "since": "2025-01-15T10:30:00Z",
    "self_healing": {
      "enabled": true,
      "attempts": 15,
      "last_attempt": "2025-01-15T10:35:00Z",
      "next_attempt": "2025-01-15T10:35:30Z"
    }
  },
  "checks": {
    "database": "error",
    "disk": "ok",
    "config": "ok"
  }
}
```

### Recovery (Automatic)

**When self-healing succeeds:**

```
1. Self-healing attempt succeeds
         │
         ▼
2. Verify system is stable
   - Run health checks
   - Verify read/write works
         │
         ▼
3. Log: "Issue resolved, exiting maintenance mode"
         │
         ▼
4. Exit maintenance mode
   - Set mode = normal
   - Remove maintenance banner
   - Re-enable write operations
         │
         ▼
5. Send notification (if email configured)
   - "System recovered from maintenance mode"
   - Include duration and cause
         │
         ▼
6. Resume normal operation
```

### Maintenance Mode Config

```yaml
server:
  maintenance:
    # Self-healing settings
    self_healing:
      enabled: true
      retry_interval: 30
      # seconds between retry attempts
      max_attempts: 0
      # 0 = unlimited (keep trying forever)

    # Auto-cleanup thresholds
    cleanup:
      disk_threshold: 90
      # Start cleanup when disk > 90% full
      log_retention_days: 7
      # Delete logs older than 7 days during cleanup
      backup_keep_count: 5
      # Keep last 5 backups during cleanup

    # Notifications
    notify:
      on_enter: true
      # Notify when entering maintenance mode
      on_exit: true
      # Notify when exiting maintenance mode
```

### What Goes Where

| Setting | Single Instance | Cluster Mode | Read-Only Fallback |
|---------|-----------------|--------------|-------------------|
| Database connection | server.yml | server.yml | server.yml |
| Admin credentials | Local SQLite | Remote DB | Cached in server.yml |
| Server settings | server.yml | Remote DB | Cached in server.yml |
| Branding/SEO | server.yml | Remote DB | Cached in server.yml |
| SSL settings | server.yml | Remote DB | Cached in server.yml |
| User accounts | Local SQLite | Remote DB | ❌ Unavailable |
| Sessions | Local SQLite | Remote DB | ❌ Unavailable |
| API tokens | Local SQLite | Remote DB | ❌ Unavailable |

### server.yml Structure (Cluster Mode with Cache)

```yaml
# Database connection (always present)
server:
  database:
    driver: postgres
    host: db.example.com
    port: 5432
    name: myapp
    username: myapp
    password: ${DB_PASSWORD}
    sslmode: require

# Cached configuration (synced from database)
# Used as backup when database unavailable
_cache:
  last_sync: "2025-01-15T10:30:00Z"

  admin:
    email: admin@example.com
    # Note: password NOT cached (security)

  branding:
    title: "My Application"
    tagline: "The best app ever"

  ssl:
    enabled: true
    letsencrypt:
      enabled: true
      email: admin@example.com

  # ... all other settings cached here
```

### Database Schema for Configuration

**Configuration Table (in remote database):**

| Column | Type | Description |
|--------|------|-------------|
| `key` | String | Config key (e.g., "branding.title") |
| `value` | JSON | Config value |
| `updated_at` | Timestamp | Last update time |
| `updated_by` | String | Node ID or "admin" |

**Example data:**

| key | value | updated_at |
|-----|-------|------------|
| `branding.title` | `"My Application"` | 2025-01-15 10:30:00 |
| `branding.tagline` | `"The best app"` | 2025-01-15 10:30:00 |
| `ssl.enabled` | `true` | 2025-01-15 09:00:00 |
| `ssl.letsencrypt.enabled` | `true` | 2025-01-15 09:00:00 |
| `rate_limit.enabled` | `true` | 2025-01-14 15:00:00 |
| `rate_limit.requests` | `120` | 2025-01-14 15:00:00 |

**Cluster State Table:**

| Column | Type | Description |
|--------|------|-------------|
| `key` | String | State key |
| `value` | JSON | State value |
| `node_id` | String | Node that owns this state (or NULL for global) |
| `updated_at` | Timestamp | Last update |

**Example state data:**

| key | value | node_id | updated_at |
|-----|-------|---------|------------|
| `cluster.id` | `"cluster_abc123"` | NULL | 2025-01-10 |
| `cluster.name` | `"Production"` | NULL | 2025-01-10 |
| `encryption.key` | `"encrypted..."` | NULL | 2025-01-10 |
| `tor.onion_address` | `"abc...xyz.onion"` | `node-1` | 2025-01-15 |
| `tor.onion_address` | `"def...uvw.onion"` | `node-2` | 2025-01-15 |

### Full Database Schema Summary

**Server Tables (server_* prefix):**

| Table | Purpose |
|-------|---------|
| `server_admin_credentials` | Admin username/password/token |
| `server_admin_sessions` | Admin web sessions |
| `server_config` | All configuration key-value pairs |
| `server_cluster_state` | Cluster-wide and per-node state |
| `server_scheduler_state` | Scheduler task state |
| `server_nodes` | Registered cluster nodes |
| `server_join_tokens` | Node join tokens |

**User Tables (user_* prefix):**

| Table | Purpose |
|-------|---------|
| `user_accounts` | User accounts |
| `user_tokens` | User API tokens |
| `user_sessions` | User web sessions |
| `user_invites` | Invitation codes |

## Boolean Handling (NON-NEGOTIABLE)

**Accept ALL of these values for booleans:**

| Truthy | Falsy |
|--------|-------|
| `1` | `0` |
| `yes` | `no` |
| `true` | `false` |
| `enable` | `disable` |
| `enabled` | `disabled` |
| `on` | `off` |

**Internally convert all to `true` or `false`.**

## Environment Variables (NON-NEGOTIABLE)

### Runtime Variables (Always Checked)

| Variable | Description |
|----------|-------------|
| `DOMAIN` | FQDN override (highest priority for hostname resolution) |
| `MODE` | `production` (default) or `development` |
| `DATABASE_DRIVER` | `file`, `sqlite`, `mariadb`, `mysql`, `postgres`, `mssql`, `mongodb` |
| `DATABASE_URL` | Database connection string |

**FQDN Resolution Order:** `Reverse Proxy Headers` → `DOMAIN` → `os.Hostname()` → `$HOSTNAME` → Global IPv6 → Global IPv4 → `localhost`

**Note:** Loopback addresses (`localhost`, `127.0.0.1`, `::1`) are avoided; global IPs preferred.

### Init-Only Variables (First Run Only)

| Variable | Description |
|----------|-------------|
| `CONFIG_DIR` | Configuration directory |
| `DATA_DIR` | Data directory |
| `LOG_DIR` | Log directory |
| `BACKUP_DIR` | Backup directory |
| `DATABASE_DIR` | SQLite database directory |
| `PORT` | Server port |
| `LISTEN` | Listen address |
| `APPLICATION_NAME` | Application title |
| `APPLICATION_TAGLINE` | Application description |

**Init-only variables are used ONCE during first run, then ignored.**

---

## Configuration File (NON-NEGOTIABLE)

### Design Rules

| Rule | Description |
|------|-------------|
| **Clean & Intuitive** | Easy to read and understand |
| **Everything Configurable** | If it has a setting, it's in the config |
| **Sane Defaults** | Built-in defaults (no 1000-line configs) |
| **Comprehensive** | All options present (commented/defaulted) |
| **Comments** | Single-line, under 140 characters |

### Location

| User Type | Path |
|-----------|------|
| Root | `/etc/apimgr/airports/server.yml` |
| Regular | `~/.config/apimgr/airports/server.yml` |

### Migration

**If `server.yaml` found, auto-migrate to `server.yml` on startup.**

## Port Rules (NON-NEGOTIABLE)

**Default port is a random unused port in the 64000-64999 range.**

### Port Selection Logic

| Scenario | Port | Behavior |
|----------|------|----------|
| First run (no config) | Random 64xxx | Auto-select unused port in 64000-64999, **save to config** |
| Config specifies port | Use specified | Use exact port from config |
| Port in use | Error | Fail with clear error message |
| Privileged port (<1024) | Requires root | Warn if not running as root |

**IMPORTANT: Once a port is selected (randomly or specified), it is saved to `server.yml` and persists across restarts. The random selection only happens on first run when no port is configured.**

### Why Random 64xxx?

| Reason | Description |
|--------|-------------|
| **Avoid conflicts** | Most services use well-known ports; 64xxx is rarely used |
| **No root required** | Ports >1024 don't need root privileges |
| **Self-hosted friendly** | Users can run multiple instances without conflict |
| **Reverse proxy ready** | Designed to run behind nginx/caddy/traefik |

### Special Port Handling

| Port | Special Behavior |
|------|------------------|
| `80` | Enable Let's Encrypt HTTP-01 challenge |
| `443` | Enable Let's Encrypt TLS-ALPN-01 challenge, auto-enable SSL |
| `0` | Let OS assign any available port |
| `64000-64999` | Default range for random selection |

### Port Display Rules

| Rule | Description |
|------|-------------|
| Strip `:80` | Don't show port for HTTP on 80 |
| Strip `:443` | Don't show port for HTTPS on 443 |
| Always show others | Show port for all non-standard ports |

### Dual Port Support

**Applications can listen on two ports simultaneously for HTTP and HTTPS:**

| Format | Description | Example |
|--------|-------------|---------|
| Single | One port (HTTP; exception: port 443 = HTTPS-only; `ssl.enabled` overrides) | `8090` |
| Dual | HTTP port, HTTPS port (comma-separated) | `8090,8443` |

**Dual Port Behavior:**

| HTTP Port | HTTPS Port | Behavior |
|-----------|------------|----------|
| Any | Any | Both ports active, HTTP redirects to HTTPS optional |
| 80 | 443 | Let's Encrypt challenges enabled, standard web ports |
| 64xxx | 64xxx+1 | Common pattern for random ports |

```yaml
# Single port (HTTP by default; exception: port 443 = HTTPS-only mode; ssl.enabled overrides)
server:
  port: 8090

# Single port with ssl.enabled override (HTTPS on non-443 port)
server:
  port: 8090
  ssl:
    enabled: true  # Forces HTTPS on port 8090

# Dual port (HTTP on 8090, HTTPS on 8443)
server:
  port: "8090,8443"
  ssl:
    enabled: true

# Standard web ports
server:
  port: "80,443"
  ssl:
    enabled: true
    letsencrypt:
      enabled: true
```

### Configuration

```yaml
server:
  # Port options:
  # - Omit or empty: Random port in 64000-64999 range
  # - Single number: Use that exact port
  # - Dual (HTTP,HTTPS): "8090,8443" format
  # - 0: Let OS assign any available port
  port: 64580
```

### Admin Panel

Port can be changed via `/admin/server/settings`, but **requires server restart** (with warning shown to user).

### Example Structure

```yaml
# =============================================================================
# SERVER CONFIGURATION
# =============================================================================

server:
  port: {random}              # Default: random unused port in 64xxx range
  fqdn: {hostname}            # Auto-detected from host
  address: "[::]"             # [::] = all interfaces IPv4/IPv6
  mode: production            # production or development

  # Branding & SEO - see PART 10 for full details
  branding:
    title: "airports"
    tagline: ""
    description: ""
  seo:
    keywords: []

  # System user/group
  user: {auto}
  group: {auto}

  # PID file
  pidfile: true

  # Admin Panel
  admin:
    email: admin@{fqdn}
    # Note: username, password, and token are stored in database (admin_credentials table)
    # NOT in this config file for security

  # SSL/TLS
  ssl:
    enabled: false
    # Cert location determined automatically (by FQDN):
    #   /etc/letsencrypt/live/domain/ → system manages (certbot)
    #   /etc/letsencrypt/live/{fqdn}/ → system manages (certbot)
    #   {config_dir}/ssl/letsencrypt/{fqdn}/ → app manages (auto-renew 7 days before expiry)
    #   {config_dir}/ssl/local/{fqdn}/ → user manages (no auto-renew)

    letsencrypt:
      enabled: false
      email: admin@{fqdn}
      challenge: http-01

  # Scheduler
  schedule:
    enabled: true

  # Rate Limiting
  rate_limit:
    enabled: true
    requests: 120
    window: 60

  # Database
  database:
    driver: file

# =============================================================================
# FRONTEND CONFIGURATION
# =============================================================================

web:
  ui:
    theme: dark
  cors: "*"
```

---

# CHECKPOINT 4: CONFIGURATION VERIFICATION

Before proceeding, confirm you understand:
- [ ] Config file is `server.yml` (not .yaml)
- [ ] Boolean handling accepts multiple truthy/falsy values
- [ ] Environment variables: some runtime, some init-only
- [ ] Config auto-created on first run with sane defaults

---

# PART 7: APPLICATION MODES (NON-NEGOTIABLE)

## Mode Detection Priority

1. `--mode` CLI flag (highest priority)
2. `MODE` environment variable
3. Default: `production`

## Production Mode (Default)

| Setting | Behavior |
|---------|----------|
| Logging | `info` level, minimal output |
| Debug endpoints | Disabled (`/debug/*` returns 404) |
| Error messages | Generic (no stack traces) |
| Panic recovery | Graceful (logs error, returns 500) |
| Template caching | Enabled |
| Static file caching | Enabled |
| Rate limiting | Enforced |
| Security headers | All enabled |
| Sensitive data | Never shown |

## Development Mode

| Setting | Behavior |
|---------|----------|
| Logging | `debug` level, verbose |
| Debug endpoints | Enabled (`/debug/pprof/*`) |
| Error messages | Detailed (stack traces) |
| Panic recovery | Verbose (full stack in response) |
| Template caching | Disabled |
| Static file caching | Disabled |
| Rate limiting | Relaxed/disabled |
| Security headers | Relaxed |
| Sensitive data | Can be shown (with warning) |

## Mode Shortcuts

| Shortcut | Mode |
|----------|------|
| `--mode dev` | development |
| `--mode development` | development |
| `--mode prod` | production |
| `--mode production` | production |

---

# PART 8: SSL/TLS & LET'S ENCRYPT (NON-NEGOTIABLE)

## Built-in Let's Encrypt Support

**ALL projects MUST have built-in Let's Encrypt support.**

### Supported Challenge Types

| Type | Description |
|------|-------------|
| DNS-01 | All providers and RFC2136 |
| TLS-ALPN-01 | TLS-based challenge |
| HTTP-01 | HTTP-based challenge |

### FQDN Resolution (NON-NEGOTIABLE)

**The FQDN is determined in this order (first valid, non-loopback wins):**

| Priority | Source | Description |
|----------|--------|-------------|
| 1 | **Reverse Proxy Headers** | `X-Forwarded-Host`, `X-Real-Host`, `X-Original-Host` (preferred - we want to be behind a proxy) |
| 2 | `DOMAIN` env var | User override (recommended for production) |
| 3 | `os.Hostname()` | Go's cross-platform hostname function |
| 4 | `$HOSTNAME` env var | Shell environment fallback |
| 5 | Global IPv6 | First routable IPv6 address |
| 6 | Global IPv4 | First routable IPv4 address |

**IMPORTANT: Reverse proxy headers take priority because we prefer apps to run behind a reverse proxy. Avoid loopback addresses (`localhost`, `127.0.0.1`, `::1`) whenever possible.**

**DOMAIN Usage:**

| Environment | DOMAIN Value | Example |
|-------------|--------------|---------|
| **Development** | `airports` | `DOMAIN=jokes` |
| **Production** | Valid FQDN | `DOMAIN=api.example.com` |

**Valid Production DOMAIN formats:**
```
DOMAIN=example.com             # Simple TLD
DOMAIN=google.co.uk            # eTLD+1 with complex suffix
DOMAIN=my.server.domain.co.uk  # Subdomain of eTLD+1
DOMAIN=api.example.com         # Standard subdomain
```

**Do NOT set DOMAIN to overlay network addresses:**
- `.onion`, `.i2p`, `.exit` - these are app-generated and managed
- App handles overlay network registration/generation automatically
- Overlay addresses shown separately in console (see below)

**Go Implementation:**
```go
// GetHostFromRequest - use this for request-time host resolution (preferred)
func GetHostFromRequest(r *http.Request, projectName string) string {
    // 1. Reverse proxy headers (highest priority - we prefer to be behind a proxy)
    for _, header := range []string{"X-Forwarded-Host", "X-Real-Host", "X-Original-Host"} {
        if host := r.Header.Get(header); host != "" {
            // Strip port if present (we handle port separately)
            if h, _, err := net.SplitHostPort(host); err == nil {
                return h
            }
            return host
        }
    }

    // 2. Fall back to static resolution
    return GetFQDN(projectName)
}

// GetFQDN - use this when no request context available (startup, background tasks)
func GetFQDN(projectName string) string {
    // 1. DOMAIN env var (explicit user override)
    if domain := os.Getenv("DOMAIN"); domain != "" {
        return domain
    }

    // 2. os.Hostname() - cross-platform (Linux, macOS, Windows, BSD)
    if hostname, err := os.Hostname(); err == nil && hostname != "" {
        if !isLoopback(hostname) {
            return hostname
        }
    }

    // 3. $HOSTNAME env var (skip loopback)
    if hostname := os.Getenv("HOSTNAME"); hostname != "" {
        if !isLoopback(hostname) {
            return hostname
        }
    }

    // 4. Global IPv6 (preferred for modern networks)
    if ipv6 := getGlobalIPv6(); ipv6 != "" {
        return ipv6
    }

    // 5. Global IPv4
    if ipv4 := getGlobalIPv4(); ipv4 != "" {
        return ipv4
    }

    // Last resort (not recommended)
    return "localhost"
}

func isLoopback(host string) bool {
    lower := strings.ToLower(host)
    if lower == "localhost" {
        return true
    }
    if ip := net.ParseIP(host); ip != nil {
        return ip.IsLoopback()
    }
    return false
}

func getGlobalIPv6() string {
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        return ""
    }
    for _, addr := range addrs {
        if ipnet, ok := addr.(*net.IPNet); ok {
            if ipnet.IP.To4() == nil && ipnet.IP.IsGlobalUnicast() {
                return ipnet.IP.String()
            }
        }
    }
    return ""
}

func getGlobalIPv4() string {
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        return ""
    }
    for _, addr := range addrs {
        if ipnet, ok := addr.(*net.IPNet); ok {
            if ip4 := ipnet.IP.To4(); ip4 != nil && ipnet.IP.IsGlobalUnicast() {
                return ip4.String()
            }
        }
    }
    return ""
}
```

**Note:** For production SSL/Let's Encrypt, always set `DOMAIN` explicitly:
```bash
export DOMAIN=api.example.com
```

**The resolved FQDN is used for:**
- SSL certificate lookup and generation
- Let's Encrypt certificate requests
- `server.host` default value (if not configured)

### Dev TLD Handling (NON-NEGOTIABLE)

**Dev TLDs are allowed in development mode but require global IP fallback for remote access.**

**Dynamic Dev TLDs (project name as TLD):**
- `airports` - e.g., `app.jokes`, `my.quotes`, `dev.api`
- `airports.local` - e.g., `app.jokes.local`
- `airports.test` - e.g., `app.jokes.test`

**Static Dev TLDs:**
- `.local`, `.test`, `.example`, `.invalid` (RFC 6761)
- `.localhost`, `.lan`, `.internal`, `.home`, `.localdomain`
- `.home.arpa`, `.intranet`, `.corp`, `.private`

**Get display URL (returns ONE URL):**

```go
// GetDisplayURL returns the best URL for display/access
// Prefers valid FQDN, falls back to global IP if dev TLD
// isHTTPS: true if this is the HTTPS port (second port in dual mode)
func GetDisplayURL(projectName string, port int, isHTTPS bool) string {
    fqdn := GetFQDN(projectName)

    // If valid production FQDN, use it
    if !isDevTLD(fqdn, projectName) && fqdn != "localhost" {
        return formatURL(fqdn, port, isHTTPS)
    }

    // Dev TLD or localhost - use global IP instead
    if ipv6 := getGlobalIPv6(); ipv6 != "" {
        return formatURL("["+ipv6+"]", port, isHTTPS)
    }
    if ipv4 := getGlobalIPv4(); ipv4 != "" {
        return formatURL(ipv4, port, isHTTPS)
    }

    // Last resort
    return formatURL(fqdn, port, isHTTPS)
}

func isDevTLD(host, projectName string) bool {
    lower := strings.ToLower(host)

    // Check dynamic project-specific TLD (e.g., app.jokes, dev.quotes, quotes, jokes, airports)
    if projectName != "" && strings.HasSuffix(lower, "."+strings.ToLower(projectName)) {
        return true
    }

    // Check static dev TLDs
    for tld := range devOnlyTLDs {
        if strings.HasSuffix(lower, "."+tld) || lower == tld {
            return true
        }
    }

    return false
}
```

**Startup Banner - Pretty Console with Icons:**

**IMPORTANT: Logs are ALWAYS raw text. No icons, ASCII art, or special characters in log output.**

| Element | Icon | Usage |
|---------|------|-------|
| App name | 🚀 | Header |
| Version | 📦 | Version info |
| Production mode | 🔒 | Mode indicator |
| Development mode | 🔧 | Mode indicator |
| Tor/Onion | 🧅 | Overlay network |
| I2P | 🔗 | Overlay network |
| SSL/HTTPS | 🔐 | Secure connection |
| HTTP | 🌐 | Non-secure connection |
| IPv6 | 🌍 | Network address |
| IPv4 | 🌐 | Network address |
| Success | ✅ | Status |
| Warning | ⚠️ | Status |
| Error | ❌ | Status |
| Info | ℹ️ | Status |

**Port Configuration (Project-Wide, NON-NEGOTIABLE):**

| Mode | Config | HTTP Port | HTTPS Port |
|------|--------|-----------|------------|
| Single HTTP | `--port 8080` | 8080 | None |
| Single HTTPS | `--port 443` | None | 443 |
| Dual | `--port 80,443` | 80 | 443 |
| Dual | `--port 8080,8443` | 8080 | 8443 |

**Rules:**
- Single port: HTTP by default
- **Exception: If single port is 443 → HTTPS-only mode**
- Dual ports: First = HTTP, Second = HTTPS
- **Override: `CONFIG(ssl.enabled)` can override HTTP → HTTPS on any port**

**URL Format Rule:**

**ALWAYS strip :80 and :443.**

| Port | Protocol | URL |
|------|----------|-----|
| 80 | HTTP | `http://host` |
| 443 | HTTPS | `https://host` |
| Other | Depends on config | `{proto}://host:{port}` |

```go
func formatURL(host string, port int, isHTTPS bool) string {
    proto := "http"
    if isHTTPS || port == 443 {
        proto = "https"
    }

    // Always strip :80 and :443
    if port == 80 || port == 443 {
        return proto + "://" + host
    }

    return fmt.Sprintf("%s://%s:%d", proto, host, port)
}
```

**Usage:**
```go
// Single HTTP port
formatURL(host, 8080, false)  // http://host:8080

// Single HTTPS port (443 = HTTPS-only mode)
formatURL(host, 443, false)   // https://host  (443 forces HTTPS)

// Dual port mode
formatURL(host, 80, false)    // http://host
formatURL(host, 443, true)    // https://host
formatURL(host, 8080, false)  // http://host:8080
formatURL(host, 8443, true)   // https://host:8443
```

**Overlay Network Protocol Rules (Tor, I2P, etc.):**

| Network | Default | HTTPS-Only Mode | Certificate |
|---------|---------|-----------------|-------------|
| Clearnet | HTTP/HTTPS | Port 443 | Let's Encrypt or local |
| Tor (.onion) | HTTP | When clearnet is HTTPS-only | Self-signed (LE doesn't support .onion) |
| I2P (.i2p) | HTTP | When clearnet is HTTPS-only | Self-signed (LE doesn't support .i2p) |

**Rules:**
- Overlay networks inherit HTTPS-only mode from clearnet configuration
- If clearnet port is 443 (HTTPS-only) → overlay also uses HTTPS
- If clearnet is dual port (80,443) → overlay uses HTTP (encryption provided by overlay)
- Overlay HTTPS requires self-signed certificates (Let's Encrypt doesn't support .onion/.i2p)

**Banner Examples:**

| Clearnet Config | Tor URL | I2P URL |
|-----------------|---------|---------|
| `--port 8080` | `http://{onion}.onion` | `http://{i2p}.b32.i2p` |
| `--port 443` | `https://{onion}.onion` | `https://{i2p}.b32.i2p` |
| `--port 80,443` | `http://{onion}.onion` | `http://{i2p}.b32.i2p` |
| `--port 8080,8443` | `http://{onion}.onion` | `http://{i2p}.b32.i2p` |

**Why HTTP for overlays in dual mode?**
- Tor/I2P provide end-to-end encryption at the network layer
- HTTPS adds overhead without additional security benefit
- Only use HTTPS on overlays when HTTPS-only mode is required (port 443)

**Footer timestamp format:** `%a %b %d, %Y at %H:%M:%S %Z` → `Wed Jan 15, 2025 at 09:00:00 EST`

**Example (Production with SSL + Tor on 443):**
```
╭─────────────────────────────────────────────────────────────╮
│  🚀 AIRPORTS · 📦 v1.0.0 · 🔒 Production               │
├─────────────────────────────────────────────────────────────┤
│  🧅 Tor    http://abc123def456ghi789jklmnop.onion           │
│  🔐 HTTPS  https://api.example.com                          │
├─────────────────────────────────────────────────────────────┤
│  📡 Listening on https://203.0.113.50                       │
│  ✅ Server started on Wed Jan 15, 2025 at 09:00:00 EST      │
╰─────────────────────────────────────────────────────────────╯
```

**Example (Production with Tor + I2P on 443):**
```
╭─────────────────────────────────────────────────────────────╮
│  🚀 AIRPORTS · 📦 v1.0.0 · 🔒 Production               │
├─────────────────────────────────────────────────────────────┤
│  🧅 Tor    http://abc123def456ghi789jklmnop.onion           │
│  🔗 I2P    http://xyz789abc123def456uvwxyz.b32.i2p          │
│  🔐 HTTPS  https://api.example.com                          │
├─────────────────────────────────────────────────────────────┤
│  📡 Listening on https://203.0.113.50                       │
│  ✅ Server started on Wed Jan 15, 2025 at 09:00:00 EST      │
╰─────────────────────────────────────────────────────────────╯
```

**Example (Production on port 8080):**
```
╭─────────────────────────────────────────────────────────────╮
│  🚀 AIRPORTS · 📦 v1.0.0 · 🔒 Production               │
├─────────────────────────────────────────────────────────────┤
│  🌐 HTTP   http://api.example.com:8080                      │
├─────────────────────────────────────────────────────────────┤
│  📡 Listening on http://203.0.113.50:8080                   │
│  ✅ Server started on Wed Jan 15, 2025 at 09:00:00 EST      │
╰─────────────────────────────────────────────────────────────╯
```

**Example (Development on port 8080):**
```
╭─────────────────────────────────────────────────────────────╮
│  🚀 AIRPORTS · 📦 v1.0.0 · 🔧 Development              │
├─────────────────────────────────────────────────────────────┤
│  🌐 HTTP   http://192.168.1.100:8080                        │
├─────────────────────────────────────────────────────────────┤
│  📡 Listening on http://192.168.1.100:8080                  │
│  ✅ Server started on Wed Jan 15, 2025 at 09:00:00 EST      │
╰─────────────────────────────────────────────────────────────╯
```

**Example (Development IPv6 on port 8080):**
```
╭─────────────────────────────────────────────────────────────╮
│  🚀 AIRPORTS · 📦 v1.0.0 · 🔧 Development              │
├─────────────────────────────────────────────────────────────┤
│  🌍 IPv6   http://[2001:db8::1]:8080                        │
├─────────────────────────────────────────────────────────────┤
│  📡 Listening on http://[2001:db8::1]:8080                  │
│  ✅ Server started on Wed Jan 15, 2025 at 09:00:00 EST      │
╰─────────────────────────────────────────────────────────────╯
```

**Example (Production on port 80):**
```
╭─────────────────────────────────────────────────────────────╮
│  🚀 AIRPORTS · 📦 v1.0.0 · 🔒 Production               │
├─────────────────────────────────────────────────────────────┤
│  🌐 HTTP   http://api.example.com                           │
├─────────────────────────────────────────────────────────────┤
│  📡 Listening on http://203.0.113.50                        │
│  ✅ Server started on Wed Jan 15, 2025 at 09:00:00 EST      │
╰─────────────────────────────────────────────────────────────╯
```

**Example (First Run - Setup Required):**
```
╭─────────────────────────────────────────────────────────────╮
│  🚀 AIRPORTS · 📦 v1.0.0 · 🔧 Development              │
├─────────────────────────────────────────────────────────────┤
│  🌐 HTTP   http://192.168.1.100:8080                        │
├─────────────────────────────────────────────────────────────┤
│  📡 Listening on http://192.168.1.100:8080                  │
│  ✅ Server started on Wed Jan 15, 2025 at 09:00:00 EST      │
╰─────────────────────────────────────────────────────────────╯

┌─────────────────────────────────────────────────────────────┐
│  🔑 SETUP REQUIRED                                          │
├─────────────────────────────────────────────────────────────┤
│  Setup Token: a1b2c3d4-e5f6-7890-abcd-ef1234567890          │
│                                                             │
│  Go to /admin and enter this token to complete setup.       │
│  This token will only be shown ONCE.                        │
└─────────────────────────────────────────────────────────────┘
```

**Note:** Setup token is displayed ONCE on first run. After setup wizard is completed, this section never appears again.

### Console vs Logs (NON-NEGOTIABLE)

| Output | Icons | ASCII Art | Colors | Format |
|--------|-------|-----------|--------|--------|
| **Startup banner** | ✅ Yes | ✅ Yes | ✅ Yes | Pretty |
| **Interactive console** | ✅ Yes | ✅ Yes | ✅ Yes | Pretty |
| **Log files** | ❌ Never | ❌ Never | ❌ Never | Raw text |
| **Log output (stdout)** | ❌ Never | ❌ Never | ❌ Never | Raw text |

**Log format is ALWAYS plain text:**
```
2025-01-15 09:00:00 INFO  Server started on :8080
2025-01-15 09:00:01 INFO  Database connected
2025-01-15 09:00:05 WARN  High memory usage: 85%
2025-01-15 09:00:10 ERROR Connection timeout to upstream
```

**Never this in logs:**
```
✅ Server started    <- NO icons in logs
🔧 Development mode  <- NO icons in logs
```

### Certificate Lookup Order (NON-NEGOTIABLE)

**On startup, check for existing certificates in this order:**

| Priority | Path | Description |
|----------|------|-------------|
| 1 | `/etc/letsencrypt/live/domain/` | Literal "domain" directory (common shared setup) |
| 2 | `/etc/letsencrypt/live/{fqdn}/` | FQDN-named directory (e.g., `/etc/letsencrypt/live/api.example.com/`) |
| 3 | `{config_dir}/ssl/letsencrypt/{fqdn}/` | App-managed Let's Encrypt certificates |
| 4 | `{config_dir}/ssl/local/{fqdn}/` | Self-signed or user-provided certificates |

**Certificate Validation:**
- Certificate CN or SAN must match configured FQDN (`server.host`)
- Certificate must not be expired
- Both cert and key files must be readable

### Certificate Directory Structure

```
{config_dir}/ssl/
├── letsencrypt/
│   └── {fqdn}/                # e.g., api.example.com/
│       ├── fullchain.pem
│       └── privkey.pem
└── local/
    └── {fqdn}/                # e.g., api.example.com/
        ├── cert.pem
        └── key.pem
```

**Mirrors certbot structure** (`/etc/letsencrypt/live/{fqdn}/`) for consistency.

### Certificate Management Ownership

| Location | Manager | Renewal |
|----------|---------|---------|
| `/etc/letsencrypt/live/**` | **System** (certbot) | App does NOT renew |
| `{config_dir}/ssl/letsencrypt/{fqdn}/` | **App** | Auto-renew 7 days before expiry |
| `{config_dir}/ssl/local/{fqdn}/` | **User** | No auto-renewal (manual) |

**Rule: If cert is in `{config_dir}/ssl/**`, app manages. Subdirectory determines HOW.**

### Renewal Rules

| Directory | Check Frequency | Renewal Trigger |
|-----------|-----------------|-----------------|
| `{config_dir}/ssl/letsencrypt/{fqdn}/` | Daily (02:00) | 7 days before expiry |
| `{config_dir}/ssl/local/{fqdn}/` | Never | Manual only |
| `/etc/letsencrypt/live/**` | Never | System (certbot) manages |

**Logic Flow:**
```
Startup (for configured FQDN)
   │
   ├─► Check /etc/letsencrypt/live/domain/
   │   └─► Found + cert matches FQDN? → Use it (system manages)
   │
   ├─► Check /etc/letsencrypt/live/{fqdn}/
   │   └─► Found? → Use it (system manages)
   │
   ├─► Check {config_dir}/ssl/letsencrypt/{fqdn}/
   │   └─► Found? → Use it (app auto-renews)
   │
   ├─► Check {config_dir}/ssl/local/{fqdn}/
   │   └─► Found? → Use it (no auto-renewal)
   │
   └─► Not found anywhere
       └─► Request new cert via Let's Encrypt
           └─► Save to {config_dir}/ssl/letsencrypt/{fqdn}/ (app auto-renews)
```

**Important:**
- `/etc/letsencrypt/` → app uses but does NOT manage renewal
- `{config_dir}/ssl/letsencrypt/{fqdn}/` → app manages, auto-renews 7 days before expiry
- `{config_dir}/ssl/local/{fqdn}/` → app uses but does NOT auto-renew (user manages)

---

# PART 9: SCHEDULER (NON-NEGOTIABLE)

## Built-in Scheduler

**ALL projects MUST have a built-in scheduler that is ALWAYS RUNNING.**

### Core Requirements

| Requirement | Description |
|-------------|-------------|
| **Always Running** | Scheduler starts with application and runs until shutdown |
| **Persistent State** | Task state survives restarts (stored in server.db) |
| **Automatic Recovery** | Missed tasks run on startup if within catch-up window |
| **Cluster Aware** | Only one node runs each task in cluster mode |
| **No External Dependencies** | Built-in, no cron or external scheduler needed |

### Built-in Tasks (Required)

Every project MUST include these scheduled tasks:

| Task | Default Schedule | Purpose | Skippable |
|------|-----------------|---------|-----------|
| `ssl.renewal` | Daily at 02:00 | Renew `{config_dir}/ssl/letsencrypt/{fqdn}/` certs 7 days before expiry | No |
| `geoip.update` | Weekly (Sunday 03:00) | Update GeoIP databases | Yes |
| `blocklist.update` | Daily at 04:00 | Update IP blocklists | Yes |
| `session.cleanup` | Hourly | Remove expired sessions | No |
| `token.cleanup` | Daily at 05:00 | Remove expired tokens | No |
| `backup.auto` | Disabled by default | Automatic backups | Yes |
| `healthcheck.self` | Every 5 minutes | Self-health verification | No |
| `cluster.heartbeat` | Every 30 seconds | Cluster node heartbeat (cluster mode only) | No |

### Task Configuration

```yaml
server:
  scheduler:
    # Scheduler is ALWAYS enabled - cannot be disabled
    # This setting only controls whether custom tasks run
    enabled: true

    # Timezone for scheduled tasks (default: America/New_York)
    timezone: America/New_York

    # Catch-up window: run missed tasks if within this duration
    catch_up_window: 1h

    # Built-in tasks (can adjust schedule, cannot disable critical tasks)
    tasks:
      ssl_renewal:
        schedule: "0 2 * * *"      # Cron format: daily at 02:00
        enabled: true

      geoip_update:
        schedule: "0 3 * * 0"      # Weekly Sunday at 03:00
        enabled: true

      blocklist_update:
        schedule: "0 4 * * *"      # Daily at 04:00
        enabled: true

      session_cleanup:
        schedule: "@hourly"        # Every hour
        enabled: true

      token_cleanup:
        schedule: "0 5 * * *"      # Daily at 05:00
        enabled: true

      backup_auto:
        schedule: "0 1 * * *"      # Daily at 01:00
        enabled: false             # Disabled by default
        keep_count: 5              # Keep last 5 backups

      healthcheck_self:
        schedule: "@every 5m"      # Every 5 minutes
        enabled: true
```

### Schedule Format

| Format | Example | Description |
|--------|---------|-------------|
| Cron | `0 2 * * *` | Standard cron (minute hour day month weekday) |
| `@hourly` | `@hourly` | Every hour at minute 0 |
| `@daily` | `@daily` | Every day at 00:00 |
| `@weekly` | `@weekly` | Every Sunday at 00:00 |
| `@monthly` | `@monthly` | First day of month at 00:00 |
| `@every Xm` | `@every 5m` | Every X minutes |
| `@every Xh` | `@every 2h` | Every X hours |

### Scheduler State (Persistent)

Task state is stored in `server.db`:

| Column | Type | Description |
|--------|------|-------------|
| `task_id` | String | Unique task identifier |
| `task_name` | String | Human-readable name |
| `schedule` | String | Cron/interval expression |
| `last_run` | Timestamp | When task last completed |
| `last_status` | String | success, failed, skipped |
| `last_error` | String | Error message if failed |
| `next_run` | Timestamp | Scheduled next execution |
| `run_count` | Integer | Total successful runs |
| `fail_count` | Integer | Total failed runs |
| `enabled` | Boolean | Is task enabled |
| `locked_by` | String | Node ID holding lock (cluster mode) |
| `locked_at` | Timestamp | When lock was acquired |

### Startup Behavior

```
Application Start
       │
       ▼
Load scheduler state from database
       │
       ▼
Check for missed tasks (within catch_up_window)
       │
       ├─► Found missed tasks
       │   │
       │   ▼
       │   Queue missed tasks for immediate execution
       │   (in order of original scheduled time)
       │
       ▼
Start scheduler loop
       │
       ▼
Scheduler runs continuously until shutdown
```

### Cluster Mode Task Distribution

In cluster mode, tasks are distributed to prevent duplicate execution:

| Task Type | Execution |
|-----------|-----------|
| **Global Tasks** | Run on ONE node only (leader election) |
| **Local Tasks** | Run on EVERY node |

**Global Tasks (run once per cluster):**
- `ssl.renewal`
- `geoip.update`
- `blocklist.update`
- `backup.auto`

**Local Tasks (run on each node):**
- `session.cleanup`
- `token.cleanup`
- `healthcheck.self`
- `cluster.heartbeat`

### Task Locking (Cluster Mode)

```
Task Ready to Run
       │
       ▼
Attempt to acquire lock in database
       │
       ├─► Lock acquired
       │   │
       │   ▼
       │   Execute task
       │   │
       │   ▼
       │   Release lock, update last_run
       │
       └─► Lock held by another node
           │
           ▼
           Skip execution (other node handling it)
```

**Lock timeout:** 5 minutes (auto-release if node dies during task)

### Task Execution Flow

```
Task Triggered (scheduled or manual)
       │
       ▼
Check if enabled
       │
       ├─► Disabled: Skip
       │
       ▼
Acquire lock (cluster mode)
       │
       ├─► Lock failed: Skip (another node running)
       │
       ▼
Execute task
       │
       ├─► Success
       │   │
       │   ▼
       │   Update: last_run, last_status=success, run_count++
       │   Log to audit log
       │
       └─► Failure
           │
           ▼
           Update: last_status=failed, last_error, fail_count++
           Log to audit log
           Send notification (if configured)
           Schedule retry (if retryable)
```

### Retry Policy

| Setting | Default | Description |
|---------|---------|-------------|
| `max_retries` | 3 | Maximum retry attempts |
| `retry_delay` | 5m | Delay between retries |
| `backoff` | exponential | Delay multiplier (5m, 10m, 20m) |

### Admin Panel (/admin/server/scheduler)

| Section | Contents |
|---------|----------|
| **Task List** | All tasks with status, next run, last run |
| **Task Detail** | Full history, logs, configuration |
| **Run Now** | Button to trigger immediate execution |
| **Enable/Disable** | Toggle for non-critical tasks |
| **History** | Last 100 executions per task |

**Task List Columns:**

| Column | Description |
|--------|-------------|
| Task Name | Human-readable name |
| Status | ● running, ✓ success, ✗ failed, ○ pending |
| Last Run | Timestamp and duration |
| Next Run | Scheduled time |
| Actions | Run Now, View History |

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/scheduler` | GET | List all tasks |
| `/api/v1/admin/server/scheduler/{id}` | GET | Get task details |
| `/api/v1/admin/server/scheduler/{id}` | PATCH | Update task settings |
| `/api/v1/admin/server/scheduler/{id}/run` | POST | Run task immediately |
| `/api/v1/admin/server/scheduler/{id}/enable` | POST | Enable task |
| `/api/v1/admin/server/scheduler/{id}/disable` | POST | Disable task |
| `/api/v1/admin/server/scheduler/{id}/history` | GET | Get execution history |

### Shutdown Behavior

```
Shutdown Signal Received
       │
       ▼
Stop accepting new task executions
       │
       ▼
Wait for running tasks to complete (max 30 seconds)
       │
       ├─► Tasks completed
       │   │
       │   ▼
       │   Release all locks
       │
       └─► Timeout
           │
           ▼
           Force release locks
           Mark interrupted tasks for retry on next start
       │
       ▼
Save scheduler state to database
       │
       ▼
Shutdown complete
```

### Implementation Requirements

1. **Use Go's time/ticker** - No external cron libraries required
2. **Database-backed state** - All state in server.db, survives restarts
3. **Graceful shutdown** - Complete running tasks, release locks
4. **Cluster-safe** - Distributed locking for global tasks
5. **Audit logging** - All task executions logged
6. **Notifications** - Failed tasks trigger notifications (if configured)

---

# PART 10: GEOIP (NON-NEGOTIABLE)

## Overview

**ALL projects MUST have built-in GeoIP support using sapics/ip-location-db.**

GeoIP databases are NEVER embedded - they are downloaded on first run and updated via scheduler.

## Configuration

```yaml
server:
  geoip:
    enabled: true

    # Directory for downloaded MMDB files
    dir: "{data_dir}/geoip"

    # Update schedule: never, daily, weekly, monthly
    update: weekly

    # Block countries by ISO 3166-1 alpha-2 code
    deny_countries: []

    # Which databases to download and use
    # Full IPv4 and IPv6 support - always both
    databases:
      # ASN lookup - organization name and AS number
      asn: true
      # Country lookup - country code and name
      country: true
      # City lookup - city, region, postal, coordinates, timezone (larger download)
      city: true
```

## Database Sources

| Database | CDN URL |
|----------|---------|
| ASN | `https://cdn.jsdelivr.net/npm/@ip-location-db/asn-mmdb/asn.mmdb` |
| Country | `https://cdn.jsdelivr.net/npm/@ip-location-db/geo-whois-asn-country-mmdb/geo-whois-asn-country.mmdb` |
| City | `https://cdn.jsdelivr.net/npm/@ip-location-db/dbip-city-mmdb/dbip-city.mmdb` |

## Admin Panel (/admin/server/geoip)

| Element | Type | Description |
|---------|------|-------------|
| Enable GeoIP | Toggle | Turn GeoIP on/off |
| Update schedule | Dropdown | never, daily, weekly, monthly |
| Deny countries | Tag input | ISO 3166-1 alpha-2 codes to block |
| ASN database | Toggle | Enable ASN lookups |
| Country database | Toggle | Enable country lookups |
| City database | Toggle | Enable city lookups |
| Last update | Read-only | When databases were last updated |
| Update now | Button | Force immediate update |

---

# PART 11: METRICS (NON-NEGOTIABLE)

## Overview

**ALL projects MUST have built-in Prometheus-compatible metrics support.**

## Configuration

```yaml
server:
  metrics:
    enabled: false
    endpoint: /metrics

    # Include system metrics (CPU, memory, disk)
    include_system: true

    # Optional Bearer token for authentication
    token: ""
```

## Metrics Types

| Type | Description | Always Included |
|------|-------------|-----------------|
| Application | Request count, error rate, latency | Yes (when enabled) |
| System | CPU, memory, disk usage | Configurable |

## Admin Panel (/admin/server/metrics)

| Element | Type | Description |
|---------|------|-------------|
| Enable metrics | Toggle | Turn metrics on/off |
| Endpoint | Text input | Metrics endpoint path (default: /metrics) |
| Include system metrics | Toggle | Include CPU/memory/disk |
| Authentication token | Text input | Bearer token (empty = no auth) |

---

# PART 12: SERVER CONFIGURATION (NON-NEGOTIABLE)

## Request Limits

```yaml
server:
  limits:
    max_body_size: 10MB
    read_timeout: 30s
    write_timeout: 30s
    idle_timeout: 120s
```

| Setting | Description | Default |
|---------|-------------|---------|
| `max_body_size` | Maximum request body size | 10MB |
| `read_timeout` | Read timeout | 30s |
| `write_timeout` | Write timeout | 30s |
| `idle_timeout` | Idle connection timeout | 120s |

## Response Compression

```yaml
server:
  compression:
    enabled: true
    # Compression level 1-9
    level: 5
    # MIME types to compress
    types:
      - text/html
      - text/css
      - text/javascript
      - application/json
      - application/xml
```

## Trusted Proxies

```yaml
server:
  trusted_proxies:
    # Additional IPs/CIDRs to trust (private ranges always trusted)
    additional: []
```

## Session Configuration

```yaml
server:
  session:
    cookie_name: session_id
    # 30 days in seconds
    max_age: 2592000
    # auto, true, false
    secure: auto
    http_only: true
    # strict, lax, none
    same_site: lax
```

## Rate Limiting

```yaml
server:
  rate_limit:
    enabled: true
    # Requests allowed per window
    requests: 120
    # Window size in seconds
    window: 60
```

## Internationalization (i18n)

```yaml
server:
  i18n:
    default_language: en
    supported: [en]
```

## Cache Configuration (NON-NEGOTIABLE)

**EVERY application MUST support Valkey/Redis.** This is REQUIRED for:
- Clustering (session sharing, state sync)
- Mixed Mode (cross-database synchronization)
- Horizontal scaling
- Rate limiting in distributed deployments

| Mode | Cache Requirement |
|------|-------------------|
| **Single Instance** | `memory` (default) - works standalone |
| **Cluster** | `valkey` or `redis` - REQUIRED for state sync |
| **Mixed Mode** | `valkey` or `redis` - REQUIRED for cross-DB sync |

```yaml
server:
  cache:
    # Type: none (disabled), memory (default), redis, valkey, memcache
    # IMPORTANT: Use valkey/redis for cluster or mixed mode deployments
    type: memory

    # Connection settings (for redis, valkey, memcache)
    host: localhost
    port: 6379
    password: ""
    db: 0

    # TLS settings (for secure connections)
    tls: false
    tls_skip_verify: false

    # Key prefix to avoid collisions (use unique prefix per app)
    prefix: "airports:"

    # Default TTL in seconds
    ttl: 3600

    # Cluster settings (when using Valkey/Redis Cluster)
    cluster: false
    cluster_nodes: []  # ["node1:6379", "node2:6379", "node3:6379"]
```

### Valkey/Redis Configuration Examples

**Single Valkey/Redis server:**
```yaml
server:
  cache:
    type: valkey
    host: valkey.example.com
    port: 6379
    password: ${VALKEY_PASSWORD}
    db: 0
    prefix: "airports:"
```

**Valkey/Redis Cluster:**
```yaml
server:
  cache:
    type: valkey
    cluster: true
    cluster_nodes:
      - valkey1.example.com:6379
      - valkey2.example.com:6379
      - valkey3.example.com:6379
    password: ${VALKEY_PASSWORD}
    prefix: "airports:"
```

### Cache Usage in Application

| Feature | Uses Cache | Purpose |
|---------|------------|---------|
| Sessions | Yes | Session data storage |
| Rate limiting | Yes | Request counters per IP/user |
| API responses | Optional | Response caching |
| Cluster heartbeat | Yes | Node liveness detection |
| Pub/Sub events | Yes | Real-time state sync |
| Distributed locks | Yes | Prevent duplicate task execution |

## Admin Panel (/admin/server/settings)

All settings above MUST be configurable via admin panel:

| Section | Settings |
|---------|----------|
| Request Limits | Body size, timeouts |
| Compression | Enable, level, MIME types |
| Trusted Proxies | Additional IPs/CIDRs |
| Session | Cookie name, max age, secure, http_only, same_site |
| Rate Limiting | Enable, requests, window |
| i18n | Default language, supported languages |
| Cache | Type, connection settings, prefix, TTL |

---

# PART 13: WEB FRONTEND (NON-NEGOTIABLE)

## Requirements

**ALL PROJECTS MUST HAVE A FANTASTIC FRONTEND BUILT IN.**

| Requirement | Description |
|-------------|-------------|
| Mobile Support | Full responsive design |
| HTML5 | Full web standards compliance |
| Accessibility | Full a11y support |
| UX | Readable, navigable, intuitive, self-explanatory |

## Technology Stack (NON-NEGOTIABLE)

| Rule | Description |
|------|-------------|
| **Go Templates** | ALL HTML uses Go `html/template` - NO EXCEPTIONS |
| Templates | Use partials (header, nav, body, footer, etc.) |
| Vanilla JS/CSS | Preferred, no frameworks unless necessary |
| **NO JS Alerts** | NEVER use default JavaScript alerts/confirms/prompts |
| Custom UI | Always use CSS modals, toast notifications |
| **NO Inline CSS** | NEVER use inline styles |

### HTML5 & CSS Over JavaScript (NON-NEGOTIABLE)

**Minimize JavaScript - prefer HTML5 and CSS solutions whenever possible.**

| Use Case | Use HTML5/CSS | Use JavaScript Only When |
|----------|---------------|--------------------------|
| Form validation | HTML5 `required`, `pattern`, `min`, `max`, `type="email"` | Complex cross-field validation |
| Collapsible sections | `<details>/<summary>` | Need animation or programmatic control |
| Tabs | CSS `:target` or radio button hack | Need deep linking or state management |
| Tooltips | CSS `::after` with `data-tooltip` | Need dynamic positioning |
| Modals | CSS `:target` selector | Need focus trap, escape key, backdrop click |
| Hover effects | CSS `:hover`, `:focus`, `:active` | Never - always CSS |
| Animations | CSS `@keyframes`, `transition` | Complex sequenced animations |
| Responsive design | CSS media queries | Never - always CSS |
| Toggle switches | CSS with hidden checkbox | Need state persistence |
| Dropdowns/menus | CSS `:focus-within` | Complex multi-level menus |
| Progress bars | HTML5 `<progress>` | Need dynamic updates |
| Sliders | HTML5 `<input type="range">` | Complex custom styling |
| Date pickers | HTML5 `<input type="date">` | Need custom calendar UI |
| Color pickers | HTML5 `<input type="color">` | Need swatches or advanced features |
| Accordions | `<details>/<summary>` | Need single-open behavior |

**JavaScript Guidelines:**
- **Last resort** - only when HTML5/CSS cannot achieve the functionality
- **Progressive enhancement** - features must work without JS where possible
- **No JS for styling** - unless it cannot be done in HTML5 and CSS
- **No JS for simple interactions** - hover, focus, basic toggles are CSS-only

**HTML5 Required (NON-NEGOTIABLE):**
- ALL HTML MUST be valid HTML5
- Use `<!DOCTYPE html>` declaration
- Use semantic HTML5 elements: `<header>`, `<nav>`, `<main>`, `<footer>`, `<article>`, `<section>`, `<aside>`
- Use HTML5 form elements: `<input type="email">`, `<input type="date">`, `<input type="number">`, etc.
- Use HTML5 attributes: `required`, `pattern`, `placeholder`, `autofocus`, `autocomplete`
- NO deprecated elements: `<center>`, `<font>`, `<marquee>`, `<blink>`, etc.
- NO deprecated attributes: `align`, `bgcolor`, `border`, `cellpadding`, etc.
- **Required for**: API calls, dynamic content loading, complex state, WebSockets
- **Size matters** - keep JS minimal, no large libraries for simple tasks

**Inline JavaScript - Allowed for simple operations:**
```html
<!-- Navigation -->
<button onclick="history.back()">Go Back</button>
<button onclick="history.forward()">Go Forward</button>
<button onclick="location.reload()">Refresh</button>

<!-- Clipboard -->
<button onclick="navigator.clipboard.writeText('text')">Copy</button>

<!-- Print -->
<button onclick="window.print()">Print</button>

<!-- Scroll -->
<button onclick="window.scrollTo(0,0)">Back to Top</button>

<!-- Simple toggles -->
<button onclick="document.getElementById('menu').classList.toggle('hidden')">Toggle Menu</button>

<!-- Form helpers -->
<button onclick="document.getElementById('myform').reset()">Reset Form</button>
<button onclick="document.getElementById('field').select()">Select All</button>

<!-- Simple show/hide -->
<button onclick="this.nextElementSibling.hidden = !this.nextElementSibling.hidden">Show/Hide</button>
```

**Rule:** Inline JS is fine for one-liner operations. Move to external JS file if:
- Logic exceeds one simple statement
- Same logic is repeated in multiple places
- Requires error handling or conditionals

### Go Templates (NON-NEGOTIABLE)

**ALL frontend HTML MUST use Go's `html/template` package.**

| Location | Purpose |
|----------|---------|
| `src/server/templates/` | All `.tmpl` template files |
| `src/server/templates/partials/` | Reusable template partials |
| `src/server/templates/layouts/` | Base layouts |
| `src/server/templates/pages/` | Page-specific templates |
| `src/server/static/` | Static assets (CSS, JS, images) |

**Template Structure (all files use `.tmpl` extension):**
```
src/server/templates/
├── layouts/
│   ├── base.tmpl           # Base layout with html, head, body
│   └── admin.tmpl          # Admin-specific layout
├── partials/
│   ├── header.tmpl         # Site header
│   ├── nav.tmpl            # Navigation
│   ├── footer.tmpl         # Site footer
│   ├── head.tmpl           # <head> contents (meta, css, etc.)
│   └── scripts.tmpl        # JavaScript includes
├── pages/
│   ├── index.tmpl          # Home page
│   ├── healthz.tmpl        # Health check page
│   └── error.tmpl          # Error pages (404, 500, etc.)
├── admin/
│   ├── dashboard.tmpl      # Admin dashboard
│   ├── settings.tmpl       # Settings page
│   └── ...
└── components/
    ├── modal.tmpl          # Reusable modal component
    ├── toast.tmpl          # Toast notifications
    └── ...
```

**Mandatory Partials (NON-NEGOTIABLE):**

ALL pages MUST use these partials to ensure consistent site-wide layout:

| Partial | Purpose | Required |
|---------|---------|----------|
| `header.tmpl` | Site header (logo, branding) | YES |
| `nav.tmpl` | Navigation menu | YES |
| `footer.tmpl` | Site footer (copyright, links) | YES |
| `head.tmpl` | `<head>` contents (meta, CSS) | YES |
| `scripts.tmpl` | JavaScript includes | YES |

**Page Structure (NON-NEGOTIABLE):**

```
┌─────────────────────────────────────────┐
│              <header>                   │  ← header.tmpl (logo, branding)
├─────────────────────────────────────────┤
│               <nav>                     │  ← nav.tmpl (TOP - navigation links)
├─────────────────────────────────────────┤
│                                         │
│              <main>                     │  ← Page content
│                                         │
├─────────────────────────────────────────┤
│              <footer>                   │  ← footer.tmpl (BOTTOM - info links)
└─────────────────────────────────────────┘
```

**Nav vs Footer (NON-NEGOTIABLE):**

| Element | Position | Purpose | Contents |
|---------|----------|---------|----------|
| `<nav>` | TOP | Navigation | Links to app sections, user menu |
| `<footer>` | BOTTOM | Information | About, Privacy, Contact, Help, GitHub, version |

**Nav contains (app navigation):**
- Home link
- App-specific sections (project-defined)
- User menu (right side):
  - If logged in: Username dropdown → Profile, Settings, Logout
  - If logged out: Login link

**Nav does NOT contain:**
- API link (users access via /openapi if needed)
- Admin link (don't advertise - admins know where it is)
- Help link (belongs in footer)

**Default Navigation (nav.tmpl):**

```
Desktop:
┌─────────────────────────────────────────────────────────────────┐
│  airports                                      [User Icon] │  ← Header
├─────────────────────────────────────────────────────────────────┤
│  Home  |  [App Section 1]  |  [App Section 2]  |  ...           │  ← Nav
└─────────────────────────────────────────────────────────────────┘

Mobile:
┌─────────────────────────────────────────────────────────────────┐
│  airports                                      [User Icon] │  ← Header
├─────────────────────────────────────────────────────────────────┤
│                                                      [☰ Menu]   │  ← Nav row
└─────────────────────────────────────────────────────────────────┘
                                              ┌───────────────────┐
                                              │  Home             │
                                              │  App Section 1    │  ← Slide-in
                                              │  App Section 2    │     from right
                                              │  ...              │
                                              └───────────────────┘
```

```html
<!-- Header bar: site name + user icon -->
<header class="header">
  <a href="/" class="site-brand">airports</a>

  <!-- User icon (always visible, far right) -->
  <div class="user-menu">
    {{ if .User }}
      <!-- Logged in: user icon dropdown -->
      <div class="dropdown">
        <button class="dropdown-toggle user-icon" aria-label="User menu">
          <svg>...</svg>
        </button>
        <div class="dropdown-menu">
          <span class="dropdown-header">{{ .User.Username }}</span>
          <a href="/user/profile">Profile</a>
          <a href="/user/settings">Settings</a>
          <hr />
          <a href="/auth/logout">Logout</a>
        </div>
      </div>
    {{ else }}
      <!-- Logged out: login icon -->
      <a href="/auth/login" class="user-icon" aria-label="Login">
        <svg>...</svg>
      </a>
    {{ end }}
  </div>
</header>

<!-- Nav bar: separate row below header -->
<nav class="nav" id="nav-menu">
  <!-- Desktop: inline links | Mobile: hamburger only -->
  <div class="nav-links">
    <a href="/">Home</a>
    <!-- App-specific sections (project-defined) -->
  </div>

  <!-- Mobile: hamburger menu toggle (far right of nav row) -->
  <button class="nav-toggle" aria-label="Toggle navigation">☰</button>
</nav>

<!-- Slide-in panel for mobile (hidden by default) -->
<div class="nav-panel" id="nav-panel">
  <a href="/">Home</a>
  <!-- App-specific sections (project-defined) -->
</div>

<!-- Overlay for mobile menu (click to close) -->
<div class="nav-overlay" onclick="closeNav()"></div>
```

**Mobile Menu Behavior:**
- Menu slides in from RIGHT edge
- Slides LEFT to open (right-to-left)
- Slides RIGHT to close (left-to-right)
- Overlay dims background, click to close
- User icon stays in header (NOT in menu) - keeps menu clean

**Smart Menu (NON-NEGOTIABLE):**
- If all nav links fit on screen → show inline links, NO hamburger
- If nav links overflow → show hamburger menu
- Detect dynamically on load and resize
- Don't show hamburger if not needed

**CSS Animation:**
```css
/* Mobile slide-in menu */
@media (max-width: 768px) {
  .nav {
    position: fixed;
    top: 0;
    right: -280px;           /* Hidden off-screen right */
    width: 280px;
    height: 100vh;
    transition: right 0.3s ease;
  }

  .nav.open {
    right: 0;                /* Slide in from right */
  }

  .nav-overlay {
    display: none;
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
  }

  .nav.open + .nav-overlay {
    display: block;
  }
}
```

**Mobile Responsive Rules:**
- Nav row below header: inline links or hamburger
- User icon ALWAYS in header (never in menu)
- Menu slides from right edge
- Touch-friendly: minimum 44x44px tap targets
- Overlay closes menu on tap

**No Fixed/Pinned Elements (NON-NEGOTIABLE):**
- Header, nav, footer all scroll with page
- NOTHING pinned/fixed to viewport
- User scrolls down → header/nav scroll away
- User scrolls to bottom → footer appears

**Footer Position (NON-NEGOTIABLE):**
- Footer ALWAYS at bottom of page (not floating in middle)
- If content is short → footer still at bottom of viewport
- If content is long → footer below content (scroll to see)
- Use min-height layout to push footer down

```css
/* Footer at bottom, no fixed elements */
body {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

main {
  flex: 1;  /* Grows to push footer to bottom */
}

/* NO position: fixed or position: sticky on header/nav/footer */
```

**Footer contains (informational links):**
- Standard pages: About, Privacy, Contact, Help
- External links: GitHub
- Branding: project name, version, copyright

**Rule:** Every page template MUST include header, nav, and footer partials. No page may define its own header/nav/footer - use the shared partials only. This ensures:
- Consistent branding across all pages
- Single point of change for navigation updates
- Uniform user experience

**App-Specific Partials (Optional):**

Projects can create additional partials for functionality unique to that application. Place these in `partials/` alongside the mandatory ones.

| Example Partial | Project | Purpose |
|-----------------|---------|---------|
| `search-box.tmpl` | airports, jokes | Reusable search form component |
| `airport-card.tmpl` | airports | Airport info display card |
| `joke-card.tmpl` | jokes | Joke display with copy button |
| `map.tmpl` | airports | Embedded map component |
| `passphrase-generator.tmpl` | wordList | Generator form and output |
| `geoip-result.tmpl` | airports | GeoIP lookup result display |
| `code-block.tmpl` | gitignore | Syntax-highlighted code display |
| `pagination.tmpl` | any | Reusable pagination controls |
| `filters.tmpl` | any | Search/filter form for lists |
| `stats-card.tmpl` | any | Statistics display card |

**App-Specific Partial Structure:**
```
src/server/templates/
├── partials/
│   ├── header.tmpl         # MANDATORY - site header
│   ├── nav.tmpl            # MANDATORY - navigation
│   ├── footer.tmpl         # MANDATORY - site footer
│   ├── head.tmpl           # MANDATORY - <head> contents
│   ├── scripts.tmpl        # MANDATORY - JS includes
│   ├── search-box.tmpl     # APP-SPECIFIC - search component
│   ├── result-card.tmpl    # APP-SPECIFIC - result display
│   └── pagination.tmpl     # APP-SPECIFIC - pagination controls
```

**Usage in page templates:**
```go
{{ define "content" }}
<div class="search-section">
  {{ template "search-box" . }}
</div>

<div class="results">
  {{ range .Results }}
    {{ template "result-card" . }}
  {{ end }}
</div>

{{ template "pagination" .Pagination }}
{{ end }}
```

**Guidelines for app-specific partials:**
- Create a partial when the same HTML is used in 2+ places
- Keep partials focused on one component/purpose
- Pass only the data the partial needs
- Name clearly: `{thing}-{purpose}.tmpl` (e.g., `airport-card.tmpl`, `joke-list.tmpl`)

**Embedding Templates (NON-NEGOTIABLE):**

All templates and static assets MUST be embedded in the binary:

```go
package server

import "embed"

//go:embed templates/*.tmpl templates/**/*.tmpl
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS
```

**Template Usage:**
```go
{{ define "base" }}
<!DOCTYPE html>
<html lang="en">
<head>{{ template "head" . }}</head>
<body>
  {{ template "header" . }}
  {{ template "nav" . }}
  <main>{{ template "content" . }}</main>
  {{ template "footer" . }}
  {{ template "scripts" . }}
</body>
</html>
{{ end }}
```

### Embedded vs External Assets

| Type | Embedded in Binary | External (Downloaded) |
|------|-------------------|----------------------|
| Templates (`.tmpl`) | YES | NO |
| CSS files | YES | NO |
| JavaScript files | YES | NO |
| Images/Icons | YES | NO |
| Fonts | YES | NO |
| Application data (JSON) | YES | NO |
| GeoIP databases | NO | YES - downloaded on first run |
| Blocklists | NO | YES - downloaded on first run |
| SSL certificates | NO | YES - only when using ports 80,443 |

**External Data Rules:**
- Security-related data that needs frequent updates is NOT embedded
- Downloaded automatically on first run
- Updated automatically via built-in scheduler
- Scheduler updates configurable via admin panel
- SSL certificates only generated/managed when running on ports `80,443`

**Benefits:**
- Single static binary deployment
- No external file dependencies at runtime
- Consistent layout across all pages
- Reusable components (DRY principle)
- Auto-escaping for security (XSS prevention)

### CSS Rules

| Bad | Good |
|-----|------|
| `<div style="color: red;">` | `<div class="error-text">` |
| `style="margin: 10px;"` | `class="spacing-sm"` |

**All styles MUST be in CSS files, not HTML elements.**

### Frontend UI Elements (NON-NEGOTIABLE)

**NEVER use default JavaScript UI elements. ALWAYS use custom styled components.**

| NEVER Use | ALWAYS Use Instead |
|-----------|---------------------|
| `alert()` | Custom modal with CSS classes |
| `confirm()` | Custom confirmation modal |
| `prompt()` | Custom input modal or inline form |
| Plain text inputs for options | Dropdowns (`<select>`) |
| Plain text for yes/no | Checkboxes or toggle switches |
| Plain text for multiple options | Radio buttons or dropdown |
| Inline text entry | Only when truly needed (search, names, etc.) |

**UI Element Guidelines:**

| Element | When to Use |
|---------|-------------|
| **Dropdown (`<select>`)** | Selecting from predefined options |
| **Checkbox** | Boolean on/off, enable/disable |
| **Toggle Switch** | Boolean with visual feedback |
| **Radio Buttons** | Mutually exclusive options (2-5 choices) |
| **Dropdown** | Mutually exclusive options (>5 choices) |
| **Multi-select** | Multiple selections from list |
| **Text Input** | Free-form text (names, URLs, search) |
| **Textarea** | Multi-line free-form text |
| **Number Input** | Numeric values with spin buttons |
| **Date/Time Picker** | Date and time selection |
| **Color Picker** | Color selection |
| **File Upload** | File selection with drag-drop |

**Modal Requirements:**
- Custom CSS-styled modals (no browser defaults)
- Backdrop overlay
- Close button (X) in corner
- Click outside to close (optional, configurable)
- Escape key to close
- Focus trap (tab stays within modal)
- Animated entrance/exit
- **Auto-close on action** - clicking any action button (OK, Yes, No, Cancel, Save, Delete, Submit, etc.) automatically closes the modal after performing the action. User should never need to click an action then manually close.

**Toast/Notification Requirements:**
- Non-blocking notifications
- Auto-dismiss with configurable timeout
- Manual dismiss option
- Stacking for multiple notifications
- Types: success, error, warning, info
- Icon + message format

## Layout

| Screen Size | Width |
|-------------|-------|
| ≥ 720px | 90% (5% margins) |
| < 720px | 98% (1% margins) |
| Footer | Always centered, always at bottom |

## Themes

| Theme | Description |
|-------|-------------|
| **Dark** | Based on Dracula - **DEFAULT** |
| **Light** | Based on popular light theme |
| **Auto** | Based on user's system |

## Branding & SEO (NON-NEGOTIABLE)

**White labeling is cosmetic only - it changes what users see, not how the system works.**

### What Branding Changes

| Changes (User-Visible) | Does NOT Change (System) |
|------------------------|--------------------------|
| Page titles | Directory names (`airports/`) |
| Browser tab | System username (`airports`) |
| Header/logo text | Log filenames |
| Footer branding | Config paths |
| Email "From" name | Binary name |
| SEO meta tags | API routes |
| OpenGraph data | Service names |
| Swagger UI title | Container names |

### Configuration

```yaml
server:
  branding:
    title: "airports"           # Display name (e.g., "Jokes API")
    tagline: ""                      # Short slogan (e.g., "The best jokes API")
    description: ""                  # Longer description for SEO/about

  seo:
    # Project-specific - define per app
    keywords: []                     # e.g., ["jokes", "api", "humor", "free api"]
    author: ""                       # Author/organization name
    og_image: ""                     # OpenGraph image URL for social sharing
    twitter_handle: ""               # Twitter @handle for cards
```

### Where Branding Is Used

| Field | Used In |
|-------|---------|
| `title` | `<title>` tag, header, emails, footer, Swagger UI |
| `tagline` | Homepage hero section, meta description fallback |
| `description` | Meta description, OpenGraph description, about page |
| `keywords` | Meta keywords tag |
| `author` | Meta author tag |
| `og_image` | OpenGraph/Twitter card image |
| `twitter_handle` | Twitter card attribution |

### SEO Meta Tags (Generated)

```html
<head>
  <title>{title} - {tagline}</title>
  <meta name="description" content="{description}">
  <meta name="keywords" content="{keywords}">
  <meta name="author" content="{author}">

  <!-- OpenGraph -->
  <meta property="og:title" content="{title}">
  <meta property="og:description" content="{description}">
  <meta property="og:image" content="{og_image}">
  <meta property="og:type" content="website">
  <meta property="og:url" content="{current_url}">

  <!-- Twitter Card -->
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:title" content="{title}">
  <meta name="twitter:description" content="{description}">
  <meta name="twitter:image" content="{og_image}">
  <meta name="twitter:site" content="{twitter_handle}">
</head>
```

### Static Files

| File | Purpose | Generated |
|------|---------|-----------|
| `/sitemap.xml` | Site map for search engines | Yes - auto-generated |
| `/favicon.ico` | Browser favicon | Embedded default, customizable |

### Admin Panel (/admin/server/branding)

| Element | Type | Description |
|---------|------|-------------|
| Title | Text input | Application display name |
| Tagline | Text input | Short slogan |
| Description | Textarea | Longer description for SEO |
| Keywords | Tag input | SEO keywords (comma-separated) |
| Author | Text input | Author/organization |
| OG Image | File upload / URL | Social sharing image |
| Twitter Handle | Text input | @handle |
| Favicon | File upload / URL | Custom favicon |
| Logo | File upload / URL | Custom logo (header) |

### Image Sources

**Logo, favicon, and OG image can be from local file or remote URL.**

| Source | Format | Example |
|--------|--------|---------|
| Local file | File upload | Upload via admin panel |
| Remote URL | URL input | `https://example.com/logo.png` |
| Embedded default | - | Built-in fallback |

### Image Scaling

**Images are automatically scaled/resized as needed:**

| Image | Sizes Generated |
|-------|-----------------|
| Logo | Original, 200px width (header), 50px width (mobile) |
| Favicon | 16x16, 32x32, 48x48, 180x180 (apple-touch-icon), 192x192, 512x512 |
| OG Image | Original, 1200x630 (OpenGraph standard) |

**Scaling Rules:**
- Preserve aspect ratio
- Generate multiple sizes on upload/fetch
- Cache scaled versions locally
- Re-fetch remote URLs periodically (configurable, default: daily)
- Fallback to embedded default if remote URL fails

### Defaults

| Field | Default Value |
|-------|---------------|
| `title` | `airports` |
| `tagline` | Empty |
| `description` | Empty |
| `keywords` | Empty |
| All others | Empty |

**Rule:** If `title` is empty, fall back to `airports`. Other fields are optional.

## Announcements (NON-NEGOTIABLE)

**Admin messages shown in UI for downtime notices, updates, etc.**

### Configuration

```yaml
web:
  announcements:
    enabled: true
    # List of announcement messages
    messages: []
```

### Announcement Structure

```yaml
messages:
  - id: "maintenance-2025-01"
    type: warning
    # warning, info, error, success
    title: "Scheduled Maintenance"
    message: "The system will be down for maintenance on Jan 15, 2025 from 2-4 AM UTC."
    start: "2025-01-14T00:00:00Z"
    # When to start showing
    end: "2025-01-15T04:00:00Z"
    # When to stop showing
    dismissible: true
    # User can dismiss
```

### Admin Panel (/admin/server/announcements)

| Element | Type | Description |
|---------|------|-------------|
| Enable announcements | Toggle | Turn announcements on/off |
| Announcement list | Table | All announcements |
| Add announcement | Button | Create new announcement |
| Type | Dropdown | warning, info, error, success |
| Title | Text input | Short title |
| Message | Textarea | Full message content |
| Start date | Datetime picker | When to start showing |
| End date | Datetime picker | When to stop showing |
| Dismissible | Toggle | Allow users to dismiss |
| Delete | Button | Remove announcement |

## CORS (NON-NEGOTIABLE)

**Default CORS policy allows all origins (`*`).**

### Configuration

```yaml
web:
  # CORS origin configuration
  # - "*": Allow all origins (default)
  # - "https://example.com": Single origin
  # - "https://example.com,https://app.example.com": Multiple origins (comma-separated)
  # - "": Disable CORS headers entirely
  cors: "*"
```

### CORS Headers

| Header | Value |
|--------|-------|
| `Access-Control-Allow-Origin` | Configured origin(s) or `*` |
| `Access-Control-Allow-Methods` | `GET, POST, PUT, PATCH, DELETE, OPTIONS` |
| `Access-Control-Allow-Headers` | `*` |
| `Access-Control-Allow-Credentials` | `true` (only when specific origin, not `*`) |
| `Access-Control-Max-Age` | `86400` (24 hours) |

### Behavior

| Scenario | Behavior |
|----------|----------|
| `cors: "*"` | Allow all origins, credentials NOT allowed |
| `cors: "https://example.com"` | Allow single origin, credentials allowed |
| `cors: "https://a.com,https://b.com"` | Allow listed origins, credentials allowed |
| `cors: ""` | No CORS headers (same-origin only) |
| Preflight (OPTIONS) | Return CORS headers, 204 No Content |

### Mode-Specific Behavior

| Mode | Default | Behavior |
|------|---------|----------|
| Production | `*` | Allow all origins by default (configure if needed) |
| Development | `*` | Allow all origins |

### Admin Panel (/admin/server/web)

| Element | Type | Description |
|---------|------|-------------|
| CORS Origins | Text input | Comma-separated list of allowed origins |
| Allow All | Toggle | Quick toggle for `*` (all origins) |
| Preview | Read-only | Shows resulting CORS headers |

## CSRF Protection (NON-NEGOTIABLE)

**ALL forms MUST have CSRF protection.**

### Configuration

```yaml
web:
  csrf:
    enabled: true
    # Token length in bytes
    token_length: 32
    cookie_name: csrf_token
    header_name: X-CSRF-Token
    # Secure cookie: auto, true, false
    secure: auto
```

### Implementation

- All forms include hidden CSRF token field
- All non-GET requests validate CSRF token
- Token stored in cookie and must match form/header value
- Tokens regenerated on login

## Footer Customization (NON-NEGOTIABLE)

### Configuration

```yaml
web:
  footer:
    # Google Analytics tracking ID (empty = disabled)
    # Example: UA-936146-1 or G-XXXXXXXXXX
    tracking_id: ""

    # Cookie consent popup (EU GDPR compliance)
    cookie_consent:
      enabled: true
      message: "This site uses cookies for functionality and analytics."
      policy_url: ""
      # URL to privacy policy

    # Custom branding HTML above the Application Footer
    # - Not set or "": Use default branding (built-in)
    # - " " (space): Disable branding, show only Application Footer
    # - Custom HTML: Use your own branding
    custom_html: ""
```

### Available Footer Variables

| Variable | Description |
|----------|-------------|
| `{currentyear}` | Current year (e.g., 2025) |
| `airports` | Project name |
| `apimgr` | Organization name |
| `{projectversion}` | Application version |
| `{builddatetime}` | Build date/time |

### Default Application Footer (Always Shown)

```html
<footer class="footer">
  <!-- Standard page links (always first) -->
  <p>
    <a href="/server/about">About</a>
    <span>•</span>
    <a href="/server/privacy">Privacy</a>
    <span>•</span>
    <a href="/server/contact">Contact</a>
    <span>•</span>
    <a href="/server/help">Help</a>
  </p>

  <br />

  <!-- Application branding -->
  <p>
    <a href="https://github.com/apimgr/airports" target="_blank">airports</a>
    <span>•</span>
    <span>Made with ❤️</span>
    <span>•</span>
    <span>{projectversion}</span>
  </p>

  <br />

  <a href="/healthz">Last update: {builddatetime}</a>
</footer>
```

### Admin Panel (/admin/server/footer)

| Element | Type | Description |
|---------|------|-------------|
| Google Analytics ID | Text input | Tracking ID (empty = disabled) |
| Cookie consent enabled | Toggle | Show cookie consent popup |
| Cookie consent message | Textarea | Consent message |
| Privacy policy URL | Text input | Link to privacy policy |
| Custom branding HTML | Textarea | HTML above application footer |
| Preview | Preview pane | Shows rendered footer |

## Standard Pages (NON-NEGOTIABLE)

**ALL applications MUST have these standard pages. Content is defined per-application.**

### /server/about

**About the application - what it does, version info, and optionally Tor access.**

| Section | Description |
|---------|-------------|
| Application name | From branding config |
| Version | Current version |
| Description | From branding config or project-specific |
| Features | Key features list (project-specific) |
| Tor access | If Tor enabled, show .onion address with copy button |
| Links | GitHub, documentation, etc. |

**Tor Section (shown only if `server.tor.enabled: true`):**

```html
<!-- Example Tor section -->
<div class="tor-access">
  <h3>Tor Hidden Service</h3>
  <p>This application is also available via Tor for enhanced privacy.</p>
  <div class="onion-address">
    <code>{onion_address}</code>
    <button onclick="copyToClipboard()">[Copy]</button>
  </div>
  <p class="note">Requires Tor Browser or Tor-enabled browser.</p>
</div>
```

### /server/privacy

**Privacy policy - what data is collected, how it's used, retention, etc.**

| Section | Description |
|---------|-------------|
| Data collection | What data is collected |
| Data usage | How data is used |
| Data retention | How long data is kept |
| Third parties | What data is shared with third parties |
| Cookies | What cookies are used |
| User rights | How users can access/delete their data |
| Contact | How to contact for privacy concerns |

**Default template provided, customizable via admin panel.**

### /server/contact

**Contact form - sends message to admin or dedicated contact address.**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Name | Text | Yes | Sender's name |
| Email | Email | Yes | Sender's email (for reply) |
| Subject | Text | Yes | Message subject |
| Message | Textarea | Yes | Message body |
| Captcha | Captcha | Yes | Spam prevention |

**Submission sends email to `server.contact` address (or admin email if not set).**

### /server/help

**Help page - documentation and usage instructions for the application.**

| Section | Description |
|---------|-------------|
| Getting started | Quick start guide |
| Features | How to use key features |
| API Documentation | Links to Swagger (/openapi) and GraphQL (/graphql) - both in sync |
| FAQ | Frequently asked questions |
| Troubleshooting | Common issues and solutions |

**API Documentation section (always shown):**
```html
<div class="api-docs">
  <h3>API Documentation</h3>
  <p>This application provides a full REST API with interactive documentation.</p>
  <ul>
    <li><a href="/openapi">Swagger UI</a> - Interactive REST API explorer</li>
    <li><a href="/graphql">GraphiQL</a> - Interactive GraphQL explorer</li>
  </ul>
</div>
```

**Content is project-specific. Markdown supported.**

### Configuration

```yaml
server:
  # Contact form recipient
  # If not set, uses admin email
  contact: ""

  pages:
    about:
      # Additional content for about page (markdown supported)
      content: ""
      # Show Tor section (auto-detected from tor.enabled, but can override)
      show_tor: auto

    privacy:
      # Privacy policy content (markdown supported)
      # If empty, uses default template
      content: ""

    contact:
      # Enable contact form
      enabled: true
      # Recipient email (if empty, uses server.contact or admin email)
      recipient: ""
      # Captcha type: recaptcha, hcaptcha, simple (built-in)
      captcha: simple
      # Success message after form submission
      success_message: "Thank you for your message. We'll respond soon."

    help:
      # Help page content (markdown supported)
      # Project-specific - must be defined per application
      content: ""
```

### Admin Panel (/admin/server/pages)

| Element | Type | Description |
|---------|------|-------------|
| **About Page** | | |
| Content | Markdown editor | Additional about page content |
| Show Tor section | Toggle | Show .onion address (auto/yes/no) |
| Preview | Button | Preview about page |
| **Privacy Policy** | | |
| Content | Markdown editor | Privacy policy content |
| Reset to default | Button | Restore default template |
| Preview | Button | Preview privacy page |
| **Contact Page** | | |
| Enable contact form | Toggle | Enable/disable contact form |
| Recipient email | Text input | Email to receive messages |
| Captcha type | Dropdown | recaptcha, hcaptcha, simple |
| Success message | Textarea | Message shown after submission |
| Test form | Button | Send test message |
| **Help Page** | | |
| Content | Markdown editor | Help/documentation content |
| Preview | Button | Preview help page |

---

# PART 14: API STRUCTURE (NON-NEGOTIABLE)

## API Versioning

**Use versioned API: `/api/v1`**

## API Types

**ALL PROJECTS GET ALL THREE:**

| Type | Required |
|------|----------|
| REST API | YES (primary) |
| Swagger/OpenAPI | YES |
| GraphQL | YES |

### Swagger & GraphQL Sync (NON-NEGOTIABLE)

**Swagger and GraphQL MUST always be in sync with each other AND with the project's API.**

| Rule | Description |
|------|-------------|
| **Sync with project** | Both MUST reflect current project API at all times |
| **Sync with each other** | Both Swagger and GraphQL expose identical functionality |
| **Auto-generated** | Both specs generated from code (annotations, comments, or schema) |
| **Never manual** | NEVER manually edit OpenAPI JSON or GraphQL schema |
| **Build-time generation** | Specs regenerated on every build |
| **JSON only** | OpenAPI spec uses JSON format only (NO YAML) |
| **Single source of truth** | Code is the source, specs are generated output |
| **Match site theme** | Both UIs styled to match site theme (Dracula dark) |

**Sync Flow:**
```
Code Changes (handlers, routes, types)
         │
         ▼
Build Process
         │
         ├──► Generate OpenAPI JSON from code annotations
         │
         └──► Generate GraphQL schema from code annotations
         │
         ▼
Both specs automatically in sync with project
```

**Implementation:**
- Use Go struct tags and comments for documentation
- Generate OpenAPI spec at build time (e.g., swaggo/swag)
- Generate GraphQL schema from same structs
- Embed generated specs in binary
- Serve at runtime from embedded files

**Why both?**
- Swagger: Industry standard, tooling support, code generation
- GraphQL: Flexible queries, reduced over-fetching, modern clients

**NEVER:**
- Manually edit `openapi.json`
- Manually edit GraphQL schema files
- Let specs drift from actual API
- Have endpoints not documented in both

### Swagger & GraphQL Theming (NON-NEGOTIABLE)

**Both Swagger UI and GraphiQL MUST match the site theme while remaining easy to read.**

| Element | Style |
|---------|-------|
| Background | Dark (`#282a36` - Dracula background) |
| Text | Light (`#f8f8f2` - Dracula foreground) |
| Links/Accents | Dracula colors (cyan, green, purple) |
| Code blocks | Syntax highlighted (Dracula scheme) |
| Inputs | Dark background, light text, visible borders |
| Buttons | Match site button styles |

**Swagger UI Customization:**

```css
/* Swagger UI - Dracula Theme */
.swagger-ui {
  background: #282a36;
  color: #f8f8f2;
}

.swagger-ui .topbar {
  background: #1e1f29;
}

.swagger-ui .info .title,
.swagger-ui .opblock-tag {
  color: #f8f8f2;
}

.swagger-ui .opblock.opblock-get {
  background: rgba(139, 233, 253, 0.1);
  border-color: #8be9fd;
}

.swagger-ui .opblock.opblock-post {
  background: rgba(80, 250, 123, 0.1);
  border-color: #50fa7b;
}

.swagger-ui .opblock.opblock-put {
  background: rgba(255, 184, 108, 0.1);
  border-color: #ffb86c;
}

.swagger-ui .opblock.opblock-delete {
  background: rgba(255, 85, 85, 0.1);
  border-color: #ff5555;
}

.swagger-ui input, .swagger-ui textarea, .swagger-ui select {
  background: #44475a;
  color: #f8f8f2;
  border: 1px solid #6272a4;
}

.swagger-ui .btn {
  background: #6272a4;
  color: #f8f8f2;
}
```

**GraphiQL Customization:**

```css
/* GraphiQL - Dracula Theme */
.graphiql-container {
  background: #282a36;
  color: #f8f8f2;
}

.graphiql-container .CodeMirror {
  background: #282a36;
  color: #f8f8f2;
}

.graphiql-container .CodeMirror-gutters {
  background: #1e1f29;
  border-right: 1px solid #44475a;
}

.graphiql-container .result-window {
  background: #282a36;
}

.graphiql-container .execute-button {
  background: #50fa7b;
  color: #282a36;
}

.graphiql-container .toolbar-button {
  background: #44475a;
  color: #f8f8f2;
}
```

**Theming Rules:**
- NEVER use default light themes
- MUST match site's Dracula dark theme
- Text MUST be readable (sufficient contrast)
- Syntax highlighting MUST use Dracula colors
- Interactive elements MUST be clearly visible

## Root-Level Endpoints (NON-NEGOTIABLE)

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/` | GET | None | Web interface (HTML) |
| `/healthz` | GET | None | Health check (HTML) |
| `/openapi` | GET | None | Swagger UI |
| `/openapi.json` | GET | None | OpenAPI spec (JSON only) |
| `/graphql` | GET | None | GraphiQL interface |
| `/graphql` | POST | None | GraphQL queries |
| `/metrics` | GET | Optional | Prometheus metrics |
| `/admin` | GET | Session | Admin panel login |
| `/admin/*` | ALL | Session | Admin panel pages |
| `/api/v1/healthz` | GET | None | Health check (JSON) |
| `/api/v1/admin/*` | ALL | Bearer | Admin API |

**NOTE: No `/openapi.yaml` endpoint. JSON only.**

## Response Standards

| Route Type | Response Format |
|------------|-----------------|
| `/` routes | HTML |
| `/api` routes | JSON (default) or text |
| `/api/**/*.txt` | Text |

### Error Response Format

```json
{
  "error": "Human readable message",
  "code": "ERROR_CODE",
  "status": 400,
  "details": {}
}
```

### Pagination (default: 250 items)

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "limit": 250,
    "total": 1000,
    "pages": 4
  }
}
```

---

# CHECKPOINT 5: FRONTEND & API VERIFICATION

Before proceeding, confirm you understand:
- [ ] Frontend is required for ALL projects
- [ ] NO inline CSS, NO JS alerts
- [ ] Dark theme (Dracula) is default
- [ ] All 3 API types required: REST, Swagger, GraphQL (Swagger & GraphQL in sync)
- [ ] Standard endpoints must exist (/healthz, /openapi, /openapi.json, /graphql, /admin)
- [ ] OpenAPI uses JSON only (no YAML)

---

# PART 15: ADMIN PANEL (NON-NEGOTIABLE)

**ALL projects MUST have a full admin panel.**

## Admin Panel Isolation (NON-NEGOTIABLE)

**The admin panel is completely isolated from the public site.**

| Rule | Description |
|------|-------------|
| **NEVER link to /admin** | No links to /admin on ANY public routes (`/**`) |
| **Intentional access only** | Users must manually type `/admin` in browser |
| **Separate authentication** | Admin account is ONLY valid for `/admin/**` routes |
| **No admin mentions** | Don't advertise admin panel existence anywhere |
| **Separate session** | Admin session is separate from user sessions |

### User Types

| User Type | Valid Routes | Authentication |
|-----------|--------------|----------------|
| **Admin** | `/admin/**` ONLY | Admin credentials (username/password) |
| **Guest/Anon** | `/**` (except /admin) | None |
| **Normal User** | `/**` (except /admin) | User account (if multi-user enabled) |

**Admin credentials are stored in `server.db` (admin_credentials table), NOT in config file.**

### Why Isolated?

- Security: Admin panel not discoverable
- Separation: Admin functions separate from user functions
- Simplicity: No confusion between admin and user roles
- Protection: Reduces attack surface

## Design Principles

| Principle | Description |
|-----------|-------------|
| Server Admin Focus | Designed for server administration, not end-users |
| Pretty | Clean, modern, professional design |
| Intuitive | Self-explanatory, no manual needed |
| Easy Navigation | Logical grouping, breadcrumbs, search |
| Frontend Rules | Dracula theme (default), responsive, accessible |
| No JS Alerts | Custom modals, toasts, confirmations |
| Real-time Feedback | Show save status, validation errors inline |
| Mobile-Friendly | Works on all screen sizes |
| Keyboard Shortcuts | Power users can navigate quickly |

## Admin Panel Layout (NON-NEGOTIABLE)

### Overall Structure

```
┌─────────────────────────────────────────────────────────────────────────┐
│ HEADER                                                                   │
│ ┌─────────────────────────────────────────────────────────────────────┐ │
│ │ Logo/Title          Search...              [Status] [User] [Logout] │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
├──────────────┬──────────────────────────────────────────────────────────┤
│   SIDEBAR    │                    MAIN CONTENT                          │
│              │                                                          │
│  Dashboard   │  ┌─────────────────────────────────────────────────────┐ │
│              │  │ Breadcrumb: Dashboard > Server > Settings           │ │
│  Server ▼    │  └─────────────────────────────────────────────────────┘ │
│   Settings   │                                                          │
│   SSL/TLS    │  ┌─────────────────────────────────────────────────────┐ │
│   Scheduler  │  │                                                     │ │
│   Logs       │  │              PAGE CONTENT                           │ │
│              │  │                                                     │ │
│  Security ▼  │  │                                                     │ │
│   Auth       │  │                                                     │ │
│   Tokens     │  │                                                     │ │
│   Firewall   │  │                                                     │ │
│              │  │                                                     │ │
│  Network ▼   │  │                                                     │ │
│   Tor        │  │                                                     │ │
│   GeoIP      │  │                                                     │ │
│              │  └─────────────────────────────────────────────────────┘ │
│  Users ▼     │                                                          │
│  (if multi)  │  ┌─────────────────────────────────────────────────────┐ │
│              │  │ FOOTER: Version | Docs | Status                     │ │
│  System ▼    │  └─────────────────────────────────────────────────────┘ │
│   Backup     │                                                          │
│   Info       │                                                          │
└──────────────┴──────────────────────────────────────────────────────────┘
```

### Header

| Element | Position | Description |
|---------|----------|-------------|
| Logo/Title | Left | Project name, clickable → dashboard |
| Search | Center | Global search (settings, logs, etc.) |
| Status Indicator | Right | ● Green (OK), ● Yellow (Warning), ● Red (Error) |
| Admin Name | Right | Current admin username |
| Logout | Right | Always visible, one-click logout |

### Sidebar Navigation

**Collapsible sidebar with grouped sections.**

```
📊 Dashboard

📦 Server
   ├── Settings
   ├── Branding
   ├── SSL/TLS
   ├── Scheduler
   ├── Email
   └── Logs

🔒 Security
   ├── Authentication
   ├── API Tokens
   ├── Rate Limiting
   └── Firewall

🌐 Network
   ├── Tor
   ├── GeoIP
   └── Blocklists

👥 Users (if multi-user)
   ├── User List
   ├── Invites
   └── Roles

🖥️ System
   ├── Backup & Restore
   ├── Maintenance
   ├── Updates
   └── System Info

🔗 Cluster (if enabled)
   ├── Nodes
   ├── Add Node
   └── Settings

❓ Help
   └── Documentation
```

### Sidebar Behavior

| Feature | Description |
|---------|-------------|
| Collapsible | Click section header to expand/collapse |
| Active indicator | Highlight current page |
| Collapse all | Double-click header to collapse sidebar |
| Remember state | Persist expanded/collapsed state |
| Icons | Each section has icon for quick recognition |
| Mobile | Hamburger menu, slide-out drawer |

## /admin (Web Interface)

### Authentication

| Feature | Description |
|---------|-------------|
| Login page | `/admin` (when not logged in) |
| Login form | Username/password, centered card |
| Session | Cookie-based (30 days default, configurable) |
| CSRF | Protection on all forms |
| Remember Me | Option available (extends to 90 days) |
| Logout | Always visible in header |
| MFA | TOTP support (optional, configurable) |

### Login Page (`/admin`)

```
┌─────────────────────────────────────────┐
│                                         │
│         ┌───────────────────┐           │
│         │   {Project Name}  │           │
│         │   Admin Panel     │           │
│         ├───────────────────┤           │
│         │                   │           │
│         │  Username: [____] │           │
│         │  Password: [____] │           │
│         │                   │           │
│         │  [ ] Remember me  │           │
│         │                   │           │
│         │    [  Login  ]    │           │
│         │                   │           │
│         └───────────────────┘           │
│                                         │
│              v1.2.3                     │
└─────────────────────────────────────────┘
```

**Login page rules:**
- Centered card on dark background
- Project name/logo at top
- No links to public site
- Version number at bottom (small)
- No "Forgot password" (admin resets via CLI if needed)

### Dashboard (`/admin/dashboard`)

**Overview of system status at a glance.**

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Dashboard                                                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │   STATUS     │  │   UPTIME     │  │   REQUESTS   │  │   ERRORS     │ │
│  │   ● Online   │  │   5d 12h 3m  │  │   12,345     │  │   23         │ │
│  │              │  │              │  │   (24h)      │  │   (24h)      │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘ │
│                                                                         │
│  ┌─────────────────────────────────┐  ┌─────────────────────────────┐   │
│  │ SYSTEM RESOURCES                │  │ QUICK ACTIONS               │   │
│  │                                 │  │                             │   │
│  │ CPU:    [████████░░] 78%        │  │ [Restart Server]            │   │
│  │ Memory: [██████░░░░] 62%        │  │ [Clear Cache]               │   │
│  │ Disk:   [████░░░░░░] 45%        │  │ [Create Backup]             │   │
│  │                                 │  │ [View Logs]                 │   │
│  └─────────────────────────────────┘  └─────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────┐  ┌─────────────────────────────┐   │
│  │ RECENT ACTIVITY                 │  │ SCHEDULED TASKS             │   │
│  │                                 │  │                             │   │
│  │ 10:30 Config updated            │  │ SSL Renewal    in 23 days   │   │
│  │ 10:15 Admin login               │  │ GeoIP Update   in 2 days    │   │
│  │ 09:45 Backup completed          │  │ Auto Backup    in 5 hours   │   │
│  │ 09:00 SSL renewed               │  │ Session Clean  in 45 min    │   │
│  └─────────────────────────────────┘  └─────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ ALERTS / WARNINGS                                                │    │
│  │                                                                  │    │
│  │ ⚠️  SSL certificate expires in 23 days                          │    │
│  │ ⚠️  Disk usage above 80% threshold                              │    │
│  │ ℹ️  Update available: v1.2.4                                    │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Dashboard Widgets

| Widget | Content |
|--------|---------|
| Status | Online/Maintenance/Error indicator |
| Uptime | Time since last restart |
| Requests | Request count (24h) |
| Errors | Error count (24h) |
| System Resources | CPU, Memory, Disk usage bars |
| Quick Actions | Common admin tasks |
| Recent Activity | Last 5-10 audit log entries |
| Scheduled Tasks | Next scheduled tasks |
| Alerts | Warnings and notifications |

### Required Admin Pages

| Route | Page | Description |
|-------|------|-------------|
| `/admin` | Login | Login form (if not authenticated) |
| `/admin/dashboard` | Dashboard | Overview, stats, quick actions |
| `/admin/server/settings` | Server Settings | Port, mode, FQDN, etc. |
| `/admin/server/branding` | Branding | Title, logo, favicon, colors |
| `/admin/server/ssl` | SSL/TLS | Certificates, Let's Encrypt |
| `/admin/server/scheduler` | Scheduler | View/edit scheduled tasks |
| `/admin/server/email` | Email | SMTP settings, templates |
| `/admin/server/logs` | Logs | View access, error, audit logs |
| `/admin/security/auth` | Authentication | Password, MFA, sessions |
| `/admin/security/tokens` | API Tokens | Generate, revoke tokens |
| `/admin/security/ratelimit` | Rate Limiting | Configure rate limits |
| `/admin/security/firewall` | Firewall | IP allow/block lists |
| `/admin/network/tor` | Tor | Enable/disable, onion address |
| `/admin/network/geoip` | GeoIP | Country blocking, database updates |
| `/admin/network/blocklists` | Blocklists | IP/domain blocklists |
| `/admin/users` | Users | User list (if multi-user) |
| `/admin/users/invites` | Invites | Invite codes (if multi-user) |
| `/admin/system/backup` | Backup | Create/restore backups |
| `/admin/system/maintenance` | Maintenance | Maintenance mode |
| `/admin/system/updates` | Updates | Check/apply updates |
| `/admin/system/info` | System Info | Version, environment, deps |
| `/admin/cluster/nodes` | Nodes | Cluster node management |
| `/admin/cluster/add` | Add Node | Generate join token |
| `/admin/help` | Help | Documentation links |

### Settings Page Layout

**Standard layout for all settings pages.**

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Server Settings                                              [Save All] │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  General                                                                │
│  ─────────────────────────────────────────────────────────────────────  │
│                                                                         │
│  Port                                                                   │
│  [64580        ]  ⓘ The port the server listens on                     │
│                   ⚠️ Requires restart                                   │
│                                                                         │
│  Mode                                                                   │
│  [Production ▼]   ⓘ Production enforces strict host validation         │
│                                                                         │
│  FQDN                                                                   │
│  [api.example.com]  ⓘ Fully qualified domain name (auto-detected)      │
│                                                                         │
│  ─────────────────────────────────────────────────────────────────────  │
│                                                                         │
│  Advanced                                                    [Expand ▼] │
│  ─────────────────────────────────────────────────────────────────────  │
│                                                                         │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                                            [Cancel] [Save]      │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Form Elements

| Element | Usage |
|---------|-------|
| Text input | Single-line values |
| Textarea | Multi-line values (JSON, lists) |
| Toggle | Boolean on/off |
| Dropdown | Selection from list |
| Tag input | Multiple values (keywords, IPs) |
| File upload | Logos, certificates |
| Number input | Ports, limits |
| Password input | Secrets (with show/hide) |

### Form Behavior

| Feature | Description |
|---------|-------------|
| Tooltips | ⓘ icon shows help on hover |
| Validation | Real-time, inline error messages |
| Unsaved indicator | Show when form has unsaved changes |
| Save feedback | Toast notification on save |
| Confirm dangerous | Modal for destructive actions |
| Restart warning | ⚠️ icon if setting requires restart |
| Default values | Show default in placeholder |

### Log Viewer (`/admin/server/logs`)

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Logs                                                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  [Access ▼]  [Last 100 ▼]  [Search...        ]  [Auto-refresh: ON]     │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ 2025-01-15 10:30:45  GET  /api/v1/health  200  12ms  192.168.1.1│    │
│  │ 2025-01-15 10:30:44  POST /api/v1/data    201  45ms  192.168.1.2│    │
│  │ 2025-01-15 10:30:43  GET  /healthz        200  2ms   192.168.1.1│    │
│  │ 2025-01-15 10:30:42  GET  /api/v1/users   401  5ms   10.0.0.50  │    │
│  │ ...                                                              │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                         │
│  [< Prev]  Page 1 of 50  [Next >]           [Download] [Clear Logs]    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Log Types

| Log | Description |
|-----|-------------|
| Access | HTTP request logs |
| Error | Application errors |
| Audit | Security/admin events |
| Security | Auth failures, blocked IPs |
| Debug | Debug output (dev mode) |

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `g d` | Go to Dashboard |
| `g s` | Go to Settings |
| `g l` | Go to Logs |
| `/` | Focus search |
| `Esc` | Close modal/menu |
| `Ctrl+S` | Save current form |
| `?` | Show shortcuts help |

## /admin Authentication Flow

```
User visits /admin
       │
       ▼
Check for valid admin session
       │
       ├─► No session/expired
       │   │
       │   ▼
       │   Show login form
       │   │
       │   ▼
       │   User submits credentials
       │   │
       │   ▼
       │   Validate against admin_credentials table
       │   │
       │   ├─► Invalid: Show error, log attempt
       │   │
       │   └─► Valid: Create session, redirect to dashboard
       │
       └─► Valid session
           │
           ▼
           Show requested admin page
```

## Admin Session vs User Session

| Aspect | Admin Session | User Session |
|--------|---------------|--------------|
| Cookie name | `admin_session` | `user_session` |
| Valid routes | `/admin/**` only | `/**` except `/admin/**` |
| Stored in | `server.db` (admin_sessions) | `users.db` (user_sessions) |
| Credentials | admin_credentials table | user_accounts table |
| Default duration | 30 days | 7 days |
| MFA | Optional (TOTP) | Optional (TOTP) |

### Scheduler Management (Admin Panel)

The admin panel MUST include a scheduler section with:

| Feature | Description |
|---------|-------------|
| **Task List** | View all scheduled tasks with status |
| **Next Run** | Show next scheduled run time for each task |
| **Last Run** | Show last run time and result (success/failure) |
| **Run History** | View history of past runs with timestamps |
| **Manual Trigger** | Button to manually run any task |
| **Enable/Disable** | Toggle tasks on/off |
| **Edit Schedule** | Modify task frequency (cron-style or preset) |
| **Task Details** | View task configuration and logs |

**Preset Schedules:**
- `hourly` - Every hour
- `daily` - Once per day (configurable time)
- `weekly` - Once per week (configurable day/time)
- `monthly` - Once per month (configurable day/time)
- `custom` - Cron expression

## /api/v1/admin (REST API)

### Authentication

`Authorization: Bearer {token}`

### Required Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/config` | GET | Get full config |
| `/api/v1/admin/config` | PUT | Update full config |
| `/api/v1/admin/config` | PATCH | Partial update |
| `/api/v1/admin/status` | GET | Server status |
| `/api/v1/admin/health` | GET | Detailed health |
| `/api/v1/admin/stats` | GET | Statistics |
| `/api/v1/admin/logs/access` | GET | Access logs |
| `/api/v1/admin/logs/error` | GET | Error logs |
| `/api/v1/admin/backup` | POST | Create backup |
| `/api/v1/admin/restore` | POST | Restore backup |
| `/api/v1/admin/test/email` | POST | Send test email |
| `/api/v1/admin/password` | POST | Change password |
| `/api/v1/admin/token/regenerate` | POST | Regenerate API token |

---

# PART 16: EMAIL TEMPLATES (NON-NEGOTIABLE)

## Overview

**ALL projects MUST have customizable email templates.**

Email templates allow server admins to customize ALL notification messages, including account-related emails (password reset, email verification, login alerts, etc.). Default templates with sane defaults are embedded in the binary; custom templates are stored in `{config_dir}/templates/email/`.

**Key Points:**
- ALL email templates are fully customizable via the admin panel
- Account emails (password reset, verification, security alerts) follow the same customization pattern as system emails
- Each template has sensible defaults that work out-of-the-box
- Changes take effect immediately (live reload)

## Template Storage

| Type | Location |
|------|----------|
| Default templates | Embedded in binary (`src/templates/email/`) |
| Custom templates | `{config_dir}/templates/email/` |

**Behavior:**
- If custom template exists → use custom
- If not → fall back to embedded default
- Reset to default → delete custom file

## SMTP Requirement (NON-NEGOTIABLE)

**ALL emails require a valid and working SMTP server. No SMTP = No emails. Don't even try.**

| Rule | Description |
|------|-------------|
| **No SMTP configured** | Email functionality completely disabled |
| **SMTP configured but invalid** | Validate on save, reject invalid config |
| **SMTP configured and working** | Email functionality enabled |

**Behavior When No SMTP:**

| Feature | Behavior |
|---------|----------|
| Password reset | Feature hidden/disabled, show "Contact administrator" |
| Email verification | Skipped entirely (emails auto-verified) |
| Login alerts | Not sent, not attempted, not logged |
| Welcome email | Not sent, not attempted |
| Security alerts | Not sent, not attempted |
| All notifications | Not sent, not attempted |

**DO NOT:**
- ❌ Attempt to send emails without valid SMTP
- ❌ Queue emails hoping SMTP will be configured later
- ❌ Log "would have sent email" messages
- ❌ Show email-related options if SMTP not configured

**DO:**
- ✓ Check SMTP status once at startup and on config change
- ✓ Completely disable email features if no SMTP
- ✓ Hide email-dependent UI elements when SMTP unavailable
- ✓ Show clear message: "Email features require SMTP configuration"

**Admin Panel:**
- If SMTP not configured, show banner: "⚠️ SMTP not configured. Email features disabled. [Configure SMTP](/admin/server/email)"
- Email-dependent features (password reset link, etc.) hidden until SMTP configured
- Test email button validates SMTP actually works before enabling email features

## Default Templates

| Template | Purpose | Account Email? |
|----------|---------|:--------------:|
| `welcome` | New user registration / admin setup | ✓ |
| `password_reset` | Password reset request | ✓ |
| `email_verify` | Email address verification | ✓ |
| `login_alert` | New login detected | ✓ |
| `security_alert` | Security event (failed logins, etc.) | ✓ |
| `2fa_enabled` | 2FA activated on account | ✓ |
| `2fa_disabled` | 2FA removed from account | ✓ |
| `password_changed` | Password was changed | ✓ |
| `backup_complete` | Backup finished successfully | ✗ |
| `backup_failed` | Backup error | ✗ |
| `ssl_expiring` | Certificate expiration warning | ✗ |
| `ssl_renewed` | Certificate renewed successfully | ✗ |
| `scheduler_error` | Scheduled task failed | ✗ |
| `test` | Test email | ✗ |

**Account Email (✓):** Must follow Account Email Requirements (visible link, disclaimer, etc.)

## Sane Defaults (NON-NEGOTIABLE)

**ALL email templates MUST have sensible defaults that work immediately without configuration.**

| Template | Default Subject | Default Behavior |
|----------|-----------------|------------------|
| `welcome` | `Welcome to {app_name}` | Sent to new users on registration (if enabled) and to admin on first setup |
| `password_reset` | `Password Reset Request - {app_name}` | 24-hour expiry, includes IP address |
| `email_verify` | `Verify Your Email - {app_name}` | 48-hour expiry |
| `login_alert` | `New Login Detected - {app_name}` | Includes IP, location (if GeoIP enabled), device |
| `security_alert` | `Security Alert - {app_name}` | Generic alert for various security events |
| `2fa_enabled` | `Two-Factor Authentication Enabled - {app_name}` | Confirmation of 2FA activation |
| `2fa_disabled` | `Two-Factor Authentication Disabled - {app_name}` | Warning about 2FA removal |
| `password_changed` | `Your Password Was Changed - {app_name}` | Confirmation of password change |
| `backup_complete` | `Backup Complete - {app_name}` | Includes filename and size |
| `backup_failed` | `Backup Failed - {app_name}` | Includes error message |
| `ssl_expiring` | `SSL Certificate Expiring - {app_name}` | Sent 30, 14, 7, 3, 1 days before expiry |
| `ssl_renewed` | `SSL Certificate Renewed - {app_name}` | Confirmation of renewal |
| `scheduler_error` | `Scheduled Task Failed - {app_name}` | Includes task name and error |
| `test` | `Test Email - {app_name}` | Simple test message |

**Default Sender:**
- From Name: `{app_name}` (defaults to binary name if not set)
- From Address: `noreply@{fqdn}` (defaults to `noreply@localhost` if FQDN not set)
- Reply-To: `{admin_email}` (if set, otherwise omitted)

**Default Expiry Times:**
| Link Type | Default Expiry | Configurable? |
|-----------|----------------|---------------|
| Password reset | 24 hours | Yes |
| Email verification | 48 hours | Yes |
| Account recovery | 1 hour | Yes |
| Setup token | 24 hours | No (security) |

## Template Format

Templates use simple `{variable}` syntax:

```
Subject: Your {app_name} backup completed
---
Hello,

Your backup completed successfully.

Filename: {filename}
Size: {size}
Time: {timestamp}

--
{app_name}
{app_url}
```

**Format Rules:**
- First line: `Subject: ...`
- Separator: `---` (three dashes on own line)
- Body: Plain text with variables
- Variables: `{variable_name}` syntax

## Global Variables (Available in All Templates)

| Variable | Description |
|----------|-------------|
| `{app_name}` | Application name/title |
| `{app_url}` | Application URL (full FQDN, e.g., `https://api.example.com`) |
| `{fqdn}` | Server FQDN only (e.g., `api.example.com`) |
| `{onion_url}` | Tor .onion URL (if enabled) |
| `{admin_email}` | Admin email address |
| `{recipient_email}` | Email address this message is being sent to |
| `{recipient_username}` | Username of the account (if applicable) |
| `{timestamp}` | Current date/time |
| `{year}` | Current year |

## Account Email Requirements (NON-NEGOTIABLE)

**ALL account-related emails MUST include:**

| Requirement | Description |
|-------------|-------------|
| **Why sent** | Clear explanation of why this email was sent |
| **Who it's for** | The recipient email address (visible in body) |
| **App identity** | App name AND full FQDN |
| **Visible link** | Plaintext URL (not just a button) - users can copy/paste |
| **Disclaimer** | "If you did not request this, ignore this message" (where applicable) |
| **No action if unsolicited** | Never include links that delete/modify without prior auth |

**Account-related emails include:**
- Welcome (user registration)
- Password reset
- Email verification
- Login alerts
- Security alerts
- 2FA changes
- Account recovery

### Example: User Welcome Email (Required Format)

```
Subject: Welcome to {app_name}
---
WELCOME TO {APP_NAME}

This email was sent to: {recipient_email}
From: {app_name} ({fqdn})

Hello {recipient_username},

Welcome to {app_name}! Your account has been created successfully.

To get started, log in at:

{login_url}

You can manage your profile and settings at:

{profile_url}

────────────────────────────────────────────────────────────────────────
GETTING STARTED

- Complete your profile
- Enable two-factor authentication for added security
- Explore the features available to you

If you have any questions, contact us at {admin_email}.
────────────────────────────────────────────────────────────────────────

--
{app_name}
{app_url}
```

### Example: Admin Welcome Email (Required Format)

```
Subject: Welcome to {app_name} - Admin Setup Complete
---
ADMIN SETUP COMPLETE

This email was sent to: {recipient_email}
From: {app_name} ({fqdn})

Congratulations! Your {app_name} instance is now configured.

Admin Panel: {admin_url}
Username: {admin_username}

────────────────────────────────────────────────────────────────────────
IMPORTANT NEXT STEPS

1. Log in to the admin panel and review your settings
2. Configure SMTP for email notifications
3. Enable SSL/TLS for secure connections
4. Set up regular backups
5. Enable two-factor authentication

Keep your admin credentials secure. If you lose access, use:
  airports --maintenance setup
────────────────────────────────────────────────────────────────────────

--
{app_name}
{app_url}
```

### Example: Password Reset Email (Required Format)

```
Subject: Password Reset Request - {app_name}
---
PASSWORD RESET REQUEST

This email was sent to: {recipient_email}
From: {app_name} ({fqdn})
Requested at: {timestamp}
Request IP: {ip}

Someone requested a password reset for the account associated with this
email address on {app_name} ({fqdn}).

To reset your password, visit the following link:

{reset_link}

This link expires in {expires}.

────────────────────────────────────────────────────────────────────────
⚠️  DID NOT REQUEST THIS?

If you did not request a password reset, you can safely ignore this email.
Your password will not be changed unless you click the link above.

No action is required on your part.
────────────────────────────────────────────────────────────────────────

--
{app_name}
{app_url}
```

### Example: Email Verification (Required Format)

```
Subject: Verify Your Email - {app_name}
---
EMAIL VERIFICATION

This email was sent to: {recipient_email}
From: {app_name} ({fqdn})
Sent at: {timestamp}

You (or someone) requested to add this email address to an account on
{app_name} ({fqdn}).

To verify this email address, visit the following link:

{verify_link}

This link expires in {expires}.

────────────────────────────────────────────────────────────────────────
⚠️  DID NOT REQUEST THIS?

If you did not request to add this email to an account, you can safely
ignore this email. No account will be created or linked.

No action is required on your part.
────────────────────────────────────────────────────────────────────────

--
{app_name}
{app_url}
```

### Example: Login Alert (Required Format)

```
Subject: New Login Detected - {app_name}
---
NEW LOGIN DETECTED

This alert was sent to: {recipient_email}
Account: {recipient_username}
From: {app_name} ({fqdn})

A new login was detected on your account:

  Time:     {time}
  IP:       {ip}
  Location: {location}
  Device:   {device}

If this was you, no action is required.

────────────────────────────────────────────────────────────────────────
⚠️  NOT YOU?

If you did not log in, your account may be compromised. Take action:

1. Change your password immediately:
   {app_url}/auth/forgot

2. Review your active sessions:
   {app_url}/settings/sessions

3. Enable 2FA if not already enabled:
   {app_url}/settings/security
────────────────────────────────────────────────────────────────────────

--
{app_name}
{app_url}
```

### Example: Security Alert (Required Format)

```
Subject: Security Alert - {app_name}
---
SECURITY ALERT

This alert was sent to: {recipient_email}
Account: {recipient_username}
From: {app_name} ({fqdn})
Time: {timestamp}

{event}

Details:
  Source IP: {ip}
  {details}

────────────────────────────────────────────────────────────────────────
⚠️  RECOMMENDED ACTIONS

If this activity was not you:

1. Change your password immediately:
   {app_url}/auth/forgot

2. Review account activity:
   {app_url}/settings/security

3. Contact support if needed:
   {admin_email}
────────────────────────────────────────────────────────────────────────

--
{app_name}
{app_url}
```

### Example: 2FA Disabled Alert (Required Format)

```
Subject: Two-Factor Authentication Disabled - {app_name}
---
2FA DISABLED

This alert was sent to: {recipient_email}
Account: {recipient_username}
From: {app_name} ({fqdn})
Time: {timestamp}

Two-factor authentication has been disabled on your account.

Method used: {method}
  (password, recovery key, or admin action)

────────────────────────────────────────────────────────────────────────
⚠️  DID NOT DO THIS?

If you did not disable 2FA, your account may be compromised:

1. Change your password immediately:
   {app_url}/auth/forgot

2. Re-enable 2FA:
   {app_url}/settings/security

3. Contact support:
   {admin_email}
────────────────────────────────────────────────────────────────────────

--
{app_name}
{app_url}
```

### Example: Password Changed Alert (Required Format)

```
Subject: Your Password Was Changed - {app_name}
---
PASSWORD CHANGED

This alert was sent to: {recipient_email}
Account: {recipient_username}
From: {app_name} ({fqdn})
Time: {timestamp}

The password for your account was successfully changed.

Method: {method}
IP Address: {ip}

If you made this change, no action is required.

────────────────────────────────────────────────────────────────────────
⚠️  DID NOT CHANGE YOUR PASSWORD?

If you did not change your password, your account may be compromised:

1. Reset your password immediately:
   {app_url}/auth/forgot

2. Review your account security:
   {app_url}/settings/security

3. Contact support:
   {admin_email}
────────────────────────────────────────────────────────────────────────

--
{app_name}
{app_url}
```

## Template-Specific Variables

**Note:** Account-related templates (marked ✓ above) also have access to `{recipient_email}`, `{recipient_username}`, and `{fqdn}` from global variables.

### welcome

**Two variants:** Admin welcome (first setup) and User welcome (registration).

**Admin Welcome Variables:**
| Variable | Description |
|----------|-------------|
| `{admin_url}` | Admin panel URL |
| `{admin_username}` | Initial admin username |

**User Welcome Variables:**
| Variable | Description |
|----------|-------------|
| `{recipient_username}` | New user's username |
| `{recipient_email}` | New user's email address |
| `{login_url}` | Login page URL |
| `{profile_url}` | User profile URL |

**When Sent:**
| Scenario | Template Used | Recipient |
|----------|---------------|-----------|
| First admin setup | Admin welcome | Server admin email |
| User registration (if enabled) | User welcome | New user's email |
| Admin creates user | User welcome | New user's email |
| User invited via invite code | User welcome | Invited user's email |

### password_reset
| Variable | Description |
|----------|-------------|
| `{reset_link}` | Password reset URL (full URL, visible in email) |
| `{expires}` | Link expiration time (e.g., "24 hours") |
| `{ip}` | Requesting IP address |

### email_verify
| Variable | Description |
|----------|-------------|
| `{verify_link}` | Email verification URL (full URL, visible in email) |
| `{expires}` | Link expiration time |

### login_alert
| Variable | Description |
|----------|-------------|
| `{ip}` | Login IP address |
| `{location}` | GeoIP location (if available) |
| `{device}` | User agent / device info |
| `{time}` | Login time |

### security_alert
| Variable | Description |
|----------|-------------|
| `{event}` | Security event type |
| `{ip}` | Source IP address |
| `{details}` | Event details |

### 2fa_enabled
| Variable | Description |
|----------|-------------|
| `{method}` | 2FA method enabled (TOTP, WebAuthn, etc.) |
| `{ip}` | IP address where 2FA was enabled |

### 2fa_disabled
| Variable | Description |
|----------|-------------|
| `{method}` | How 2FA was disabled (password, recovery key, admin) |
| `{ip}` | IP address where 2FA was disabled |

### password_changed
| Variable | Description |
|----------|-------------|
| `{ip}` | IP address where password was changed |
| `{method}` | How changed (direct, reset link, admin reset) |

### backup_complete / backup_failed
| Variable | Description |
|----------|-------------|
| `{filename}` | Backup filename |
| `{size}` | Backup file size |
| `{error}` | Error message (failed only) |

### ssl_expiring / ssl_renewed
| Variable | Description |
|----------|-------------|
| `{domain}` | Domain name |
| `{expires_in}` | Days until expiration |
| `{expiry_date}` | Expiration date |
| `{valid_until}` | New validity date (renewed only) |

### scheduler_error
| Variable | Description |
|----------|-------------|
| `{task_name}` | Failed task name |
| `{error}` | Error message |
| `{next_run}` | Next scheduled run |

## Admin Panel (/admin/email/templates)

| Element | Type | Description |
|---------|------|-------------|
| Template list | Table | All templates with status (default/custom) |
| Edit button | Button | Open template editor |
| Subject field | Text input | Editable subject line |
| Body editor | Textarea | Template body with syntax highlighting |
| Variable reference | Sidebar | Available variables for selected template |
| Preview button | Button | Render template with sample data |
| Send Test | Button | Send test email to specific address |
| Save button | Button | Save custom template |
| Reset to default | Button | Delete custom, restore embedded (confirmation required) |

**Editor Features:**
- Syntax highlighting for `{variables}`
- Variable autocomplete
- Live preview with sample data
- Validation (warn if unknown variables used)

### Template Preview

**Live preview renders the template with sample data as you edit.**

```
Email Template Editor (/admin/email/templates/password_reset)
┌─────────────────────────────────────────────────────────────┐
│  Template: password_reset                    [Default] [Custom]│
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Subject:                                                   │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Password Reset Request - {app_name}                 │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  Body:                              │  Available Variables: │
│  ┌────────────────────────────────┐ │  {app_name}           │
│  │ PASSWORD RESET REQUEST         │ │  {app_url}            │
│  │                                │ │  {fqdn}               │
│  │ This email was sent to:        │ │  {recipient_email}    │
│  │ {recipient_email}              │ │  {recipient_username} │
│  │ From: {app_name} ({fqdn})      │ │  {reset_link}         │
│  │                                │ │  {expires}            │
│  │ To reset your password:        │ │  {ip}                 │
│  │ {reset_link}                   │ │  {timestamp}          │
│  │                                │ │                       │
│  └────────────────────────────────┘ │  Click to insert ↑    │
│                                                             │
│  ─────────────────────────────────────────────────────────  │
│                                                             │
│  📧 Preview (with sample data):                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Subject: Password Reset Request - My App            │    │
│  │ ─────────────────────────────────────────────────── │    │
│  │ PASSWORD RESET REQUEST                              │    │
│  │                                                     │    │
│  │ This email was sent to: user@example.com            │    │
│  │ From: My App (app.example.com)                      │    │
│  │                                                     │    │
│  │ To reset your password:                             │    │
│  │ https://app.example.com/auth/reset/abc123...        │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  [Send Test Email]  [Reset to Default]  [Save]              │
└─────────────────────────────────────────────────────────────┘
```

**Sample Data for Preview:**
| Variable | Sample Value |
|----------|--------------|
| `{app_name}` | Current app name from config |
| `{app_url}` | Current app URL |
| `{fqdn}` | Current FQDN |
| `{recipient_email}` | `user@example.com` |
| `{recipient_username}` | `sampleuser` |
| `{reset_link}` | `https://{fqdn}/auth/reset/sample123...` |
| `{verify_link}` | `https://{fqdn}/auth/verify/sample123...` |
| `{expires}` | `24 hours` |
| `{ip}` | `192.168.1.100` |
| `{timestamp}` | Current timestamp |
| `{admin_email}` | Admin email from config |

### Send Test Email

**Send a test email to verify the template renders correctly.**

```
Send Test Email Dialog
┌─────────────────────────────────────────────────────────────┐
│  Send Test Email                                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Template: password_reset                                   │
│                                                             │
│  Send to: [admin@example.com              ]                 │
│           (defaults to your admin email)                    │
│                                                             │
│  ⚠️ This will send a real email using current SMTP settings. │
│  The email will contain sample data, not real user data.    │
│                                                             │
│  [Cancel]  [Send Test]                                      │
└─────────────────────────────────────────────────────────────┘
```

**Test Email Rules:**
- Requires valid SMTP configuration
- Defaults to current admin's email address
- Can specify any email address
- Uses sample data (not real user data)
- Subject prefixed with `[TEST]` to identify test emails
- Test sends logged to audit log

**After Sending:**
```
┌─────────────────────────────────────────────────────────────┐
│  ✅ Test Email Sent                                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Sent to: admin@example.com                                 │
│  Template: password_reset                                   │
│  Subject: [TEST] Password Reset Request - My App            │
│                                                             │
│  Check your inbox to verify the email looks correct.        │
│                                                             │
│  [Close]                                                    │
└─────────────────────────────────────────────────────────────┘
```

### Template Validation

**Templates are validated before saving:**

| Check | Error Message |
|-------|---------------|
| Unknown variable | `Unknown variable: {foo}. Did you mean {fqdn}?` |
| Missing required variable | `Account emails must include {recipient_email}` |
| Empty subject | `Subject cannot be empty` |
| Empty body | `Body cannot be empty` |
| Invalid syntax | `Invalid template syntax at line 5` |

**Warnings (non-blocking):**
- Using deprecated variables
- Very long subject line (>78 chars)
- Missing recommended sections (e.g., disclaimer for account emails)

## Notification Systems (NON-NEGOTIABLE)

**Two notification systems available. WebUI is always available. Email requires SMTP.**

| System | Availability | Use When |
|--------|--------------|----------|
| **WebUI (Toast/Banner)** | Always available | User is actively using the app |
| **Email** | Requires valid SMTP | User is away, needs permanent record, critical alerts |

## WebUI Notification System

**The WebUI has a built-in notification system for both server admins and users. This is ALWAYS available regardless of SMTP configuration.**

### How It Works

| Component | Description |
|-----------|-------------|
| **Toast** | Pop-up notifications in corner of screen |
| **Banner** | Persistent bar at top of page |
| **Notification Center** | Bell icon with history of notifications |
| **Badge Count** | Unread notification count on bell icon |

### Notification Center

**Both server admins and users have a notification center accessible via bell icon in the header.**

```
┌─────────────────────────────────────────────────────────────┐
│  Header                                    🔔 (3)  [User]   │
└─────────────────────────────────────────────────────────────┘
                                               │
                                               ▼
                              ┌────────────────────────────────┐
                              │  Notifications                 │
                              ├────────────────────────────────┤
                              │  🔴 SSL certificate expiring   │
                              │     in 3 days                  │
                              │     2 hours ago                │
                              ├────────────────────────────────┤
                              │  ✅ Backup completed           │
                              │     backup_2025-01-15.tar.gz   │
                              │     5 hours ago                │
                              ├────────────────────────────────┤
                              │  ⚠️ Login from new location    │
                              │     192.168.1.100 (New York)   │
                              │     Yesterday                  │
                              ├────────────────────────────────┤
                              │  [Mark all read]  [Clear all]  │
                              └────────────────────────────────┘
```

### WebUI Notification Types

| Type | Icon | Use For | Auto-dismiss |
|------|------|---------|--------------|
| `success` | ✅ | Completed actions, confirmations | 5 seconds |
| `info` | ℹ️ | Informational, status updates | 5 seconds |
| `warning` | ⚠️ | Non-critical issues, expiring items | 10 seconds |
| `error` | ❌ | Failures, critical issues | Manual dismiss |
| `security` | 🔒 | Security-related alerts | Manual dismiss |

### Toast vs Banner vs Notification Center

| Element | Use For | Behavior |
|---------|---------|----------|
| **Toast** | Immediate feedback for user actions | Auto-dismiss, stacks in corner |
| **Banner** | Persistent alerts requiring attention | Stays until dismissed or resolved |
| **Notification Center** | History of all notifications | Persists across sessions, stored in DB |

**When to Use Each:**

| Scenario | Toast | Banner | Center |
|----------|:-----:|:------:|:------:|
| Settings saved | ✓ | | |
| Form validation error | ✓ | | |
| Backup complete | ✓ | | ✓ |
| SSL expiring soon | | ✓ | ✓ |
| Update available | | ✓ | ✓ |
| Login from new IP | ✓ | | ✓ |
| Password changed | ✓ | | ✓ |
| SMTP not configured | | ✓ | |
| Scheduler task failed | ✓ | | ✓ |

## Server Admin Notifications

**Notifications shown to server admins in `/admin/*` routes.**

| Event | Toast | Banner | Center | Description |
|-------|:-----:|:------:|:------:|-------------|
| Settings saved | ✓ | | | Confirmation of config save |
| Config validation error | ✓ | | | Invalid config value |
| Backup started | ✓ | | | Backup in progress |
| Backup complete | ✓ | | ✓ | Backup finished |
| Backup failed | ✓ | | ✓ | Backup error |
| SSL expiring (7+ days) | | | ✓ | Warning in center only |
| SSL expiring (<3 days) | | ✓ | ✓ | Urgent banner |
| SSL renewed | ✓ | | ✓ | Certificate renewed |
| SSL renewal failed | ✓ | ✓ | ✓ | Critical - needs attention |
| Update available | | ✓ | ✓ | New version available |
| Scheduler task failed | ✓ | | ✓ | Task error |
| New admin login | | | ✓ | Another admin logged in |
| SMTP not configured | | ✓ | | Persistent warning |
| Database connection issue | | ✓ | ✓ | Critical warning |
| Disk space low | | ✓ | ✓ | System warning |
| GeoIP database outdated | | | ✓ | Update needed |
| Tor address ready | ✓ | | | Onion address generated |

## User Notifications (Multi-User Mode)

**Notifications shown to regular users in `/user/*` routes.**

| Event | Toast | Banner | Center | Description |
|-------|:-----:|:------:|:------:|-------------|
| Profile updated | ✓ | | | Settings saved |
| Password changed | ✓ | | ✓ | Security confirmation |
| Email verified | ✓ | | ✓ | Verification complete |
| 2FA enabled | ✓ | | ✓ | Security confirmation |
| 2FA disabled | ✓ | | ✓ | Security warning |
| Login from new IP | | | ✓ | Security notice |
| Login from new device | | | ✓ | Security notice |
| Session expired | ✓ | | | Re-login required |
| API token created | ✓ | | ✓ | Token generated |
| API token revoked | ✓ | | ✓ | Token deleted |
| Account suspended | | ✓ | | Admin action notice |
| Password reset required | | ✓ | | Admin-initiated reset |
| Recovery keys running low | | | ✓ | Only 1-2 keys left |

## Notification vs Email Decision Matrix

**WebUI notifications are ALWAYS used when the user is active. Email is ONLY used when:**
1. SMTP is configured AND working
2. The event warrants a permanent record OR
3. The user may be away and needs to be alerted

| Event | WebUI | Email | Reason |
|-------|:-----:|:-----:|--------|
| Settings saved | ✓ | ✗ | Immediate feedback only |
| Backup complete | ✓ | Optional | Quick confirmation |
| Backup failed | ✓ | ✓ | Critical - needs attention |
| SSL expiring (7+ days) | ✓ | ✗ | Warning, not urgent |
| SSL expiring (<3 days) | ✓ | ✓ | Urgent - needs action |
| SSL renewed | ✓ | ✗ | Informational |
| Login from new IP | ✓ | ✓ | Security - permanent record |
| Security alert | ✓ | ✓ | Critical - needs record |
| Scheduler task failed | ✓ | ✓ | Needs attention when away |
| Scheduler task success | ✗ | ✗ | No notification needed |
| Password changed | ✓ | ✓ | Security - confirmation |
| Token regenerated | ✓ | ✓ | Security - confirmation |
| 2FA enabled/disabled | ✓ | ✓ | Security - confirmation |
| Tor address regenerated | ✓ | ✗ | User initiated |
| Update available | ✓ | Optional | Informational |
| Update installed | ✓ | ✓ | Important change record |
| Welcome (new user) | ✓ | ✓ | Onboarding |
| Email verification | ✗ | ✓ | Requires email link |
| Password reset | ✗ | ✓ | Requires email link |

### Decision Logic

```
1. Is user actively using the app?
   → Always show WebUI notification (toast/center)

2. Is SMTP configured?
   → No: WebUI only, no email attempt
   → Yes: Continue to step 3

3. Is it critical (failure, security, urgent)?
   → Send email

4. Does user need a record when away?
   → Send email

5. Is it just confirmation of user action?
   → WebUI only (no email)

6. Is it routine success?
   → No notification needed
```

## Notification Storage

**WebUI notifications are stored in the database for persistence.**

| Storage | Server Admin | Regular User |
|---------|--------------|--------------|
| Table | `admin_notifications` | `user_notifications` |
| Retention | 30 days (configurable) | 30 days (configurable) |
| Max stored | 100 per admin | 100 per user |
| Sync | Real-time via WebSocket | Real-time via WebSocket |

**Notification Record:**
```json
{
  "id": "notif_01HQXYZ",
  "type": "warning",
  "title": "SSL Certificate Expiring",
  "message": "Certificate expires in 3 days",
  "link": "/admin/server/ssl",
  "read": false,
  "created_at": "2025-01-15T10:30:00Z"
}
```

## Sane Defaults

| Setting | Default | Description |
|---------|---------|-------------|
| Toast position | `top-right` | Corner for toast notifications |
| Toast duration | `5` seconds | Auto-dismiss time (0 = manual) |
| Error dismiss | `manual` | Errors require manual dismiss |
| Notification retention | `30` days | How long to keep in center |
| Max notifications | `100` | Per user/admin limit |
| Real-time updates | `enabled` | WebSocket for instant updates |

## Notification Preferences

**Both server admins and users can configure their notification preferences.**

### Admin Notification Preferences (`/admin/profile/notifications`)

| Category | Events | Default | Can Disable? |
|----------|--------|---------|--------------|
| **Security** | Login alerts, 2FA changes, password changes | All ON | No (required) |
| **System** | SSL expiring, updates available, disk space | All ON | Yes |
| **Backup** | Backup complete, backup failed | Failed ON, Complete OFF | Yes |
| **Scheduler** | Task failed, task manual run | Failed ON | Yes |
| **Other Admins** | Admin login/logout | ON | Yes |

**Security notifications cannot be disabled** - these are critical for account security.

```
Admin Notification Preferences (/admin/profile/notifications)
┌─────────────────────────────────────────────────────────────┐
│  Notification Preferences                                   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  🔒 Security (cannot be disabled)                           │
│     ☑ Login from new IP/device                              │
│     ☑ Password changed                                      │
│     ☑ 2FA enabled/disabled                                  │
│     ☑ API token regenerated                                 │
│                                                             │
│  ⚙️ System                                          WebUI Email│
│     SSL certificate expiring                        [✓]  [✓] │
│     SSL certificate renewed                         [✓]  [ ] │
│     Update available                                [✓]  [ ] │
│     Disk space low                                  [✓]  [✓] │
│                                                             │
│  💾 Backup                                                   │
│     Backup completed                                [✓]  [ ] │
│     Backup failed                                   [✓]  [✓] │
│                                                             │
│  📅 Scheduler                                                │
│     Task failed                                     [✓]  [✓] │
│     Task manually triggered                         [✓]  [ ] │
│                                                             │
│  👥 Other Admins                                             │
│     Admin logged in                                 [✓]  [ ] │
│     Admin logged out                                [ ]  [ ] │
│                                                             │
│  [Save Preferences]                                         │
└─────────────────────────────────────────────────────────────┘
```

### User Notification Preferences (`/user/settings/notifications`)

| Category | Events | Default | Can Disable? |
|----------|--------|---------|--------------|
| **Security** | Login alerts, password changes, 2FA changes | All ON | No (required) |
| **Account** | Email verified, profile updated | All ON | Yes |
| **Sessions** | Session expired, new device | All ON | Partial |

```
User Notification Preferences (/user/settings/notifications)
┌─────────────────────────────────────────────────────────────┐
│  Notification Preferences                                   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  🔒 Security (cannot be disabled)                           │
│     ☑ Login from new IP/device                              │
│     ☑ Password changed                                      │
│     ☑ 2FA enabled/disabled                                  │
│     ☑ Recovery key used                                     │
│                                                             │
│  👤 Account                                         WebUI Email│
│     Email verified                                  [✓]  [✓] │
│     Profile updated                                 [✓]  [ ] │
│                                                             │
│  🔑 Sessions                                                 │
│     Session expired                                 [✓]  [ ] │
│     Logged in from new device                       [✓]  [✓] │
│                                                             │
│  [Save Preferences]                                         │
└─────────────────────────────────────────────────────────────┘
```

### Preference Storage

| User Type | Storage | Key |
|-----------|---------|-----|
| Server Admin | `admin_preferences` table | `admin_id` |
| Regular User | `user_preferences` table | `user_id` |

**Preference Schema:**
```json
{
  "notifications": {
    "webui": {
      "backup_complete": true,
      "backup_failed": true,
      "ssl_expiring": true,
      "admin_login": true
    },
    "email": {
      "backup_complete": false,
      "backup_failed": true,
      "ssl_expiring": true,
      "admin_login": false
    }
  }
}
```

## Configuration

```yaml
server:
  notifications:
    # WebUI notifications (always enabled)
    webui:
      position: top-right
      # top-right, top-left, bottom-right, bottom-left
      duration: 5
      # seconds (0 = manual dismiss)

    # Email notifications
    email:
      enabled: false

      # Auto-detect SMTP server on local network
      autodetect: true
      autodetect_hosts:
        - localhost
        - 172.17.0.1
      autodetect_ports:
        - 25
        - 465
        - 587

      # Manual SMTP settings (used if autodetect disabled or fails)
      smtp:
        host: ""
        port: 587
        username: ""
        password: ""
        from: "noreply@{fqdn}"
        # TLS mode: auto, required, none
        tls: auto

      # Per-event email settings (override defaults)
      events:
        startup: false
        shutdown: false
        backup_complete: false
        backup_failed: true
        ssl_expiring: true
        ssl_renewed: false
        login_alert: true
        security_alert: true
        scheduler_error: true
        password_changed: true
        token_regenerated: true
        update_available: false
        update_installed: true
```

---

# PART 17: CLI INTERFACE (NON-NEGOTIABLE)

**THESE COMMANDS CANNOT BE CHANGED. This is the complete command set.**

## Main Commands

```bash
--help                       # Show help (can be run by anyone)
--version                    # Show version (can be run by anyone)
--mode {production|development}  # Set application mode
--data {datadir}             # Set data dir
--config {etcdir}            # Set the config dir
--address {listen}           # Set listen address
--port {port}                # Set the port
--status                     # Show status and health
--service {start,restart,stop,reload,--install,--uninstall,--disable,--help}
--maintenance {backup,restore,update,mode,setup} [optional-file-or-setting]
--update [check|yes|branch {stable|beta|daily}]  # Check/perform updates
```

### Commands Anyone Can Run (No Privileges)

- `--help`
- `--version`
- `--status`
- `--update check`

## Display Rules (NON-NEGOTIABLE)

| Rule | Description |
|------|-------------|
| Never show | `0.0.0.0`, `127.0.0.1`, `localhost` |
| Always show | Valid FQDN, host, or IP |
| Show only | One address, the most relevant |

## URL & FQDN Detection (NON-NEGOTIABLE)

**CRITICAL: Never hardcode host, IP, or port in project code. Always detect dynamically.**

### URL Display Rules

| Rule | Description |
|------|-------------|
| **NEVER hardcode** | `localhost`, `127.0.0.1`, `0.0.0.0`, `[::1]`, any static host/IP |
| **NEVER display** | `GET /api/`, `POST /api/` without full URL |
| **ALWAYS use** | `{proto}://{host}:{port}/path` format |
| **ALWAYS detect** | `{proto}`, `{host}`, `{port}` from request context |
| **ALWAYS strip** | `:80` for HTTP, `:443` for HTTPS |
| **Default proto** | `http` if not detected |

### URL Format Examples

| WRONG | RIGHT |
|-------|-------|
| `GET /api/v1/resource/random` | `https://api.example.com/api/v1/resource/random` |
| `POST /api/v1/admin/config` | `https://api.example.com/api/v1/admin/config` |
| `http://localhost:8080/api` | `http://192.168.1.100:64580/api` |
| `http://0.0.0.0:80/healthz` | `https://myserver.example.com/healthz` |

### FQDN Detection Order (First Valid Wins)

| Priority | Source | Description |
|----------|--------|-------------|
| 1 | **Reverse Proxy Headers** | `X-Forwarded-Host`, `X-Real-Host`, etc. |
| 2 | **DOMAIN env var** | Must be valid FQDN (e.g., `api.example.com`) |
| 3 | **HOSTNAME env var** | Must be valid FQDN (e.g., `host.example.com`) |
| 4 | **Global IPv6** | If available and routable |
| 5 | **Global IPv4** | If available and routable |

### FQDN/Host Validation Rules

**Uses `golang.org/x/net/publicsuffix` for proper TLD validation.**

**go.mod:**
```
require golang.org/x/net v0.33.0
```

This properly handles complex suffixes like `.co.uk`, `.com.au`, `.org.uk`, etc.

**Production Mode (Strict):**

| Valid | Invalid |
|-------|---------|
| `api.example.com` | `localhost` |
| `my.server.domain.co.uk` | `dev.local` |
| `app.company.com.au` | `app.test` |
| `server.company.io` | `192.168.1.1` (IP address) |
| | `myhost` (single-label) |

**Development Mode (Relaxed):**

| Valid | Invalid |
|-------|---------|
| `api.example.com` | `192.168.1.1` (IP address) |
| `localhost` | `myhost` (single-label, not localhost) |
| `dev.local` | `devbox` (single-label) |
| `app.test` | |
| `staging.internal` | |

**Validation Requirements:**

| Mode | Rules |
|------|-------|
| **Production** | Must have valid public suffix (ICANN TLD), no IPs, no dev TLDs |
| **Development** | Must have dot OR be localhost, no IPs, dev TLDs allowed |

**Internal/Dev-Only TLDs (blocked in production):**
- `localhost` (literal)
- `.localhost`, `.test`, `.example`, `.invalid` (RFC 6761)
- `.local`, `.lan`, `.internal`, `.home`, `.localdomain`
- `.home.arpa`, `.intranet`, `.corp`, `.private`
- `airports` - dynamic (e.g., `app.jokes`, `dev.quotes`, `my.api`)

**Overlay Network TLDs (App-Managed, not set in DOMAIN):**
- `.onion` - Tor hidden services (RFC 7686) - app generates/manages
- `.i2p` - I2P eepsites - app generates/manages
- `.exit` - Tor exit notation - app generates/manages

These are NOT set via `DOMAIN` env var. App handles registration and displays them in console.

**Go Implementation:**
```go
import (
    "net"
    "strings"

    "golang.org/x/net/publicsuffix"
)

var devOnlyTLDs = map[string]bool{
    "localhost": true, "test": true, "example": true, "invalid": true,
    "local": true, "lan": true, "internal": true, "home": true,
    "localdomain": true, "home.arpa": true, "intranet": true,
    "corp": true, "private": true,
}

func IsValidHost(host string, devMode bool, projectName string) bool {
    lower := strings.ToLower(strings.TrimSpace(host))

    // Reject empty
    if lower == "" {
        return false
    }

    // Reject IP addresses always
    if net.ParseIP(lower) != nil {
        return false
    }

    // Handle localhost
    if lower == "localhost" {
        return devMode
    }

    // Must contain at least one dot
    if !strings.Contains(lower, ".") {
        return false
    }

    // Overlay network TLDs - valid but app-managed (not set via DOMAIN)
    // These are checked here for internal validation, not for DOMAIN env var
    if strings.HasSuffix(lower, ".onion") ||
       strings.HasSuffix(lower, ".i2p") ||
       strings.HasSuffix(lower, ".exit") {
        return true
    }

    // Check dynamic project-specific TLD (e.g., app.jokes, dev.quotes, quotes, jokes, airports)
    if projectName != "" && strings.HasSuffix(lower, "."+strings.ToLower(projectName)) {
        return devMode // Project TLDs only valid in dev mode
    }

    // Get the public suffix (TLD or eTLD like co.uk)
    suffix, icann := publicsuffix.PublicSuffix(lower)

    // Check if it's a dev-only TLD
    if devOnlyTLDs[suffix] {
        return devMode // Dev TLDs only valid in dev mode
    }

    // In production, require valid ICANN TLD
    if !devMode && !icann {
        return false
    }

    // Verify we have at least eTLD+1 (not just the suffix itself)
    etldPlusOne, err := publicsuffix.EffectiveTLDPlusOne(lower)
    if err != nil {
        return false
    }

    // Host must be at least eTLD+1 (e.g., "domain.co.uk" not just "co.uk")
    return len(etldPlusOne) > 0
}
```

**Example Results (projectName = "jokes"):**

| Host | Production | Development | Reason |
|------|------------|-------------|--------|
| `my.server.domain.co.uk` | ✓ | ✓ | Valid eTLD+1: `domain.co.uk` |
| `api.example.com` | ✓ | ✓ | Valid eTLD+1: `example.com` |
| `app.company.com.au` | ✓ | ✓ | Valid eTLD+1: `company.com.au` |
| `dev.local` | ✗ | ✓ | Dev TLD `.local` |
| `app.test` | ✗ | ✓ | Dev TLD `.test` |
| `app.jokes` | ✗ | ✓ | Dynamic dev TLD `jokes` |
| `my.app.jokes` | ✗ | ✓ | Dynamic dev TLD `jokes` |
| `localhost` | ✗ | ✓ | Dev only |
| `co.uk` | ✗ | ✗ | No eTLD+1 (suffix only) |
| `192.168.1.1` | ✗ | ✗ | IP address |
| `myhost` | ✗ | ✗ | No dot (single-label) |

**Note:** DOMAIN and HOSTNAME environment variables MUST pass host validation for the current mode. Invalid values are skipped silently and detection continues to next source.

### SSL/Let's Encrypt FQDN Requirements

When requesting SSL certificates (Let's Encrypt), the FQDN must be **publicly resolvable**. This uses the same validation as production mode - no internal/dev-only TLDs allowed.

**Go Implementation for SSL validation:**
```go
func IsValidSSLHost(host string) bool {
    lower := strings.ToLower(host)

    // .onion addresses cannot use Let's Encrypt (not publicly resolvable)
    // Tor provides end-to-end encryption, so SSL is optional for .onion
    if strings.HasSuffix(lower, ".onion") {
        return false
    }

    // SSL always requires production-valid host (devMode=false)
    return IsValidHost(host, false)
}
```

**Behavior:**
- SSL with Let's Encrypt: Must pass production-mode validation (no dev TLDs)
- .onion addresses: Cannot use Let's Encrypt (Tor provides encryption)
- If Let's Encrypt requested with invalid host: Log warning, skip cert request, continue without SSL
- Self-signed certs: Can use any valid host for current mode

### Reverse Proxy Header Support (All Headers Supported)

**Protocol Detection (`{proto}`):**
- `X-Forwarded-Proto` - Standard: "https" or "http"
- `X-Forwarded-Ssl` - "on" = https, "off" = http
- `X-Url-Scheme` - Alternative: "https" or "http"
- `Front-End-Https` - Microsoft: "on" = https

**Host Detection (`{host}`):**
- `X-Forwarded-Host` - Standard: "example.com" or "example.com:8080"
- `X-Real-Host` - nginx: "example.com"
- `X-Original-Host` - Alternative

**Port Detection (`{port}`):**
- `X-Forwarded-Port` - Standard: "443" or "8080"
- `X-Real-Port` - nginx alternative

**Client IP Detection (for logging, rate limiting, GeoIP):**
- `X-Forwarded-For` - Standard: may contain chain "client, proxy1, proxy2"
- `X-Real-IP` - nginx: single IP
- `CF-Connecting-IP` - Cloudflare
- `True-Client-IP` - Akamai/Cloudflare Enterprise
- `X-Client-IP` - Alternative

**Request ID (for tracing):**
- `X-Request-ID` - Standard
- `X-Correlation-ID` - Alternative
- `X-Trace-ID` - Distributed tracing

### Request ID Handling (NON-NEGOTIABLE)

**Every request MUST have a Request ID for tracing and debugging.**

| Scenario | Behavior |
|----------|----------|
| Client sends `X-Request-ID` | Use client's ID (validate format) |
| No Request ID header | Generate new UUID |
| Invalid format | Generate new UUID, log warning |

**Request ID Rules:**

| Rule | Description |
|------|-------------|
| **Format** | UUID v4 (e.g., `550e8400-e29b-41d4-a716-446655440000`) |
| **Generation** | Use secure random UUID generator |
| **Propagation** | Include in all outgoing requests to downstream services |
| **Response** | Return `X-Request-ID` in response headers |
| **Logging** | Include `{request_id}` in all log entries for the request |

**Go Implementation:**
```go
func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check for existing request ID from client or upstream proxy
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = r.Header.Get("X-Correlation-ID")
        }
        if requestID == "" {
            requestID = r.Header.Get("X-Trace-ID")
        }

        // Generate new ID if none provided or invalid
        if requestID == "" || !isValidUUID(requestID) {
            requestID = uuid.New().String()
        }

        // Add to response headers
        w.Header().Set("X-Request-ID", requestID)

        // Add to request context for logging and downstream calls
        ctx := context.WithValue(r.Context(), "request_id", requestID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Auth Token Headers (All Headers Supported)

**Standard Authorization:**
- `Authorization` - Standard: `Bearer {token}`, `Basic {base64}`, `Digest {credentials}`

**API Key Headers:**
- `X-API-Key` - Common API key header
- `X-Api-Key` - Case variant (treat as same)
- `API-Key` - Alternative without X- prefix
- `ApiKey` - No hyphen variant

**Custom Token Headers:**
- `X-Auth-Token` - Custom auth token
- `X-Access-Token` - Access token header
- `X-Token` - Short form
- `Token` - Minimal form

**Session/CSRF Headers:**
- `X-CSRF-Token` - CSRF protection token
- `X-XSRF-Token` - Angular variant
- `X-Session-ID` - Session identifier

**Service-to-Service:**
- `X-Service-Token` - Internal service auth
- `X-Internal-Token` - Internal API calls

**Priority Order (first found wins):**
1. `Authorization` header (standard, preferred)
2. `X-API-Key` / `API-Key`
3. `X-Auth-Token` / `X-Access-Token`
4. `X-Token` / `Token`
5. Query parameter `?token=` (least preferred, avoid in production)

### Implementation Requirements

1. **Request Context Helper**: Create a helper function that extracts `{proto}`, `{host}`, `{port}` from each request
2. **URL Builder**: Create a helper that builds full URLs: `{proto}://{host}:{port}/path` (strip :80/:443)
3. **Never Import** hardcoded URLs into templates - always pass detected values
4. **API Response URLs**: All URLs in API responses must be absolute, using detected values
5. **Swagger/OpenAPI**: Server URL must be detected, not hardcoded

---

# PART 18: UPDATE COMMAND (NON-NEGOTIABLE)

## --update Command

```bash
--update [command]
```

**Alias:** `--maintenance update` is an alias for `--update yes`

## Commands

| Command | Description |
|---------|-------------|
| `yes` (default) | Check and perform in-place update with restart |
| `check` | Check for updates without installing (no privileges required) |
| `branch {stable\|beta\|daily}` | Set update branch |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Successful update or no update available |
| 1 | Error |

**HTTP 404 from GitHub API means no updates available (already current).**

### Update Branches

| Branch | Release Type | Tag Pattern | Example |
|--------|--------------|-------------|---------|
| `stable` (default) | Release | `v*`, `*.*.*` | `v1.0.0` |
| `beta` | Pre-release | `*-beta` | `202512051430-beta` |
| `daily` | Pre-release | `YYYYMMDDHHMMSS` | `20251205143022` |

### Examples

```bash
# Check for updates (no privileges required)
airports --update check

# Perform update (these are equivalent)
airports --update
airports --update yes
airports --maintenance update

# Switch channels
airports --update branch beta
airports --update branch daily
airports --update branch stable
```

---

# PART 19: DOCKER (NON-NEGOTIABLE)

## Docker Directory Structure (NON-NEGOTIABLE)

All Docker-related files MUST be in `./docker/`:

```
./docker/
├── Dockerfile              # Production Dockerfile
├── Dockerfile.dev          # Development Dockerfile (optional)
├── docker-compose.yml      # Production compose
├── docker-compose.dev.yml  # Development compose (optional)
└── rootfs/                 # Container filesystem overlay
    └── usr/
        └── local/
            └── bin/
                └── entrypoint.sh  # Container entrypoint (REQUIRED)
```

**Build Context:**
- Docker build context is project root (`.`)
- Dockerfile specified with `-f ./docker/Dockerfile`
- Binaries copied from `./binaries/` (project root)
- rootfs copied from `./docker/rootfs/`

**Rules:**
- NEVER place Dockerfile or docker-compose.yml in project root
- ALWAYS use `./docker/` directory for all Docker files
- ALWAYS use `entrypoint.sh` for container startup
- ALWAYS build binaries to `./binaries/` (not `./docker/binaries/`)
- rootfs structure mirrors container filesystem

## Dockerfile Requirements

| Requirement | Value |
|-------------|-------|
| Location | `./docker/Dockerfile` |
| Base | Alpine (`:latest` or version matching build) |
| Meta labels | All OCI labels (see below) |
| Required packages | curl, bash, tini, **tor** |
| Binary location | `/usr/local/bin/airports` |
| Entrypoint script | `/usr/local/bin/entrypoint.sh` |
| Init system | **tini** |
| Internal port | **80** |
| **ENV MODE** | **development** (allows localhost, .local, .test, etc.) |

### Container Paths (NON-NEGOTIABLE)

| Path | Purpose |
|------|---------|
| `/config` | Configuration directory (server.yml, templates, SSL certs) |
| `/data` | Data directory (databases, Tor keys, caches, GeoIP) |
| `/data/db` | SQLite databases (server.db, users.db) |
| `/data/logs` | Log files (access.log, error.log, audit.log, etc.) |
| `/data/tor` | Tor data (hidden service keys) |
| `/data/geoip` | GeoIP databases |
| `/data/backup` | Backup archives |
| `/usr/local/bin/airports` | Application binary |
| `/root/Dockerfile` | Build reference and backup |

### OCI Meta Labels (Required)

All Dockerfiles MUST include these labels:

| Label | Value |
|-------|-------|
| `maintainer` | `{maintainer_name} <{maintainer_email}>` |
| `org.opencontainers.image.vendor` | `apimgr` |
| `org.opencontainers.image.authors` | `apimgr` |
| `org.opencontainers.image.title` | `airports` |
| `org.opencontainers.image.base.name` | `airports` |
| `org.opencontainers.image.description` | `Containerized version of airports` |
| `org.opencontainers.image.licenses` | License (e.g., `MIT`) |
| `org.opencontainers.image.created` | `${BUILD_DATE}` (ARG) |
| `org.opencontainers.image.version` | `${VERSION}` (ARG) |
| `org.opencontainers.image.schema-version` | `${VERSION}` (ARG) |
| `org.opencontainers.image.revision` | `${VCS_REF}` (ARG) |
| `org.opencontainers.image.url` | `https://github.com/apimgr/airports` |
| `org.opencontainers.image.source` | `https://github.com/apimgr/airports` |
| `org.opencontainers.image.documentation` | `https://github.com/apimgr/airports` |
| `org.opencontainers.image.vcs-type` | `Git` |
| `com.github.containers.toolbox` | `false` |

### Dockerfile Rules (NON-NEGOTIABLE)

| Rule | Description |
|------|-------------|
| **NEVER modify ENTRYPOINT** | Always use entrypoint.sh for customization |
| **NEVER modify CMD** | Pass commands to entrypoint.sh instead |
| **STOPSIGNAL** | Use `SIGRTMIN+3` for proper shutdown |
| **ENTRYPOINT format** | `[ "tini", "-p", "SIGTERM", "--", "/usr/local/bin/entrypoint.sh" ]` |
| **HEALTHCHECK timing** | Start: 10m, Interval: 5m, Timeout: 15s |
| **Customization** | ALL customization via entrypoint.sh |

### Dockerfile Example

**Location:** `./docker/Dockerfile`

```dockerfile
# Runtime stage
FROM alpine:latest

# ARGs for build-time values (set by docker build --build-arg)
ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF
ARG TARGETARCH
ARG LICENSE=MIT

# Static Labels
LABEL maintainer="{maintainer_name} <{maintainer_email}>" \
      org.opencontainers.image.vendor="apimgr" \
      org.opencontainers.image.authors="apimgr" \
      org.opencontainers.image.title="airports" \
      org.opencontainers.image.base.name="airports" \
      org.opencontainers.image.description="Containerized version of airports" \
      org.opencontainers.image.url="https://github.com/apimgr/airports" \
      org.opencontainers.image.source="https://github.com/apimgr/airports" \
      org.opencontainers.image.documentation="https://github.com/apimgr/airports" \
      org.opencontainers.image.vcs-type="Git" \
      com.github.containers.toolbox="false"

# Dynamic Labels (from ARGs)
LABEL org.opencontainers.image.licenses="${LICENSE}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.schema-version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}"

# Install required packages including Tor
RUN apk add --no-cache \
    curl \
    bash \
    tini \
    tor

# Create directories with proper structure
RUN mkdir -p /config /data/db /data/logs /data/tor /data/geoip /data/backup

# Copy the arch-specific binary from ./binaries/ (project root)
COPY binaries/airports-linux-${TARGETARCH} /usr/local/bin/airports

# Copy entrypoint script from ./docker/rootfs/
COPY docker/rootfs/ /

# Copy Dockerfile to image (for reference and backup)
COPY docker/Dockerfile /root/Dockerfile

# Make all binaries/scripts in /usr/local/bin executable (755)
RUN chmod 755 /usr/local/bin/*

# Set environment
ENV MODE=development \
    TZ=America/New_York \
    ENABLE_TOR=false

# Expose internal port (always 80)
EXPOSE 80

# Stop signal for graceful shutdown
STOPSIGNAL SIGRTMIN+3

# Health check (long start period for services that need initialization)
HEALTHCHECK --start-period=10m --interval=5m --timeout=15s --retries=3 \
    CMD /usr/local/bin/airports --status || exit 1

# Use tini as init with signal propagation
# -p SIGTERM: propagate SIGTERM to child processes
ENTRYPOINT [ "tini", "-p", "SIGTERM", "--", "/usr/local/bin/entrypoint.sh" ]
```

**Notes:**
- `TARGETARCH` is automatically set by Docker buildx to `amd64` or `arm64`
- **NEVER modify ENTRYPOINT or CMD** - all customization goes in `entrypoint.sh`
- `STOPSIGNAL SIGRTMIN+3` allows entrypoint.sh to handle shutdown gracefully
- `tini -p SIGTERM` propagates SIGTERM to all child processes

### Entrypoint Script (REQUIRED)

**Location:** `./docker/rootfs/usr/local/bin/entrypoint.sh`

```bash
#!/usr/bin/env bash
set -e

# =============================================================================
# Container Entrypoint Script
# Handles service startup, signal handling, and graceful shutdown
# =============================================================================

APP_NAME="airports"
APP_BIN="/usr/local/bin/${APP_NAME}"
TOR_ENABLED="${ENABLE_TOR:-false}"
TOR_DATA_DIR="/data/tor"

# Array to track background PIDs
declare -a PIDS=()

# -----------------------------------------------------------------------------
# Logging
# -----------------------------------------------------------------------------
log() {
    echo "[entrypoint] $(date '+%Y-%m-%d %H:%M:%S') $*"
}

log_error() {
    echo "[entrypoint] $(date '+%Y-%m-%d %H:%M:%S') ERROR: $*" >&2
}

# -----------------------------------------------------------------------------
# Signal handling
# -----------------------------------------------------------------------------
cleanup() {
    log "Received shutdown signal, stopping services..."

    # Stop services in reverse order
    for ((i=${#PIDS[@]}-1; i>=0; i--)); do
        pid="${PIDS[i]}"
        if kill -0 "$pid" 2>/dev/null; then
            log "Stopping PID $pid..."
            kill -TERM "$pid" 2>/dev/null || true
        fi
    done

    # Wait for processes to exit (max 30 seconds)
    local timeout=30
    while [ $timeout -gt 0 ]; do
        local running=0
        for pid in "${PIDS[@]}"; do
            if kill -0 "$pid" 2>/dev/null; then
                running=1
                break
            fi
        done
        [ $running -eq 0 ] && break
        sleep 1
        ((timeout--))
    done

    # Force kill any remaining
    for pid in "${PIDS[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            log "Force killing PID $pid..."
            kill -9 "$pid" 2>/dev/null || true
        fi
    done

    log "Shutdown complete"
    exit 0
}

# Trap signals for graceful shutdown
# SIGRTMIN+3 (37) is the Docker STOPSIGNAL
# SIGTERM is propagated by tini -p SIGTERM
trap cleanup SIGTERM SIGINT SIGQUIT
trap cleanup SIGRTMIN+3 2>/dev/null || trap cleanup 37

# -----------------------------------------------------------------------------
# Directory setup
# -----------------------------------------------------------------------------
setup_directories() {
    log "Setting up directories..."
    mkdir -p /config /data/db /data/logs /data/tor /data/geoip /data/backup

    # Fix permissions for Tor (runs as tor user)
    if [ "$TOR_ENABLED" = "true" ]; then
        chown -R tor:tor "$TOR_DATA_DIR"
        chmod 700 "$TOR_DATA_DIR"
    fi
}

# -----------------------------------------------------------------------------
# Start Tor (if enabled)
# -----------------------------------------------------------------------------
start_tor() {
    if [ "$TOR_ENABLED" != "true" ]; then
        return 0
    fi

    log "Starting Tor hidden service..."

    # Create torrc if not exists
    if [ ! -f /config/torrc ]; then
        cat > /config/torrc <<EOF
DataDirectory ${TOR_DATA_DIR}
HiddenServiceDir ${TOR_DATA_DIR}/hidden_service
HiddenServicePort 80 127.0.0.1:80
Log notice file /data/logs/tor.log
EOF
    fi

    # Start Tor in background
    tor -f /config/torrc &
    PIDS+=($!)
    log "Tor started (PID: ${PIDS[-1]})"

    # Wait for .onion address
    local timeout=60
    while [ $timeout -gt 0 ]; do
        if [ -f "${TOR_DATA_DIR}/hidden_service/hostname" ]; then
            local onion_addr
            onion_addr=$(cat "${TOR_DATA_DIR}/hidden_service/hostname")
            log "Tor hidden service: ${onion_addr}"
            break
        fi
        sleep 1
        ((timeout--))
    done
}

# -----------------------------------------------------------------------------
# Start main application
# -----------------------------------------------------------------------------
start_app() {
    log "Starting ${APP_NAME}..."

    # Run the main application
    # Pass through any arguments from CMD
    "$APP_BIN" "$@" &
    PIDS+=($!)
    log "${APP_NAME} started (PID: ${PIDS[-1]})"
}

# -----------------------------------------------------------------------------
# Wait for services
# -----------------------------------------------------------------------------
wait_for_services() {
    log "All services started, waiting..."

    # Wait for any process to exit
    while true; do
        for pid in "${PIDS[@]}"; do
            if ! kill -0 "$pid" 2>/dev/null; then
                log_error "Process $pid exited unexpectedly"
                cleanup
            fi
        done
        sleep 5
    done
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------
main() {
    log "Container starting..."
    log "MODE: ${MODE:-development}"
    log "TZ: ${TZ:-UTC}"
    log "ENABLE_TOR: ${TOR_ENABLED}"

    setup_directories
    start_tor
    start_app "$@"
    wait_for_services
}

main "$@"
```

### Entrypoint Features

| Feature | Description |
|---------|-------------|
| **Signal handling** | Graceful shutdown on SIGTERM/SIGINT/SIGQUIT/SIGRTMIN+3 |
| **Docker STOPSIGNAL** | Handles SIGRTMIN+3 (signal 37) from Docker |
| **Multi-service** | Can start Tor + main app + additional services |
| **Ordered startup** | Services start in defined order |
| **Ordered shutdown** | Services stop in reverse order |
| **Process monitoring** | Detects unexpected process exits |
| **Timeout handling** | Force kill after 30s if graceful shutdown fails |
| **Directory setup** | Creates required directories on startup |
| **Logging** | Timestamped log messages |

### Why SIGRTMIN+3?

| Reason | Description |
|--------|-------------|
| **Systemd compatibility** | SIGRTMIN+3 is used by systemd for clean shutdown |
| **Container orchestration** | Works well with Docker, Podman, Kubernetes |
| **Graceful multi-process** | Allows entrypoint.sh to coordinate shutdown of all services |
| **Avoids race conditions** | Gives time for proper cleanup before forced termination |

### Environment Variables (Entrypoint)

| Variable | Default | Description |
|----------|---------|-------------|
| `MODE` | `development` | Application mode |
| `TZ` | `America/New_York` | Timezone |
| `ENABLE_TOR` | `false` | Start Tor hidden service |

## Docker Compose Requirements (NON-NEGOTIABLE)

**Location:** `./docker/docker-compose.yml`

| Requirement | Value |
|-------------|-------|
| `build:` | **NEVER include** |
| `version:` | **NEVER include** |
| `name:` | `airports` |
| Container name | `airports` |
| Network | Custom `airports` with `external: false` |
| Environment variables | **Hardcode with sane defaults** (NEVER use .env files) |
| **environment: MODE** | **production** (strict host validation) |

### Volume Paths (Host Side)

**All paths in docker-compose.yml use `./rootfs/` (project root relative).**

| Type | Path in docker-compose.yml | Container Path |
|------|---------------------------|----------------|
| Config | `./rootfs/config/{servicename}` | `/config` |
| Data | `./rootfs/data/{servicename}` | `/data` |
| PostgreSQL | `./rootfs/db/postgres` | `/var/lib/postgresql/data` |
| MariaDB | `./rootfs/db/mariadb` | `/var/lib/mysql` |
| Redis | `./rootfs/db/redis` | `/data` |
| MongoDB | `./rootfs/db/mongodb` | `/data/db` |

**Rules:**
- `{servicename}` = the service name in docker-compose (e.g., `airports`, `api`, `web`)
- `./docker/rootfs/` is for container overlay (entrypoint.sh) - NOT for runtime volumes
- Volume paths in repo use `./rootfs/` (relative notation for readability)

### Running Docker Compose (NON-NEGOTIABLE)

**NEVER run docker compose in the project directory.**

**Always use temp directory workflow:**
1. Create unique temp dir: `/tmp/airports-{unique}/`
2. Copy `./docker/docker-compose.yml` to temp dir
3. Create `rootfs/` structure in temp dir
4. Run docker compose from temp dir
5. Data lives in temp dir, isolated from project

```bash
# Setup
PROJECT_ROOT="/path/to/project"
TEMP_DIR="/tmp/airports-$(date +%s)"
mkdir -p "$TEMP_DIR/rootfs/config/airports"
mkdir -p "$TEMP_DIR/rootfs/data/airports"
mkdir -p "$TEMP_DIR/rootfs/db"

# Copy docker-compose.yml
cp "$PROJECT_ROOT/docker/docker-compose.yml" "$TEMP_DIR/"

# Run from temp dir - ./rootfs/ resolves to $TEMP_DIR/rootfs/
cd "$TEMP_DIR" && docker compose up -d

# Stop and cleanup
cd "$TEMP_DIR" && docker compose down
rm -rf "$TEMP_DIR"
```

**Why temp dir:**
- Project directory stays clean
- Data isolated from source code
- Multiple instances possible
- Safe cleanup

**NEVER:**
- Run docker compose in project directory
- Run docker compose with `--project-directory` pointing to project root
- Mount volumes to `{project_root}/rootfs/`

### Port Mapping (NON-NEGOTIABLE)

| Mode | Format | Example |
|------|--------|---------|
| Development | `{randomport}:80` | `64580:80` |
| Production | `172.17.0.1:{randomport}:80` | `172.17.0.1:64580:80` |

**Rules:**
- Internal port is ALWAYS `80`
- External port is random unused port in `64xxx` range
- Production binds to Docker bridge IP (`172.17.0.1`) for security
- Development binds to all interfaces for easier access

### Environment Variables (NON-NEGOTIABLE)

**ALL environment variables MUST be hardcoded with sane defaults. NEVER require .env files.**

| Rule | Description |
|------|-------------|
| **NEVER** | Use `${VAR}` or `${VAR:-default}` syntax requiring .env |
| **NEVER** | Create `.env`, `.env.example`, `.env.sample` files |
| **ALWAYS** | Hardcode values directly in docker-compose.yml |
| **ALWAYS** | Use sane, working defaults |

**Why hardcoded defaults?**
- Works out of the box - no setup required
- No confusion about required variables
- No outdated .env.example files
- Users can override by editing docker-compose.yml directly

### Docker Compose Example (Development)

**Location:** `./docker/docker-compose.dev.yml`

```yaml
name: airports

services:
  airports:
    image: ghcr.io/apimgr/airports:latest
    container_name: airports
    restart: unless-stopped
    environment:
      # Hardcoded with sane defaults - NO .env files
      - MODE=development
      - TZ=America/New_York
      - ENABLE_TOR=false
    ports:
      # Development: accessible from all interfaces
      - "64580:80"
    volumes:
      - ./rootfs/config/airports:/config
      - ./rootfs/data/airports:/data
    networks:
      - airports

networks:
  airports:
    name: airports
    external: false
```

**Run:** Use temp directory workflow (see "Running Docker Compose" section above).

### Docker Compose Example (Production)

**Location:** `./docker/docker-compose.yml`

```yaml
name: airports

services:
  airports:
    image: ghcr.io/apimgr/airports:latest
    container_name: airports
    restart: unless-stopped
    environment:
      # Hardcoded with sane defaults - NO .env files
      - MODE=production
      - TZ=America/New_York
      - ENABLE_TOR=true
    ports:
      # Production: bound to Docker bridge only
      - "172.17.0.1:64580:80"
    volumes:
      - ./rootfs/config/airports:/config
      - ./rootfs/data/airports:/data
    networks:
      - airports

networks:
  airports:
    name: airports
    external: false
```

**Run:** Use temp directory workflow (see "Running Docker Compose" section above).

### Docker Compose with Database Example

**Location:** `./docker/docker-compose.yml`

```yaml
name: airports

services:
  airports:
    image: ghcr.io/apimgr/airports:latest
    container_name: airports
    restart: unless-stopped
    depends_on:
      - postgres
    environment:
      - MODE=production
      - TZ=America/New_York
      - ENABLE_TOR=true
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_NAME=airports
      - DB_USER=airports
      - DB_PASS=airports
    ports:
      - "172.17.0.1:64580:80"
    volumes:
      - ./rootfs/config/airports:/config
      - ./rootfs/data/airports:/data
    networks:
      - airports

  postgres:
    image: postgres:alpine
    container_name: airports-postgres
    restart: unless-stopped
    environment:
      - POSTGRES_DB=airports
      - POSTGRES_USER=airports
      - POSTGRES_PASSWORD=airports
      - TZ=America/New_York
    volumes:
      - ./rootfs/db/postgres:/var/lib/postgresql/data
    networks:
      - airports

networks:
  airports:
    name: airports
    external: false
```

**Run:** Use temp directory workflow (see "Running Docker Compose" section above).

## Container Configuration

| Setting | Value |
|---------|-------|
| Internal port | **80** (always) |
| Config dir | `/config` |
| Data dir | `/data` |
| Database dir | `/data/db` (server.db, users.db) |
| Log dir | `/data/logs` |
| Tor data dir | `/data/tor` |
| GeoIP dir | `/data/geoip` |
| Backup dir | `/data/backup` |
| Binary | `/usr/local/bin/airports` |
| HEALTHCHECK | `{binary} --status` |

**Path Mapping (Container vs Host):**

| Container Path | Host Path | Purpose |
|----------------|-----------|---------|
| `/config` | `./rootfs/config/{servicename}` | Configuration files |
| `/data` | `./rootfs/data/{servicename}` | All persistent data |
| `/data/db` | (inside /data) | SQLite databases |
| `/data/logs` | (inside /data) | Log files |

## Tor in Container

**Tor is included in the container image and managed by the application.**

| Behavior | Description |
|----------|-------------|
| Auto-start | Application starts Tor automatically if enabled |
| Data persistence | Tor keys stored in `/data/tor/site/` (survives container restart) |
| .onion address | Persists across container restarts via volume mount |

## Container Detection

**Assume running in container if tini init system (PID 1) is detected.**

| When in Container | Behavior |
|-------------------|----------|
| Show setup token | On first run - one-time setup token displayed in console for `/admin` access |
| Defaults | Use container-appropriate defaults |
| Logging | Log to stdout/stderr (captured by container runtime) |
| Tor | Application manages Tor process internally |

## Image Tags (NON-NEGOTIABLE)

### Release Tags (Production)

| Tag | Description | Example |
|-----|-------------|---------|
| `ghcr.io/apimgr/airports:latest` | Latest stable release | `ghcr.io/myorg/myapp:latest` |
| `ghcr.io/apimgr/airports:{version}` | Specific version | `ghcr.io/myorg/myapp:1.2.3` |
| `ghcr.io/apimgr/airports:{YYMM}` | Year/month tag | `ghcr.io/myorg/myapp:2512` |
| `ghcr.io/apimgr/airports:{commit}` | Git commit (7 char) | `ghcr.io/myorg/myapp:abc1234` |

### Development Tags (Local)

| Tag | Description | Example |
|-----|-------------|---------|
| `airports:dev` | Local development build | `myapp:dev` |
| `airports:test` | Local test build | `myapp:test` |

### Registry

| Environment | Registry |
|-------------|----------|
| Release | `ghcr.io` (GitHub Container Registry) |
| Development | Local Docker daemon only |

### Tag Rules

1. **Release builds** MUST push to `ghcr.io/apimgr/airports`
2. **Development builds** MUST use local-only tags (no registry prefix)
3. **NEVER push `:dev` or `:test` tags to ghcr.io**
4. All release images built for `linux/amd64` AND `linux/arm64`

---

# PART 20: MAKEFILE (NON-NEGOTIABLE)

**Four targets only. DO NOT ADD MORE.**

## Targets

| Target | Description | Output |
|--------|-------------|--------|
| `build` | Build all platforms + host binary | `./binaries/` |
| `release` | GitHub release with source archive | `./releases/` |
| `docker` | Build and push container to ghcr.io | ghcr.io |
| `test` | Run all tests | - |

## Versioning (NON-NEGOTIABLE)

### Version File: `./release.txt`

- If `VERSION` env var is set, use it (highest priority)
- If `./release.txt` exists, read version from it
- If `./release.txt` does not exist, create with current app version
- Auto-increment patch version on each release
- Semantic versioning: `MAJOR.MINOR.PATCH` (e.g., `1.2.3`)

### Version Tag `v` Prefix Rules (NON-NEGOTIABLE)

**Only add `v` prefix if BOTH conditions are met:**
1. Tag does NOT already start with `v`
2. Tag IS a semantic version (matches `X.Y.Z` pattern)

| Tag Input | Is Semver? | Has `v`? | Display As |
|-----------|------------|----------|------------|
| `1.2.0` | ✓ | ✗ | `v1.2.0` |
| `v1.2.0` | ✓ | ✓ | `v1.2.0` |
| `1.2.3-rc1` | ✓ | ✗ | `v1.2.3-rc1` |
| `dev` | ✗ | - | `dev` |
| `beta` | ✗ | - | `beta` |
| `daily` | ✗ | - | `daily` |
| `20251218060432` | ✗ | - | `20251218060432` |
| `20251218060432-beta` | ✗ | - | `20251218060432-beta` |

```bash
# Shell function to format version tag
format_version_tag() {
    local tag="$1"
    # Check if already starts with v
    if [[ "$tag" == v* ]]; then
        echo "$tag"
        return
    fi
    # Check if it's a semantic version (X.Y.Z with optional suffix)
    if [[ "$tag" =~ ^[0-9]+\.[0-9]+\.[0-9]+ ]]; then
        echo "v$tag"
    else
        echo "$tag"
    fi
}
```

### Version Priority

1. `VERSION` environment variable (if set)
2. `./release.txt` file (if exists)
3. Create `./release.txt` with `0.1.0` (first release)

## Binary Naming (NON-NEGOTIABLE)

| Context | Name | Example |
|---------|------|---------|
| Host Build | `./binaries/airports` | `./binaries/jokes` |
| Distribution | `airports-{os}-{arch}` | `jokes-linux-amd64` |
| Local/Testing | `/tmp/apimgr-build/airports/airports` | - |

**If built with musl → strip binary before release. Final name has NO `-musl` suffix.**

## Build Matrix

| OS | Architectures |
|----|---------------|
| Linux | amd64, arm64 |
| macOS (Darwin) | amd64, arm64 |
| Windows | amd64, arm64 |
| FreeBSD | amd64, arm64 |

## Makefile Implementation

```makefile
PROJECT := airports
ORG := apimgr

# Version: env var > release.txt > default
VERSION ?= $(shell cat release.txt 2>/dev/null || echo "0.1.0")

# Build info - use TZ env var or system timezone
# Format: "Thu Dec 17, 2025 at 18:19:24 EST"
BUILD_DATE := $(shell date +"%%a %%b %%d, %%Y at %%H:%%M:%%S %%Z")
COMMIT_ID := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VCS_REF := $(COMMIT_ID)

# Linker flags to embed build info
LDFLAGS := -s -w \
	-X 'main.Version=$(VERSION)' \
	-X 'main.CommitID=$(COMMIT_ID)' \
	-X 'main.BuildDate=$(BUILD_DATE)'

# Directories
BINDIR := ./binaries
RELDIR := ./releases

# Go module cache (persistent across builds)
GOCACHE := $(HOME)/.cache/go-build
GOMODCACHE := $(HOME)/go/pkg/mod

# Build targets
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64 freebsd/arm64

# Docker
REGISTRY := ghcr.io/$(ORG)/$(PROJECT)
GO_DOCKER := docker run --rm \
	-v $(PWD):/build \
	-v $(GOCACHE):/root/.cache/go-build \
	-v $(GOMODCACHE):/go/pkg/mod \
	-w /build \
	-e CGO_ENABLED=0 \
	golang:alpine

.PHONY: build release docker test clean

# =============================================================================
# BUILD - Build all platforms + host binary (via Docker with cached modules)
# =============================================================================
build: clean
	@mkdir -p $(BINDIR)
	@echo "Building version $(VERSION)..."
	@mkdir -p $(GOCACHE) $(GOMODCACHE)

	# Download modules first (cached)
	@echo "Downloading Go modules..."
	@$(GO_DOCKER) go mod download

	# Build for host OS/ARCH
	@echo "Building host binary..."
	@$(GO_DOCKER) sh -c "GOOS=$$(go env GOOS) GOARCH=$$(go env GOARCH) \
		go build -ldflags \"$(LDFLAGS)\" -o $(BINDIR)/$(PROJECT) ./src"

	# Build all platforms
	@for platform in $(PLATFORMS); do \
		OS=$${platform%/*}; \
		ARCH=$${platform#*/}; \
		OUTPUT=$(BINDIR)/$(PROJECT)-$$OS-$$ARCH; \
		[ "$$OS" = "windows" ] && OUTPUT=$$OUTPUT.exe; \
		echo "Building $$OS/$$ARCH..."; \
		$(GO_DOCKER) sh -c "GOOS=$$OS GOARCH=$$ARCH \
			go build -ldflags \"$(LDFLAGS)\" \
			-o $$OUTPUT ./src" || exit 1; \
	done

	@echo "Build complete: $(BINDIR)/"

# =============================================================================
# RELEASE - Manual local release (stable only)
# =============================================================================
release: build
	@mkdir -p $(RELDIR)
	@echo "Preparing release $(VERSION)..."

	# Create version.txt
	@echo "$(VERSION)" > $(RELDIR)/version.txt

	# Copy binaries to releases (strip if needed)
	@for f in $(BINDIR)/$(PROJECT)-*; do \
		[ -f "$$f" ] || continue; \
		strip "$$f" 2>/dev/null || true; \
		cp "$$f" $(RELDIR)/; \
	done

	# Create source archive (exclude VCS and build artifacts)
	@tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
		--exclude='binaries' --exclude='releases' --exclude='*.tar.gz' \
		-czf $(RELDIR)/$(PROJECT)-$(VERSION)-source.tar.gz .

	# Delete existing release/tag if exists
	@gh release delete $(VERSION) --yes 2>/dev/null || true
	@git tag -d $(VERSION) 2>/dev/null || true
	@git push origin :refs/tags/$(VERSION) 2>/dev/null || true

	# Create new release (stable)
	@gh release create $(VERSION) $(RELDIR)/* \
		--title "$(PROJECT) $(VERSION)" \
		--notes "Release $(VERSION)" \
		--latest

	@echo "Release complete: $(VERSION)"

# =============================================================================
# DOCKER - Build and push container to ghcr.io
# =============================================================================
docker:
	@echo "Building Docker image $(VERSION)..."
	@mkdir -p $(GOCACHE) $(GOMODCACHE) $(BINDIR)

	# Ensure buildx is available
	@docker buildx version > /dev/null 2>&1 || (echo "docker buildx required" && exit 1)

	# Download modules first (cached)
	@echo "Downloading Go modules..."
	@$(GO_DOCKER) go mod download

	# Build Linux binaries for container (amd64 and arm64) to ./binaries/
	@echo "Building Linux binaries for container..."
	@$(GO_DOCKER) sh -c "GOOS=linux GOARCH=amd64 \
		go build -ldflags \"$(LDFLAGS)\" \
		-o $(BINDIR)/$(PROJECT)-linux-amd64 ./src"
	@$(GO_DOCKER) sh -c "GOOS=linux GOARCH=arm64 \
		go build -ldflags \"$(LDFLAGS)\" \
		-o $(BINDIR)/$(PROJECT)-linux-arm64 ./src"

	# Create/use builder
	@docker buildx create --name $(PROJECT)-builder --use 2>/dev/null || \
		docker buildx use $(PROJECT)-builder

	# Build and push multi-arch (context is project root, Dockerfile in ./docker/)
	@docker buildx build \
		-f ./docker/Dockerfile \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION="$(VERSION)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		--build-arg VCS_REF="$(VCS_REF)" \
		-t $(REGISTRY):$(VERSION) \
		-t $(REGISTRY):latest \
		--push \
		.

	@echo "Docker push complete: $(REGISTRY):$(VERSION)"

# =============================================================================
# TEST - Run all tests (via Docker with cached modules)
# =============================================================================
test:
	@echo "Running tests in Docker..."
	@mkdir -p $(GOCACHE) $(GOMODCACHE)
	@$(GO_DOCKER) go mod download
	@$(GO_DOCKER) go test -v -cover ./...
	@echo "Tests complete"

# =============================================================================
# CLEAN - Remove build artifacts
# =============================================================================
clean:
	@rm -rf $(BINDIR) $(RELDIR)
```

## Embedded Build Info (NON-NEGOTIABLE)

Every binary MUST have these values embedded at build time:

| Variable | Example | Description |
|----------|---------|-------------|
| `Version` | `1.2.3` | Semantic version from release.txt |
| `CommitID` | `a1b2c3d` | Git short commit hash |
| `BuildDate` | `Thu Dec 17, 2025 at 18:19:24 EST` | Build timestamp with timezone |

**Go code requirement** (in `main.go` or `version.go`):

```go
// Build info - set via -ldflags at build time
var (
    Version   = "dev"
    CommitID  = "unknown"
    BuildDate = "unknown"
)
```

**Build date format:** Uses build system timezone or `TZ` env var.

## Go Module Caching

All Docker builds use persistent Go module caching to avoid re-downloading dependencies:

| Cache | Host Path | Container Path |
|-------|-----------|----------------|
| Build cache | `~/.cache/go-build` | `/root/.cache/go-build` |
| Module cache | `~/go/pkg/mod` | `/go/pkg/mod` |

**Benefits:**
- First build downloads modules once
- Subsequent builds use cached modules
- Shared across all projects on the same machine
- Significantly faster builds after first run

## Target Details

### `make build`

1. Creates cache directories if needed
2. Downloads Go modules (cached)
3. Creates `./binaries/` directory
4. Builds host binary: `./binaries/airports`
5. Builds all platform binaries: `./binaries/airports-{os}-{arch}`
6. Uses `CGO_ENABLED=0` for static binaries
7. Embeds Version, CommitID, BuildDate via `-ldflags`
8. All builds via Docker (`golang:alpine`)

### `make release`

1. Runs `build` first
2. Creates `releases/version.txt` with current version
3. Strips binaries (removes symbols)
4. Creates source archive (excludes `.git`, `.github`, `.gitea`, `binaries/`, `releases/`)
5. Deletes existing GitHub release/tag if exists
6. Creates new GitHub release marked as `--latest`
7. Uses `gh` CLI - **manual local release only**

### `make docker`

1. Downloads Go modules (cached)
2. Builds Linux binaries to `./binaries/` (amd64 and arm64)
3. Uses `docker buildx` for multi-arch builds
4. Context is project root, Dockerfile at `./docker/Dockerfile`
5. Builds for `linux/amd64` and `linux/arm64`
6. Pushes to `ghcr.io/{org}/airports:{version}` and `:latest`
7. Passes VERSION, BUILD_DATE, VCS_REF as build args

### `make test`

1. Downloads Go modules (cached)
2. Runs tests inside Docker container (`golang:alpine`)
3. Mounts project directory to `/build`
4. Runs `go test` with coverage
5. Tests all packages (`./...`)
6. Container removed after completion (`--rm`)

## Directory Rules (NON-NEGOTIABLE)

| Rule | Description |
|------|-------------|
| **NEVER symlink** | Never symlink between `binaries/` and `releases/` |
| **NEVER copy** | Never copy files between `binaries/` and `releases/` |
| **binaries/** | Build output only - temporary, gitignored |
| **releases/** | Release artifacts only - packaged for distribution |
| **version.txt** | Every release includes `version.txt` with current version |

## Release Types

### Stable Release

| Property | Value |
|----------|-------|
| Trigger | Git tag push (`v*`, `*.*.*`) |
| Version format | Semantic (`X.Y.Z`) |
| Release name | `{tag}` or `v{tag}` if semver without `v` (see v prefix rules) |
| version.txt | `{version}` (without `v` prefix) |
| GitHub release | Yes, marked as latest |

### Beta Release

| Property | Value |
|----------|-------|
| Trigger | Push to `beta` branch |
| Version format | `{YYYYMMDDHHMMSS}-beta` (e.g., `20251205143022-beta`) |
| Release name | `{YYYYMMDDHHMMSS}-beta` |
| version.txt | `{YYYYMMDDHHMMSS}-beta` |
| GitHub release | Yes, marked as pre-release |

### Daily Build

| Property | Value |
|----------|-------|
| Trigger | Daily schedule (3am UTC) + push to main/master |
| Version format | `{YYYYMMDDHHMMSS}` (e.g., `20251218060432`) |
| Release name | `daily` (single rolling release) |
| version.txt | `{YYYYMMDDHHMMSS}` |
| GitHub release | Yes, **replaces previous daily** |
| Max releases | **1** (always overwrites previous daily) |

**Daily Build Rules:**
- Only ONE daily release exists at any time
- Each daily build **deletes and replaces** the previous `daily` release
- Prevents accumulation of thousands of releases
- Users always get the latest daily build

## Version Files

| File | Purpose | When Updated |
|------|---------|--------------|
| `release.txt` | Source of truth for stable version | Manual |
| `releases/version.txt` | Included in release archive | During release build |

## Release Summary

| Type | Method | Version Example | Max Releases |
|------|--------|-----------------|--------------|
| Stable | GitHub Actions (tag) or `make release` (local) | `1.2.3` | Unlimited |
| Beta | GitHub Actions only | `20251205143022-beta` | Unlimited |
| Daily | GitHub Actions only | `20251218060432` | **1** (rolling) |

**Note:** `make release` is for manual local releases only. All automated releases use GitHub Actions.

---

# PART 21: GITHUB ACTIONS (NON-NEGOTIABLE)

**All projects MUST have GitHub Actions workflows.**

## Workflow Files

| File | Trigger | Purpose |
|------|---------|---------|
| `release.yml` | Tag push (`v*`, `*.*.*`) | Production releases |
| `beta.yml` | Push to `beta` branch | Beta releases |
| `daily.yml` | Daily at 3am UTC + push to main/master | Daily builds |
| `docker.yml` | Version tag, push to main/master/beta | Docker images |

## Build Info Variables

All workflows MUST set these environment variables:

```yaml
env:
  VERSION: ${{ github.ref_name }}  # or from release.txt
  COMMIT_ID: ${{ github.sha }}
  BUILD_DATE: $(date +"%a %b %d, %Y at %H:%M:%S %Z")
  LDFLAGS: "-s -w -X 'main.Version=${{ env.VERSION }}' -X 'main.CommitID=${{ env.COMMIT_ID }}' -X 'main.BuildDate=${{ env.BUILD_DATE }}'"
```

## Release Workflow (Stable)

**File:** `.github/workflows/release.yml`

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'
      - '[0-9]*.[0-9]*.[0-9]*'

env:
  PROJECT: airports

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: linux
            goarch: amd64
          - goos: linux
            goarch: arm64
          - goos: darwin
            goarch: amd64
          - goos: darwin
            goarch: arm64
          - goos: windows
            goarch: amd64
            ext: .exe
          - goos: windows
            goarch: arm64
            ext: .exe
          - goos: freebsd
            goarch: amd64
          - goos: freebsd
            goarch: arm64

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: 'stable'

      - name: Set build info
        run: |
          echo "VERSION=${GITHUB_REF_NAME#v}" >> $GITHUB_ENV
          echo "COMMIT_ID=$(git rev-parse --short HEAD)" >> $GITHUB_ENV
          echo "BUILD_DATE=$(date +"%a %b %d, %Y at %H:%M:%S %Z")" >> $GITHUB_ENV

      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: 0
        run: |
          LDFLAGS="-s -w -X 'main.Version=${{ env.VERSION }}' -X 'main.CommitID=${{ env.COMMIT_ID }}' -X 'main.BuildDate=${{ env.BUILD_DATE }}'"
          go build -ldflags "${LDFLAGS}" -o ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}${{ matrix.ext }} ./src

      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}
          path: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}${{ matrix.ext }}

  release:
    needs: build
    runs-on: ubuntu-latest
    permissions:
      contents: write

    steps:
      - uses: actions/checkout@v4

      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          path: binaries
          merge-multiple: true

      - name: Set version
        run: |
          TAG="${GITHUB_REF_NAME}"
          # VERSION for version.txt (strip v if present)
          echo "VERSION=${TAG#v}" >> $GITHUB_ENV
          # RELEASE_TAG: add v prefix only if semver without v
          if [[ "$TAG" == v* ]]; then
            echo "RELEASE_TAG=$TAG" >> $GITHUB_ENV
          elif [[ "$TAG" =~ ^[0-9]+\.[0-9]+\.[0-9]+ ]]; then
            echo "RELEASE_TAG=v$TAG" >> $GITHUB_ENV
          else
            echo "RELEASE_TAG=$TAG" >> $GITHUB_ENV
          fi

      - name: Create version.txt
        run: echo "${{ env.VERSION }}" > binaries/version.txt

      - name: Create source archive
        run: |
          tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
            --exclude='binaries' --exclude='releases' --exclude='*.tar.gz' \
            -czf binaries/${{ env.PROJECT }}-${{ env.VERSION }}-source.tar.gz .

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          tag_name: ${{ env.RELEASE_TAG }}
          files: binaries/*
          generate_release_notes: true
          make_latest: true
```

## Beta Workflow

**File:** `.github/workflows/beta.yml`

```yaml
name: Beta Release

on:
  push:
    branches:
      - beta

env:
  PROJECT: airports

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: linux
            goarch: amd64
          - goos: linux
            goarch: arm64

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: 'stable'

      - name: Set build info
        run: |
          echo "VERSION=$(date -u +"%Y%m%d%H%M%S")-beta" >> $GITHUB_ENV
          echo "COMMIT_ID=$(git rev-parse --short HEAD)" >> $GITHUB_ENV
          echo "BUILD_DATE=$(date +"%a %b %d, %Y at %H:%M:%S %Z")" >> $GITHUB_ENV

      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: 0
        run: |
          LDFLAGS="-s -w -X 'main.Version=${{ env.VERSION }}' -X 'main.CommitID=${{ env.COMMIT_ID }}' -X 'main.BuildDate=${{ env.BUILD_DATE }}'"
          go build -ldflags "${LDFLAGS}" -o ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }} ./src

      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}
          path: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}

  release:
    needs: build
    runs-on: ubuntu-latest
    permissions:
      contents: write

    steps:
      - uses: actions/checkout@v4

      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          path: binaries
          merge-multiple: true

      - name: Set version
        run: echo "VERSION=$(date -u +"%Y%m%d%H%M%S")-beta" >> $GITHUB_ENV

      - name: Create version.txt
        run: echo "${{ env.VERSION }}" > binaries/version.txt

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          tag_name: ${{ env.VERSION }}
          files: binaries/*
          prerelease: true
          generate_release_notes: true
```

## Daily Workflow

**File:** `.github/workflows/daily.yml`

```yaml
name: Daily Build

on:
  schedule:
    - cron: '0 3 * * *'  # 3am UTC daily
  push:
    branches:
      - main
      - master
  workflow_dispatch:

env:
  PROJECT: airports

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: linux
            goarch: amd64
          - goos: linux
            goarch: arm64

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: 'stable'

      - name: Set build info
        run: |
          echo "VERSION=$(date -u +"%Y%m%d%H%M%S")" >> $GITHUB_ENV
          echo "COMMIT_ID=$(git rev-parse --short HEAD)" >> $GITHUB_ENV
          echo "BUILD_DATE=$(date +"%a %b %d, %Y at %H:%M:%S %Z")" >> $GITHUB_ENV

      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: 0
        run: |
          LDFLAGS="-s -w -X 'main.Version=${{ env.VERSION }}' -X 'main.CommitID=${{ env.COMMIT_ID }}' -X 'main.BuildDate=${{ env.BUILD_DATE }}'"
          go build -ldflags "${LDFLAGS}" -o ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }} ./src

      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}
          path: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}

  release:
    needs: build
    runs-on: ubuntu-latest
    permissions:
      contents: write

    steps:
      - uses: actions/checkout@v4

      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          path: binaries
          merge-multiple: true

      - name: Set version
        run: echo "VERSION=$(date -u +"%Y%m%d%H%M%S")" >> $GITHUB_ENV

      - name: Create version.txt
        run: echo "${{ env.VERSION }}" > binaries/version.txt

      - name: Delete previous daily release
        run: |
          gh release delete daily --yes 2>/dev/null || true
          git push origin :refs/tags/daily 2>/dev/null || true
        env:
          GH_TOKEN: ${{ github.token }}

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          tag_name: daily
          name: "Daily Build ${{ env.VERSION }}"
          files: binaries/*
          prerelease: true
          body: "Daily build: ${{ env.VERSION }}"
```

## Docker Workflow

### Triggers and Tags

| Trigger | Image Tags |
|---------|------------|
| Version tag | `{version}`, `latest`, `YYMM`, `{GIT_COMMIT}` |
| Push to main/master | `dev`, `{GIT_COMMIT}` |
| Push to beta | `beta`, `{GIT_COMMIT}` |

**Notes:**
- `{GIT_COMMIT}` = short SHA (7 characters)
- `YYMM` = year/month (e.g., `2512`)
- Built for `linux/amd64` and `linux/arm64` using `docker buildx`
- Registry: `ghcr.io`

**File:** `.github/workflows/docker.yml`

```yaml
name: Docker Build

on:
  push:
    branches:
      - main
      - master
      - beta
    tags:
      - 'v*'
      - '*.*.*'
  workflow_dispatch:

env:
  PROJECT: airports
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Set build info
        run: |
          echo "GIT_COMMIT=$(git rev-parse --short HEAD)" >> $GITHUB_ENV
          echo "YYMM=$(date +"%y%m")" >> $GITHUB_ENV
          if [[ "${{ github.ref }}" == refs/tags/* ]]; then
            echo "VERSION=${GITHUB_REF#refs/tags/}" >> $GITHUB_ENV
            echo "IS_TAG=true" >> $GITHUB_ENV
          else
            echo "VERSION=$(git rev-parse --short HEAD)" >> $GITHUB_ENV
            echo "IS_TAG=false" >> $GITHUB_ENV
          fi
          echo "BUILD_DATE=$(date +"%a %b %d, %Y at %H:%M:%S %Z")" >> $GITHUB_ENV
          echo "COMMIT_ID=$(git rev-parse --short HEAD)" >> $GITHUB_ENV

      - name: Determine tags
        id: tags
        run: |
          TAGS="${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ env.GIT_COMMIT }}"

          if [[ "${{ env.IS_TAG }}" == "true" ]]; then
            # Version tag - add version, latest, YYMM
            TAGS="$TAGS,${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ env.VERSION }}"
            TAGS="$TAGS,${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:latest"
            TAGS="$TAGS,${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ env.YYMM }}"
          elif [[ "${{ github.ref }}" == refs/heads/beta ]]; then
            # Beta branch
            TAGS="$TAGS,${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:beta"
          else
            # Main/master branch
            TAGS="$TAGS,${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:dev"
          fi

          echo "tags=$TAGS" >> $GITHUB_OUTPUT

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: ./docker/Dockerfile
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.tags.outputs.tags }}
          build-args: |
            VERSION="${{ env.VERSION }}"
            BUILD_DATE="${{ env.BUILD_DATE }}"
            COMMIT_ID="${{ env.COMMIT_ID }}"
          labels: |
            org.opencontainers.image.title="${{ env.PROJECT }}"
            org.opencontainers.image.version="${{ env.VERSION }}"
            org.opencontainers.image.created="${{ env.BUILD_DATE }}"
            org.opencontainers.image.revision="${{ env.COMMIT_ID }}"
            org.opencontainers.image.source="${{ github.server_url }}/${{ github.repository }}"
```

---

# GITEA ACTIONS (Alternative to GitHub)

**For projects hosted on Gitea, use these equivalent workflows.**

Gitea Actions use nearly identical syntax to GitHub Actions. Key differences:
- Directory: `.gitea/workflows/` instead of `.github/workflows/`
- Use `gitea.com/actions/*` actions or compatible alternatives
- Registry: Use Gitea's container registry or specify external registry
- Some GitHub-specific variables have Gitea equivalents

## Workflow Files

| File | Trigger | Purpose |
|------|---------|---------|
| `release.yml` | Tag push (`v*`, `*.*.*`) | Production releases |
| `beta.yml` | Push to `beta` branch | Beta releases |
| `daily.yml` | Daily at 3am UTC + push to main/master | Daily builds |
| `docker.yml` | Version tag, push to main/master/beta | Docker images |

## Release Workflow (Stable)

**File:** `.gitea/workflows/release.yml`

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'
      - '[0-9]*.[0-9]*.[0-9]*'

env:
  PROJECT: airports

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: linux
            goarch: amd64
          - goos: linux
            goarch: arm64
          - goos: darwin
            goarch: amd64
          - goos: darwin
            goarch: arm64
          - goos: windows
            goarch: amd64
            ext: .exe
          - goos: windows
            goarch: arm64
            ext: .exe
          - goos: freebsd
            goarch: amd64
          - goos: freebsd
            goarch: arm64

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: 'stable'

      - name: Set build info
        run: |
          echo "VERSION=${GITEA_REF_NAME#v}" >> $GITEA_ENV
          echo "COMMIT_ID=$(git rev-parse --short HEAD)" >> $GITEA_ENV
          echo "BUILD_DATE=$(date +"%a %b %d, %Y at %H:%M:%S %Z")" >> $GITEA_ENV

      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: 0
        run: |
          LDFLAGS="-s -w -X 'main.Version=${{ env.VERSION }}' -X 'main.CommitID=${{ env.COMMIT_ID }}' -X 'main.BuildDate=${{ env.BUILD_DATE }}'"
          go build -ldflags "${LDFLAGS}" -o ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}${{ matrix.ext }} ./src

      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}
          path: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}${{ matrix.ext }}

  release:
    needs: build
    runs-on: ubuntu-latest
    permissions:
      contents: write

    steps:
      - uses: actions/checkout@v4

      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          path: binaries
          merge-multiple: true

      - name: Set version
        run: echo "VERSION=${GITEA_REF_NAME#v}" >> $GITEA_ENV

      - name: Create version.txt
        run: echo "${{ env.VERSION }}" > binaries/version.txt

      - name: Create source archive
        run: |
          tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
            --exclude='binaries' --exclude='releases' --exclude='*.tar.gz' \
            -czf binaries/${{ env.PROJECT }}-${{ env.VERSION }}-source.tar.gz .

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: binaries/*
          generate_release_notes: true
```

## Beta Workflow

**File:** `.gitea/workflows/beta.yml`

```yaml
name: Beta Release

on:
  push:
    branches:
      - beta

env:
  PROJECT: airports

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: linux
            goarch: amd64
          - goos: linux
            goarch: arm64

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: 'stable'

      - name: Set build info
        run: |
          echo "VERSION=$(date -u +"%Y%m%d%H%M%S")-beta" >> $GITEA_ENV
          echo "COMMIT_ID=$(git rev-parse --short HEAD)" >> $GITEA_ENV
          echo "BUILD_DATE=$(date +"%a %b %d, %Y at %H:%M:%S %Z")" >> $GITEA_ENV

      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: 0
        run: |
          LDFLAGS="-s -w -X 'main.Version=${{ env.VERSION }}' -X 'main.CommitID=${{ env.COMMIT_ID }}' -X 'main.BuildDate=${{ env.BUILD_DATE }}'"
          go build -ldflags "${LDFLAGS}" -o ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }} ./src

      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}
          path: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}

  release:
    needs: build
    runs-on: ubuntu-latest
    permissions:
      contents: write

    steps:
      - uses: actions/checkout@v4

      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          path: binaries
          merge-multiple: true

      - name: Set version
        run: echo "VERSION=$(date -u +"%Y%m%d%H%M%S")-beta" >> $GITEA_ENV

      - name: Create version.txt
        run: echo "${{ env.VERSION }}" > binaries/version.txt

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          tag_name: ${{ env.VERSION }}
          files: binaries/*
          prerelease: true
          generate_release_notes: true
```

## Daily Workflow

**File:** `.gitea/workflows/daily.yml`

```yaml
name: Daily Build

on:
  schedule:
    - cron: '0 3 * * *'  # 3am UTC daily
  push:
    branches:
      - main
      - master
  workflow_dispatch:

env:
  PROJECT: airports

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: linux
            goarch: amd64
          - goos: linux
            goarch: arm64

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: 'stable'

      - name: Set build info
        run: |
          echo "VERSION=$(date -u +"%Y%m%d%H%M%S")" >> $GITEA_ENV
          echo "COMMIT_ID=$(git rev-parse --short HEAD)" >> $GITEA_ENV
          echo "BUILD_DATE=$(date +"%a %b %d, %Y at %H:%M:%S %Z")" >> $GITEA_ENV

      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: 0
        run: |
          LDFLAGS="-s -w -X 'main.Version=${{ env.VERSION }}' -X 'main.CommitID=${{ env.COMMIT_ID }}' -X 'main.BuildDate=${{ env.BUILD_DATE }}'"
          go build -ldflags "${LDFLAGS}" -o ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }} ./src

      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}
          path: ${{ env.PROJECT }}-${{ matrix.goos }}-${{ matrix.goarch }}

  release:
    needs: build
    runs-on: ubuntu-latest
    permissions:
      contents: write

    steps:
      - uses: actions/checkout@v4

      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          path: binaries
          merge-multiple: true

      - name: Set version
        run: echo "VERSION=$(date -u +"%Y%m%d%H%M%S")" >> $GITEA_ENV

      - name: Create version.txt
        run: echo "${{ env.VERSION }}" > binaries/version.txt

      - name: Delete previous daily release
        run: |
          # Use Gitea API to delete previous daily release
          curl -X DELETE \
            -H "Authorization: token ${{ secrets.GITEA_TOKEN }}" \
            "${{ gitea.server_url }}/api/v1/repos/${{ gitea.repository }}/releases/tags/daily" || true
          git push origin :refs/tags/daily 2>/dev/null || true

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          tag_name: daily
          name: "Daily Build ${{ env.VERSION }}"
          files: binaries/*
          prerelease: true
          body: "Daily build: ${{ env.VERSION }}"
```

## Docker Workflow

**File:** `.gitea/workflows/docker.yml`

```yaml
name: Docker Build

on:
  push:
    branches:
      - main
      - master
      - beta
    tags:
      - 'v*'
      - '*.*.*'
  workflow_dispatch:

env:
  PROJECT: airports
  REGISTRY: gitea.{domain}  # or ghcr.io
  IMAGE_NAME: ${{ gitea.repository }}

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ gitea.actor }}
          password: ${{ secrets.GITEA_TOKEN }}

      - name: Set build info
        run: |
          echo "GIT_COMMIT=$(git rev-parse --short HEAD)" >> $GITEA_ENV
          echo "YYMM=$(date +"%y%m")" >> $GITEA_ENV
          if [[ "${{ gitea.ref }}" == refs/tags/* ]]; then
            echo "VERSION=${GITEA_REF_NAME}" >> $GITEA_ENV
            echo "IS_TAG=true" >> $GITEA_ENV
          else
            echo "VERSION=$(git rev-parse --short HEAD)" >> $GITEA_ENV
            echo "IS_TAG=false" >> $GITEA_ENV
          fi
          echo "BUILD_DATE=$(date +"%a %b %d, %Y at %H:%M:%S %Z")" >> $GITEA_ENV
          echo "COMMIT_ID=$(git rev-parse --short HEAD)" >> $GITEA_ENV

      - name: Determine tags
        id: tags
        run: |
          TAGS="${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ env.GIT_COMMIT }}"

          if [[ "${{ env.IS_TAG }}" == "true" ]]; then
            # Version tag - add version, latest, YYMM
            TAGS="$TAGS,${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ env.VERSION }}"
            TAGS="$TAGS,${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:latest"
            TAGS="$TAGS,${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ env.YYMM }}"
          elif [[ "${{ gitea.ref }}" == refs/heads/beta ]]; then
            # Beta branch
            TAGS="$TAGS,${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:beta"
          else
            # Main/master branch
            TAGS="$TAGS,${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:dev"
          fi

          echo "tags=$TAGS" >> $GITEA_OUTPUT

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: ./docker/Dockerfile
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.tags.outputs.tags }}
          build-args: |
            VERSION="${{ env.VERSION }}"
            BUILD_DATE="${{ env.BUILD_DATE }}"
            COMMIT_ID="${{ env.COMMIT_ID }}"
          labels: |
            org.opencontainers.image.title="${{ env.PROJECT }}"
            org.opencontainers.image.version="${{ env.VERSION }}"
            org.opencontainers.image.created="${{ env.BUILD_DATE }}"
            org.opencontainers.image.revision="${{ env.COMMIT_ID }}"
            org.opencontainers.image.source="${{ gitea.server_url }}/${{ gitea.repository }}"
```

## Gitea vs GitHub Variable Mapping

| GitHub | Gitea | Description |
|--------|-------|-------------|
| `${{ github.* }}` | `${{ gitea.* }}` | Context variables |
| `GITHUB_ENV` | `GITEA_ENV` | Environment file |
| `GITHUB_OUTPUT` | `GITEA_OUTPUT` | Output file |
| `GITHUB_REF_NAME` | `GITEA_REF_NAME` | Reference name |
| `github.token` | `secrets.GITEA_TOKEN` | Auth token |
| `github.actor` | `gitea.actor` | Username |
| `github.repository` | `gitea.repository` | Repo path |
| `github.server_url` | `gitea.server_url` | Server URL |

**Notes:**
- Most GitHub Actions work on Gitea with minimal changes
- Use `secrets.GITEA_TOKEN` for authentication (create in Gitea settings)
- Container registry URL depends on Gitea instance configuration
- Some advanced GitHub features may not be available

---

## Jenkins (NON-NEGOTIABLE)

### Configuration

| Setting | Value |
|---------|-------|
| Server | `jenkins.casjay.cc` |
| Agents | `arm64`, `amd64` |
| Build | Both architectures in parallel |
| Trigger | Push to main/master, tags, PRs |

### Jenkinsfile

All projects MUST have a `Jenkinsfile` in the repository root.

```groovy
pipeline {
    agent none

    environment {
        PROJECT = 'airports'
        ORG = 'apimgr'
        REGISTRY = "ghcr.io/${ORG}/${PROJECT}"
        BINDIR = 'binaries'
        RELDIR = 'releases'
        GOCACHE = '/tmp/go-cache'
        GOMODCACHE = '/tmp/go-mod-cache'
    }

    stages {
        stage('Setup') {
            agent { label 'amd64' }
            steps {
                script {
                    env.VERSION = sh(script: 'cat release.txt 2>/dev/null || echo "0.1.0"', returnStdout: true).trim()
                    env.COMMIT_ID = sh(script: 'git rev-parse --short HEAD', returnStdout: true).trim()
                    env.BUILD_DATE = sh(script: 'date +"%a %b %d, %Y at %H:%M:%S %Z"', returnStdout: true).trim()
                    env.LDFLAGS = "-s -w -X 'main.Version=${env.VERSION}' -X 'main.CommitID=${env.COMMIT_ID}' -X 'main.BuildDate=${env.BUILD_DATE}'"
                }
                sh 'mkdir -p ${BINDIR} ${RELDIR}'
            }
        }

        stage('Build') {
            parallel {
                stage('Linux AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                -v ${WORKSPACE}:/build \
                                -v ${GOCACHE}:/root/.cache/go-build \
                                -v ${GOMODCACHE}:/go/pkg/mod \
                                -w /build \
                                -e CGO_ENABLED=0 \
                                -e GOOS=linux \
                                -e GOARCH=amd64 \
                                golang:alpine \
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT}-linux-amd64 ./src
                        '''
                    }
                }
                stage('Linux ARM64') {
                    agent { label 'arm64' }
                    steps {
                        sh '''
                            docker run --rm \
                                -v ${WORKSPACE}:/build \
                                -v ${GOCACHE}:/root/.cache/go-build \
                                -v ${GOMODCACHE}:/go/pkg/mod \
                                -w /build \
                                -e CGO_ENABLED=0 \
                                -e GOOS=linux \
                                -e GOARCH=arm64 \
                                golang:alpine \
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT}-linux-arm64 ./src
                        '''
                    }
                }
            }
        }

        stage('Test') {
            agent { label 'amd64' }
            steps {
                sh '''
                    docker run --rm \
                        -v ${WORKSPACE}:/build \
                        -v ${GOCACHE}:/root/.cache/go-build \
                        -v ${GOMODCACHE}:/go/pkg/mod \
                        -w /build \
                        golang:alpine \
                        go test -v -cover ./...
                '''
            }
        }

        stage('Release') {
            agent { label 'amd64' }
            when {
                buildingTag()
            }
            steps {
                sh '''
                    # Create version.txt
                    echo "${VERSION}" > ${RELDIR}/version.txt

                    # Copy and strip binaries
                    for f in ${BINDIR}/${PROJECT}-*; do
                        [ -f "$f" ] || continue
                        strip "$f" 2>/dev/null || true
                        cp "$f" ${RELDIR}/
                    done

                    # Create source archive
                    tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
                        --exclude='binaries' --exclude='releases' --exclude='*.tar.gz' \
                        -czf ${RELDIR}/${PROJECT}-${VERSION}-source.tar.gz .
                '''
                archiveArtifacts artifacts: 'releases/*', fingerprint: true
            }
        }

        stage('Docker') {
            agent { label 'amd64' }
            when {
                anyOf {
                    branch 'main'
                    branch 'master'
                    buildingTag()
                }
            }
            steps {
                sh '''
                    docker buildx create --name ${PROJECT}-builder --use 2>/dev/null || docker buildx use ${PROJECT}-builder
                    docker buildx build \
                        -f ./docker/Dockerfile \
                        --platform linux/amd64,linux/arm64 \
                        --build-arg VERSION="${VERSION}" \
                        --build-arg VCS_REF="${COMMIT_ID}" \
                        --build-arg BUILD_DATE="${BUILD_DATE}" \
                        -t ${REGISTRY}:${VERSION} \
                        -t ${REGISTRY}:latest \
                        --push \
                        .
                '''
            }
        }
    }

    post {
        always {
            cleanWs()
        }
    }
}
```

### Jenkins Requirements

| Requirement | Description |
|-------------|-------------|
| Agent labels | `amd64` and `arm64` MUST be available |
| Docker | Required on all agents (builds use golang:alpine) |
| Docker buildx | Required on amd64 agent for multi-arch container builds |
| Registry auth | ghcr.io credentials configured in Jenkins |
| Go module cache | `/tmp/go-cache` and `/tmp/go-mod-cache` for caching |

---

# CHECKPOINT 6: BUILD & DEPLOYMENT VERIFICATION

Before proceeding, confirm you understand:
- [ ] Docker uses tini as init, Alpine base
- [ ] Makefile has exactly 4 targets: build, release, docker, test
- [ ] Binary naming: NEVER include -musl suffix
- [ ] 4 GitHub/Gitea workflows: release, beta, daily, docker
- [ ] Jenkins builds for ARM64 and AMD64
- [ ] All 8 platform builds required (4 OS x 2 arch)

---

# PART 22: BINARY REQUIREMENTS (NON-NEGOTIABLE)

## Single Static Binary

| Requirement | Description |
|-------------|-------------|
| Type | **SINGLE STATIC BINARY** |
| Assets | Embedded using Go's `embed` package |
| Dependencies | None at runtime |
| Build | **CGO_ENABLED=0** |
| Libraries | Pure Go only (no CGO) |

## Default Behavior

| Behavior | Description |
|----------|-------------|
| No arguments | Initialize (if needed) and start server |
| First run | Auto-create config with defaults |
| First run | Auto-create required directories |
| Signals | Proper handling (SIGTERM, SIGINT, SIGHUP) |
| PID file | Enabled by default |

## Embedded Assets

| Asset Type | Location |
|------------|----------|
| Templates | `src/server/templates/` |
| Static files | `src/server/static/` |
| Application data | `src/data/` (JSON files) |

## External Data (NOT Embedded)

| Data Type | Description |
|-----------|-------------|
| GeoIP databases | Download, update via scheduler |
| Blocklists | Download, update via scheduler |
| Security databases | Any security-related data |

---

# PART 23: TESTING & DEVELOPMENT (NON-NEGOTIABLE)

## NEVER Use Project Directory for Testing (NON-NEGOTIABLE)

**ALL testing, debugging, and runtime data MUST use temp directories. NEVER the project directory.**

| FORBIDDEN | REASON |
|-----------|--------|
| `./data/` | Project directory - will pollute repo |
| `./config/` | Project directory - will pollute repo |
| `./test-data/` | Project directory - will pollute repo |
| `./logs/` | Project directory - will pollute repo |
| `{project_path}/anything` | NEVER write test data to project |

**The project directory is for SOURCE CODE ONLY. All runtime/test data goes to `/tmp/`.**

## NEVER Create Example Files (NON-NEGOTIABLE)

**Do NOT create example/sample configuration files in the repository.**

| FORBIDDEN | REASON |
|-----------|--------|
| `server.example.yml` | Unnecessary - defaults are in binary |
| `server.sample.yml` | Unnecessary - defaults are in binary |
| `.env` | We don't use .env files |
| `.env.example` | We don't use .env files |
| `.env.sample` | We don't use .env files |
| `.env.local` | We don't use .env files |
| `config.example.json` | Unnecessary |
| `*.example.*` | No example files of any kind |
| `*.sample.*` | No sample files of any kind |

**If docker-compose.yml needs env vars → hardcode with sane defaults. NEVER use .env files.**

**Why no example files?**

| Reason | Description |
|--------|-------------|
| **Self-documenting** | Binary generates default config on first run |
| **Always current** | Embedded defaults are always in sync with code |
| **No maintenance** | Example files become outdated and misleading |
| **Cleaner repo** | Less clutter in repository |

**How users get configuration:**

1. Run binary - it auto-generates `server.yml` with defaults on first run
2. Admin panel shows all settings with descriptions
3. Documentation in AI.md describes all options

## Temporary Directory Structure (NON-NEGOTIABLE)

**Format:** `/tmp/{orgname}-{purpose}/airports/`

| Purpose | Path | Example |
|---------|------|---------|
| Build output | `/tmp/apimgr-build/airports/` | `/tmp/apimgr-build/jokes/` |
| Test config | `/tmp/apimgr-test/airports/config/` | `/tmp/apimgr-test/jokes/config/` |
| Test data | `/tmp/apimgr-test/airports/data/` | `/tmp/apimgr-test/jokes/data/` |
| Test logs | `/tmp/apimgr-test/airports/logs/` | `/tmp/apimgr-test/jokes/logs/` |
| Debug files | `/tmp/apimgr-debug/airports/` | `/tmp/apimgr-debug/jokes/` |
| Runtime temp | `/tmp/apimgr-runtime/airports/` | `/tmp/apimgr-runtime/jokes/` |

### Rules

| Rule | Description |
|------|-------------|
| **NEVER** | Use project directory for test/runtime data |
| **NEVER** | Use `/tmp/airports` directly |
| **NEVER** | Use `/tmp/` root for project files |
| **ALWAYS** | Use `/tmp/{orgname}-{purpose}/airports/` format |
| **ALWAYS** | Include organization prefix to avoid conflicts |
| **ALWAYS** | Include purpose subdirectory for organization |

### Why This Structure?

| Reason | Description |
|--------|-------------|
| **Avoid conflicts** | Multiple projects/orgs won't collide |
| **Easy cleanup** | `rm -rf /tmp/apimgr-*` cleans all org temp files |
| **Clear purpose** | Directory name shows what files are for |
| **Debugging** | Easy to find specific project's temp files |

### Correct vs Incorrect

| WRONG | RIGHT |
|-------|-------|
| `/tmp/jokes` | `/tmp/apimgr-build/jokes/` |
| `/tmp/build/jokes` | `/tmp/apimgr-build/jokes/` |
| `/tmp/test` | `/tmp/apimgr-test/jokes/` |
| `/tmp/jokes-test` | `/tmp/apimgr-test/jokes/` |

## Container Usage (NON-NEGOTIABLE)

**ALL builds, tests, and debugging MUST use Docker containers. NEVER build directly on the host system.**

| Rule | Description |
|------|-------------|
| Build environment | Docker (preferred), Incus, or LXD |
| Go Image | **`golang:alpine`** (Alpine-based, latest) |
| Runtime Image | **`alpine:latest`** |
| Test binaries | Temp directories only |
| **NEVER** | Run `go build` directly on host |
| **NEVER** | Install Go on host system |
| **ALWAYS** | Use containerized build command below |

### Why Containerized Builds?

| Reason | Description |
|--------|-------------|
| **Clean system** | No Go installation, no build artifacts on host |
| **Cross-platform** | Works on any distro/platform with Docker |
| **Consistent environment** | Same Go version and dependencies across all builds |
| **Static binaries** | `CGO_ENABLED=0` produces binaries with no runtime dependencies |
| **Reproducible** | Builds work regardless of host OS setup |
| **No pollution** | Host system stays clean |

### Container Images

| Purpose | Image | Why |
|---------|-------|-----|
| Building Go | `golang:alpine` | Latest Go on Alpine, small image |
| Runtime | `alpine:latest` | Minimal runtime with curl, bash, tini |
| Testing | `golang:alpine` | Same as build for consistency |

## Build Command (NON-NEGOTIABLE)

**This is the REQUIRED build command. Not an example - this MUST be used.**

```bash
docker run --rm -v /path/to/project:/build -w /build -e CGO_ENABLED=0 \
  golang:alpine go build -o /tmp/apimgr-build/airports/airports ./src
```

### Testing a Built Binary

```bash
# Build
docker run --rm -v /root/Projects/github/apimgr/airports:/build -w /build \
  -e CGO_ENABLED=0 golang:alpine go build -o /tmp/apimgr-build/airports/airports ./src

# Basic tests (no config needed)
/tmp/apimgr-build/airports/airports --help
/tmp/apimgr-build/airports/airports --version

# Full test with config/data (ALWAYS use temp directories)
mkdir -p /tmp/apimgr-test/airports/{config,data,logs}

# Run with temp directories (use correct CLI flags)
/tmp/apimgr-build/airports/airports \
  --config /tmp/apimgr-test/airports/config \
  --data /tmp/apimgr-test/airports/data

# Cleanup after testing
rm -rf /tmp/apimgr-test/airports
rm -rf /tmp/apimgr-build/airports
```

### Test Directory Setup

**ALWAYS create temp directories before running tests:**

```bash
# Create all test directories
TEST_BASE="/tmp/apimgr-test/airports"
mkdir -p "$TEST_BASE"/{config,data/db,data/tor,data/geoip,logs}

# Run binary with test directories (use correct CLI flags)
/tmp/apimgr-build/airports/airports \
  --config "$TEST_BASE/config" \
  --data "$TEST_BASE/data"
```

**NEVER run the binary without specifying temp directories - it may default to project directory!**

## Process Management (NON-NEGOTIABLE)

**STRICT RULES: Only kill/remove the EXACT process or container being worked on. NEVER anything else.**

### FORBIDDEN Commands (NEVER Use)

| Command | Reason |
|---------|--------|
| `pkill -f {pattern}` | Too broad, kills unrelated processes |
| `pkill {name}` | Too broad without `-x` flag |
| `killall {name}` | Too broad, may kill unrelated processes |
| `kill -9 {pid}` | Use graceful `kill {pid}` first |
| `docker kill` | Use `docker stop` for graceful shutdown |
| `docker rm $(docker ps -aq)` | Removes ALL containers |
| `docker rm $(docker ps -q)` | Removes ALL running containers |
| `docker rmi $(docker images -q)` | Removes ALL images |
| `docker system prune` | Cleans ALL unused resources |
| `docker container prune` | Removes ALL stopped containers |
| `docker image prune` | Removes ALL dangling images |
| `docker volume prune` | Removes ALL unused volumes |
| `docker network prune` | Removes ALL unused networks |
| `rm -rf /` | Catastrophic |
| `rm -rf /*` | Catastrophic |
| `rm -rf ~` | Destroys home directory |
| `rm -rf .` | Dangerous in wrong directory |
| `rm -rf *` | Dangerous without proper scoping |

### Process Termination Rules

| Rule | Description |
|------|-------------|
| **Identify first** | ALWAYS get exact PID before killing |
| **Graceful first** | Use `kill {pid}` (SIGTERM), wait, then `kill -9 {pid}` only if needed |
| **One at a time** | Kill ONE specific PID, never patterns |
| **Verify PID** | Confirm PID belongs to the project process |
| **Document** | Log what was killed and why |

**Kill Process Flow:**
```
1. pgrep -la airports           # List matching processes
2. Verify the PID is correct          # CHECK before killing
3. kill {pid}                         # Graceful termination (SIGTERM)
4. sleep 5                            # Wait for graceful shutdown
5. pgrep -la airports           # Check if still running
6. kill -9 {pid}                      # Force kill ONLY if still running
```

### Docker Container Rules

| Rule | Description |
|------|-------------|
| **ONLY this project** | Only stop/remove containers named `airports` |
| **NEVER other containers** | Even if they look related or unused |
| **NEVER images not ours** | Only remove `apimgr/airports:*` images |
| **NEVER base images** | Never remove `golang`, `alpine`, `ubuntu`, etc. |
| **NEVER volumes** | Unless explicitly part of this project |

**Docker Cleanup Flow:**
```
1. docker ps -a --filter name=airports     # List ONLY this project's containers
2. Verify output shows ONLY airports       # CHECK before removing
3. docker stop airports                    # Stop gracefully
4. docker rm airports                      # Remove container

# For images:
1. docker images apimgr/airports     # List ONLY this project's images
2. Verify output shows ONLY our images          # CHECK before removing
3. docker rmi apimgr/airports:tag    # Remove SPECIFIC tag
```

### Allowed Commands (Project-Scoped ONLY)

| Command | Description |
|---------|-------------|
| `kill {specific-pid}` | Kill exact PID only (after verification) |
| `pkill -x airports` | Exact binary name match only |
| `docker stop airports` | Stop specific container by name |
| `docker rm airports` | Remove specific container by name |
| `docker rmi apimgr/airports:tag` | Remove specific image:tag |
| `rm -rf /tmp/apimgr-build/airports/` | Remove specific project temp files |
| `rm -rf /tmp/apimgr-*/airports/` | Remove all temp files for one project |

### Before ANY Kill/Remove Operation

1. **List first**: See exactly what will be affected
2. **Verify**: Confirm it's the correct process/container/file
3. **Be specific**: Use exact names, PIDs, or paths - NEVER patterns
4. **Ask if unsure**: When in doubt, ask the user
5. **Document**: Log what was removed and why

## File Cleanup Rules (NON-NEGOTIABLE)

**Always be explicit and project-scoped when deleting files.**

### Safe Cleanup Commands

| Purpose | Command |
|---------|---------|
| Project build temp | `rm -rf /tmp/apimgr-build/airports/` |
| Project test temp | `rm -rf /tmp/apimgr-test/airports/` |
| Project debug temp | `rm -rf /tmp/apimgr-debug/airports/` |
| All project temp | `rm -rf /tmp/apimgr-*/airports/` |
| All org temp | `rm -rf /tmp/apimgr-*/` |
| Project binaries | `rm -rf ./binaries/airports*` |
| Project releases | `rm -rf ./releases/airports*` |

### NEVER Delete Without Confirmation

| Item | Why |
|------|-----|
| User data directories | Irreversible data loss |
| Config files | User customizations lost |
| Database files | Data loss |
| SSL certificates | Service disruption |
| Git repositories | Code loss |
| Anything outside project scope | Affects other systems |

### Cleanup Checklist

Before running any `rm -rf`:

1. **Echo first**: `echo "Would delete: /path/to/delete"` - verify the path
2. **Check pwd**: `pwd` - make sure you're in the right directory
3. **List first**: `ls -la /path/to/delete` - see what will be deleted
4. **Be specific**: Use full paths, not relative paths with wildcards
5. **Ask if unsure**: When in doubt, ask the user before deleting

---

# PART 24: DATABASE & CLUSTER (NON-NEGOTIABLE)

## Database Migrations

**ALL apps MUST have built-in AUTOMATIC database migration support.**

| Feature | Description |
|---------|-------------|
| Automatic | Runs on startup |
| Versioned | Migrations with timestamps |
| Tracking | `schema_migrations` table |
| Rollback | Automatic on failure |

## Cluster Support

**ALL apps MUST support cluster mode.**

### Single Instance (Auto-detected)

- No external cache/database configured
- Uses local file/SQLite for state

### Cluster Mode (Auto-detected)

- Auto-enabled when external cache or shared database detected
- Primary election for cluster-wide tasks
- Distributed locks
- Session sharing

---

# PART 25: SECURITY & LOGGING (NON-NEGOTIABLE)

## Security Headers

**All responses MUST include:**

```
X-Content-Type-Options: nosniff
X-Frame-Options: SAMEORIGIN
X-XSS-Protection: 1; mode=block
Referrer-Policy: strict-origin-when-cross-origin
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'
Permissions-Policy: geolocation=(), microphone=(), camera=()
```

**In development mode, these may be relaxed.**

## Well-Known Files (NON-NEGOTIABLE)

**Standard files served at well-known paths. Generated automatically if no file exists.**

### Required Files

| File | Path | Purpose |
|------|------|---------|
| `robots.txt` | `/robots.txt` | Search engine crawling rules |
| `security.txt` | `/.well-known/security.txt` | Security vulnerability reporting (RFC 9116) |

### Additional Well-Known Paths

| Path | Purpose |
|------|---------|
| `/.well-known/acme-challenge/` | Let's Encrypt HTTP-01 challenge |
| `/.well-known/change-password` | Password change URL (redirects to `/auth/forgot`) |

### Well-Known Directory Support

Files can be served from:
1. Files in `{data_dir}/web/.well-known/` (checked first)
2. Embedded files in binary
3. Dynamically generated (e.g., ACME challenges, config-based security.txt)

### robots.txt

```
# Served at /robots.txt - generated if no file exists

User-agent: *
Allow: /
Allow: /api
Disallow: /admin
Sitemap: {app_url}/sitemap.xml
```

**Configuration:**
```yaml
web:
  robots:
    allow:
      - /
      - /api
    deny:
      - /admin
```

### security.txt (RFC 9116)

**ALL projects MUST serve a valid security.txt file.**

```
# Served at /.well-known/security.txt

Contact: mailto:{security_contact}
Expires: {expiry_date}
```

**Configuration:**
```yaml
web:
  security:
    contact: "security@{fqdn}"    # Security contact email
    expires: "{1year}"            # Auto-calculated 1 year from generation
```

**Fields:**
| Field | Required | Description |
|-------|----------|-------------|
| `Contact` | YES | Email for reporting vulnerabilities (mailto: prefix added automatically) |
| `Expires` | YES | Expiration date (auto-renewed yearly by default) |

### Admin Panel (/admin/web)

**robots.txt Settings:**

| Element | Type | Description |
|---------|------|-------------|
| Allow paths | Tag input / List | Paths to allow crawling (e.g., `/`, `/api`) |
| Deny paths | Tag input / List | Paths to deny crawling (e.g., `/admin`) |
| Preview | Read-only textarea | Shows generated robots.txt content |

**security.txt Settings:**

| Element | Type | Description |
|---------|------|-------------|
| Security contact | Text input | Email for vulnerability reports |
| Expires | Date picker | Expiration date (default: 1 year from now, auto-renews) |
| Preview | Read-only textarea | Shows generated security.txt content |

## Logging

### Log Files

| Log | Purpose | Default Format | Available Formats |
|-----|---------|----------------|-------------------|
| `access.log` | HTTP requests | `apache` | `apache`, `nginx`, `json` |
| `server.log` | Application events | `text` | `text`, `json` |
| `error.log` | Error messages | `text` | `text`, `json` |
| `audit.log` | Security events | `json` | `json`, `text` |
| `security.log` | Security/auth events | `fail2ban` | `fail2ban`, `syslog`, `cef`, `json`, `text` |
| `debug.log` | Debug (dev mode) | `text` | `text`, `json` |

### Log Format Details

**Access Log Formats:**
| Format | Description | Example |
|--------|-------------|---------|
| `apache` | Apache Combined Log Format (default) | `127.0.0.1 - - [10/Oct/2024:13:55:36 -0700] "GET /api/v1/health HTTP/1.1" 200 2326 "-" "curl/7.64.1"` |
| `nginx` | Nginx Common Log Format | `127.0.0.1 - - [10/Oct/2024:13:55:36 -0700] "GET /api/v1/health HTTP/1.1" 200 2326` |
| `json` | Structured JSON | `{"ip":"127.0.0.1","time":"2024-10-10T13:55:36Z","method":"GET","path":"/api/v1/health","status":200,"size":2326,"ua":"curl/7.64.1"}` |

**Security Log Formats:**
| Format | Description | Use Case |
|--------|-------------|----------|
| `fail2ban` | Fail2ban compatible (default) | Intrusion prevention integration |
| `syslog` | RFC 5424 syslog format | SIEM integration, centralized logging |
| `cef` | Common Event Format | SIEM/security tools (ArcSight, Splunk) |
| `json` | Structured JSON | Custom parsing, ELK stack |
| `text` | Plain text | Human readable |

**Text Log Format:**
```
2024-10-10 13:55:36 [INFO] Server started on :8080
2024-10-10 13:55:40 [ERROR] Database connection failed: timeout
```

**JSON Log Format:**
```json
{"time":"2024-10-10T13:55:36Z","level":"INFO","msg":"Server started on :8080"}
{"time":"2024-10-10T13:55:40Z","level":"ERROR","msg":"Database connection failed","error":"timeout"}
```

**Fail2ban Format:**
```
2024-10-10 13:55:36 [security] Failed login attempt from 192.168.1.100 for user admin
2024-10-10 13:55:40 [security] Rate limit exceeded from 192.168.1.100
```

### Custom Format Variables

When using `format: custom`, these variables are available:

| Variable | Description |
|----------|-------------|
| `{time}` | Time only |
| `{date}` | Date only |
| `{datetime}` | Date and time |
| `{remote_ip}` | Client IP address |
| `{method}` | HTTP method |
| `{path}` | Request path |
| `{query}` | Query string |
| `{status}` | HTTP status code |
| `{bytes}` | Response size |
| `{latency}` | Request latency (human readable) |
| `{latency_ms}` | Request latency (milliseconds) |
| `{user_agent}` | User agent string |
| `{referer}` | Referer header |
| `{request_id}` | Request ID |
| `{host}` | Request host |
| `{protocol}` | HTTP protocol version |
| `{tls_version}` | TLS version (if HTTPS) |
| `{country}` | GeoIP country code |
| `{asn}` | GeoIP ASN |

### Rotation Options

| Option | Description |
|--------|-------------|
| `never` | Never rotate |
| `daily` | Rotate daily |
| `weekly` | Rotate weekly |
| `monthly` | Rotate monthly |
| `yearly` | Rotate yearly |
| `NMB` | Rotate at N megabytes (e.g., `50MB`) |
| `NGB` | Rotate at N gigabytes (e.g., `1GB`) |
| Combined | Time + size, whichever first (e.g., `weekly,50MB`) |

### Retention Options

| Option | Description |
|--------|-------------|
| `none` | Do not keep old logs (delete after rotation) |
| `N` | Keep N old log files |
| `Nd` | Keep logs for N days |
| `Nw` | Keep logs for N weeks |
| `Nm` | Keep logs for N months |

### Configuration

```yaml
server:
  logs:
    # Global log level: debug, info, warn, error
    level: warn

    # All log types share these options:
    #   filename: name of log file
    #   format: output format (varies by log type)
    #   custom: custom format string (when format=custom)
    #   rotate: rotation policy
    #   keep: retention policy

    access:
      filename: access.log
      # Format: apache, nginx, json, custom
      format: apache
      custom: ""
      rotate: monthly
      keep: none

    server:
      filename: server.log
      # Format: text, json
      format: text
      custom: ""
      rotate: weekly,50MB
      keep: none

    error:
      filename: error.log
      # Format: text, json
      format: text
      custom: ""
      rotate: weekly,50MB
      keep: none

    audit:
      filename: audit.log
      # Format: json, text
      format: json
      custom: ""
      rotate: weekly,50MB
      keep: none

    security:
      filename: security.log
      # Format: fail2ban, syslog, cef, json, text
      format: fail2ban
      custom: ""
      rotate: weekly,50MB
      keep: none

    debug:
      # Debug log has an enabled flag since it's for troubleshooting only
      enabled: false
      filename: debug.log
      # Format: text, json
      format: text
      custom: ""
      rotate: weekly,50MB
      keep: none
```

### Log Output Rules (NON-NEGOTIABLE)

**All log FILES MUST use raw text only:**
- NO emojis
- NO ANSI color codes
- NO special characters or formatting
- Plain ASCII text only
- Machine-parseable format

**Console output (stdout/stderr) CAN be pretty:**
- Emojis allowed (e.g., `✅ Server started`, `❌ Error`, `⚠️ Warning`)
- ANSI colors allowed
- Pretty formatting allowed
- Used for start/stop/restart/status messages
- User-facing CLI output can be visually appealing

**Rule:** Log files = raw/plain text. Console = pretty is OK.

### Log Rotation

**Defaults:**
| Log Type | Rotation | Keep |
|----------|----------|------|
| access.log | monthly | none |
| All others | weekly,50MB | none |

**Rules:**
- `weekly,50MB` = rotate on weekly OR 50MB, whichever comes first
- `keep: none` = do not retain old logs (default)
- Built-in rotation support (no external logrotate needed)
- Old logs deleted immediately after rotation (default)
- Optional: compress before delete, retain with `keep: weekly:N` or `monthly:N`

### Audit Log (NON-NEGOTIABLE)

**The audit log records ALL security-relevant events and administrative actions. It is the authoritative record of who did what and when.**

## Audit Log Purpose

| Purpose | Description |
|---------|-------------|
| **Accountability** | Track all admin and user actions |
| **Security** | Detect unauthorized access attempts |
| **Compliance** | Meet regulatory requirements (GDPR, SOC2, etc.) |
| **Debugging** | Investigate issues and incidents |
| **Forensics** | Post-incident analysis |

## Audit Log Events

### Server Admin Events

| Event | Description | Logged Data |
|-------|-------------|-------------|
| `admin.login` | Admin logged in | IP, user agent, MFA used, admin username |
| `admin.logout` | Admin logged out | Admin username, session duration |
| `admin.login_failed` | Failed login attempt | IP, user agent, reason, attempted username |
| `admin.created` | New admin account created | New admin username, created by (admin username) |
| `admin.deleted` | Admin account removed | Deleted admin username, deleted by (admin username) |
| `admin.password_changed` | Admin changed password | Admin username, IP (NEVER log password) |
| `admin.mfa_enabled` | Admin enabled 2FA | Admin username, method (TOTP, WebAuthn) |
| `admin.mfa_disabled` | Admin disabled 2FA | Admin username, method |
| `admin.token_regenerated` | Admin API token regenerated | Admin username, IP |
| `admin.session_expired` | Admin session timed out | Admin username, session ID |
| `admin.session_revoked` | Admin session manually ended | Admin username, revoked by |

### User Events (Multi-User Mode)

| Event | Description | Logged Data |
|-------|-------------|-------------|
| `user.registered` | New user registered | User ID, IP, registration method (form, OIDC, invite) |
| `user.login` | User logged in | User ID, IP, user agent, auth method |
| `user.logout` | User logged out | User ID, session duration |
| `user.login_failed` | Failed login attempt | IP, user agent, reason (NOT username/email) |
| `user.created` | Admin created user | User ID, created by (admin username) |
| `user.deleted` | User account deleted | User ID, deleted by (admin/self), reason |
| `user.suspended` | User account suspended | User ID, suspended by, reason |
| `user.unsuspended` | User account reactivated | User ID, unsuspended by |
| `user.role_changed` | User role modified | User ID, old role, new role, changed by |
| `user.password_changed` | User changed password | User ID, IP, method (direct, reset link) |
| `user.password_reset_requested` | Password reset requested | IP (NOT email/username) |
| `user.password_reset_completed` | Password reset completed | User ID, IP |
| `user.email_verified` | Email address verified | User ID, email (masked) |
| `user.mfa_enabled` | User enabled 2FA | User ID, method |
| `user.mfa_disabled` | User disabled 2FA | User ID, method, disabled by (self/admin) |
| `user.recovery_key_used` | Recovery key consumed | User ID, keys remaining |

### OIDC/LDAP Events

| Event | Description | Logged Data |
|-------|-------------|-------------|
| `oidc.login` | User logged in via OIDC | User ID, provider name, IP |
| `oidc.login_failed` | OIDC login failed | Provider name, IP, reason |
| `oidc.user_created` | Auto-provisioned user via OIDC | User ID, provider name |
| `oidc.admin_granted` | Admin access via group mapping | User ID, provider name, group name |
| `oidc.admin_revoked` | Admin access removed (group change) | User ID, provider name |
| `ldap.login` | User logged in via LDAP | User ID, IP |
| `ldap.login_failed` | LDAP login failed | IP, reason |
| `ldap.admin_granted` | Admin access via group mapping | User ID, group DN |
| `ldap.admin_revoked` | Admin access removed (group change) | User ID |

### Configuration Events

| Event | Description | Logged Data |
|-------|-------------|-------------|
| `config.updated` | Configuration changed | Changed keys (NOT sensitive values), changed by |
| `config.smtp_updated` | SMTP settings changed | Changed by (NOT credentials) |
| `config.ssl_updated` | SSL certificate changed | Subject, expiry, changed by |
| `config.ssl_expired` | SSL certificate expired | Domain |
| `config.tor_enabled` | Tor hidden service enabled | Changed by |
| `config.tor_disabled` | Tor hidden service disabled | Changed by |
| `config.tor_address_regenerated` | Onion address regenerated | Changed by |
| `config.branding_updated` | Branding settings changed | Changed by |
| `config.oidc_provider_added` | OIDC provider configured | Provider name, added by |
| `config.oidc_provider_removed` | OIDC provider removed | Provider name, removed by |
| `config.ldap_updated` | LDAP settings changed | Changed by |
| `config.admin_groups_updated` | Admin group mapping changed | Old groups, new groups, changed by |

### Security Events

| Event | Description | Logged Data |
|-------|-------------|-------------|
| `security.rate_limit_exceeded` | Rate limit hit | IP, endpoint, limit |
| `security.ip_blocked` | IP address blocked | IP, reason, duration |
| `security.ip_unblocked` | IP address unblocked | IP, unblocked by |
| `security.country_blocked` | Request blocked by GeoIP | IP, country code |
| `security.csrf_failure` | CSRF token validation failed | IP, endpoint |
| `security.invalid_token` | Invalid API token used | Token type, IP |
| `security.brute_force_detected` | Brute force attempt detected | IP, target (masked), attempt count |
| `security.suspicious_activity` | Unusual activity detected | IP, activity type, details |

### Token Events

| Event | Description | Logged Data |
|-------|-------------|-------------|
| `token.created` | API token created | Token ID (partial), permissions, expiry, created by |
| `token.revoked` | API token revoked | Token ID (partial), revoked by |
| `token.expired` | API token expired | Token ID (partial) |
| `token.used` | API token used (optional, high volume) | Token ID (partial), endpoint, IP |

### Backup & System Events

| Event | Description | Logged Data |
|-------|-------------|-------------|
| `backup.created` | Backup created | Filename, size, created by |
| `backup.restored` | Backup restored | Filename, restored by |
| `backup.deleted` | Backup deleted | Filename, deleted by |
| `backup.failed` | Backup failed | Error message |
| `system.started` | Application started | Version, mode, node ID |
| `system.stopped` | Application stopped | Reason, uptime |
| `system.maintenance_entered` | Maintenance mode enabled | Reason, enabled by |
| `system.maintenance_exited` | Maintenance mode disabled | Duration, disabled by |
| `system.updated` | Application updated | Old version, new version |
| `scheduler.task_failed` | Scheduled task failed | Task name, error |
| `scheduler.task_manual_run` | Task manually triggered | Task name, triggered by |

### Cluster Events

| Event | Description | Logged Data |
|-------|-------------|-------------|
| `cluster.node_joined` | Node joined cluster | Node ID, IP |
| `cluster.node_removed` | Node removed from cluster | Node ID, removed by |
| `cluster.node_failed` | Node became unreachable | Node ID, last seen |
| `cluster.token_generated` | Join token generated | Token ID (partial), generated by |
| `cluster.mode_changed` | Cluster mode changed | Old mode, new mode, changed by |

## Audit Log Format

**All audit logs are JSON format, one entry per line (JSON Lines).**

```json
{
  "id": "audit_01HQXYZ123ABC",
  "time": "2025-01-15T10:30:00.123Z",
  "event": "admin.login",
  "category": "authentication",
  "severity": "info",
  "actor": {
    "type": "admin",
    "id": "administrator",
    "ip": "192.168.1.100",
    "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)..."
  },
  "target": {
    "type": "session",
    "id": "sess_abc123"
  },
  "details": {
    "mfa_used": true,
    "mfa_method": "totp"
  },
  "result": "success",
  "node_id": "node-1"
}
```

**Required Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | String | Unique audit entry ID (ULID format) |
| `time` | String | ISO 8601 timestamp with milliseconds, UTC |
| `event` | String | Event type (e.g., `admin.login`) |
| `category` | String | Event category (e.g., `authentication`) |
| `severity` | String | `info`, `warn`, `error`, `critical` |
| `actor` | Object | Who performed the action |
| `result` | String | `success` or `failure` |

**Optional Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `target` | Object | What was acted upon |
| `details` | Object | Event-specific details |
| `node_id` | String | Node ID (cluster mode) |
| `reason` | String | Reason for action (if provided) |

## Severity Levels

| Severity | Use For | Examples |
|----------|---------|----------|
| `info` | Successful normal operations | Login, config save, backup complete |
| `warn` | Failed attempts, recoverable issues | Failed login, rate limit hit |
| `error` | Failures requiring attention | Backup failed, scheduler error |
| `critical` | Security incidents, system failures | Brute force detected, maintenance mode |

## Audit Log Configuration

```yaml
server:
  logging:
    audit:
      enabled: true
      file: "{log_dir}/audit.log"
      format: "json"           # json only (text not supported for audit)

      # Rotation
      rotate: "daily"          # daily, size, none
      max_size: "100MB"        # Max size before rotation (if rotate: size)
      keep: 0                  # Days to retain (0 = delete on rotation, default)
      compress: false          # Compress rotated logs (only if keep > 0)

      # What to log
      events:
        authentication: true   # Login/logout events
        configuration: true    # Config changes
        security: true         # Security events
        tokens: true           # Token create/revoke
        users: true            # User management
        backup: true           # Backup/restore
        system: true           # System events
        cluster: true          # Cluster events
        token_usage: false     # Individual token uses (high volume)

      # Sensitive data handling
      mask_emails: true        # Show j***n@e***.com instead of full
      mask_usernames: false    # Show full usernames in logs
      include_user_agent: true # Include browser/client info
```

## Sane Defaults

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | `true` | Audit logging enabled |
| `format` | `json` | JSON format (required) |
| `rotate` | `daily` | Rotate daily |
| `keep` | `0` | Delete on rotation (no old logs kept) |
| `compress` | `false` | No compression (deleted immediately) |
| `mask_emails` | `true` | Mask email addresses |
| All event categories | `true` | Log all events |
| `token_usage` | `false` | Don't log every token use |

**Why `keep: 0` by default?**
- Reduces disk usage
- Minimizes data retention liability
- Users who need log history can configure retention
- Current log always available until rotation

## Audit Log Rules (NON-NEGOTIABLE)

**NEVER Log:**
- ❌ Passwords (plain, hashed, or encrypted)
- ❌ API tokens or secrets (full value)
- ❌ Session tokens (full value)
- ❌ Recovery keys
- ❌ TOTP secrets
- ❌ Private keys
- ❌ Credit card numbers
- ❌ Full email addresses (mask them)

**ALWAYS Log:**
- ✓ Timestamp in UTC with milliseconds
- ✓ IP address for all events
- ✓ Actor identity (who did it)
- ✓ Event result (success/failure)
- ✓ Unique event ID

**Token/ID Masking:**
- Show only first 8 characters: `token_abc12345...`
- Or use separate ID field that doesn't expose token value

## Admin Panel (`/admin/server/logs/audit`)

| Element | Type | Description |
|---------|------|-------------|
| Log viewer | Table | Paginated audit log entries |
| Filters | Dropdowns | Filter by category, severity, date range |
| Search | Text input | Search by actor, IP, event type |
| Export | Button | Download filtered results as JSON/CSV |
| Retention | Display | Show current retention policy |
| Stats | Cards | Event counts by category/severity |

**Log Viewer Columns:**
| Column | Description |
|--------|-------------|
| Time | Timestamp (local timezone) |
| Event | Event type with icon |
| Actor | Who performed action |
| Target | What was affected |
| IP | Source IP address |
| Result | Success/failure badge |
| Details | Expandable row |

**Filters:**
- Category: All, Authentication, Configuration, Security, etc.
- Severity: All, Info, Warn, Error, Critical
- Result: All, Success, Failure
- Date range: Today, Last 7 days, Last 30 days, Custom
- Actor: Text search
- IP: Text search

**Export Options:**
- Format: JSON (default), CSV
- Range: Current view, All matching filters, Full log
- Note: Export respects same masking rules as display

## Audit Log Integrity

**The audit log is append-only and tamper-evident.**

| Protection | Description |
|------------|-------------|
| Append-only | Application can only append, never modify or delete entries |
| No truncate | Application cannot truncate the log file |
| Rotation only | Only log rotation can remove old entries |
| Checksum | Optional: Include running checksum for tamper detection |

**File Permissions:**
```
audit.log: 0640 (rw-r-----)
Owner: application user
Group: audit group (if configured)
```

## Audit Log Retention

| Retention | Description |
|-----------|-------------|
| `keep: 0` | Delete on rotation (default) - no old logs kept |
| `keep: 30` | Keep 30 days |
| `keep: 90` | Keep 90 days |
| `keep: 365` | Keep 1 year |
| `keep: -1` | Keep forever (no automatic deletion) |

**Rotation Schedule:**
- Daily rotation at midnight UTC
- If `keep: 0`: Old log deleted immediately after rotation
- If `keep: N`: Rotated files named `audit.log.2025-01-15.gz`, deleted after N days
- Current day's log always available until next rotation

---

# PART 26: BACKUP & RESTORE (NON-NEGOTIABLE)

## Backup Command

```bash
airports --maintenance backup [filename]
```

### Backup Contents

| Content | Included | Notes |
|---------|----------|-------|
| `server.yml` | ✓ Always | Configuration file |
| `server.db` | ✓ Always | Main database (admin credentials, settings) |
| `users.db` | ✓ If exists | User database (multi-user mode) |
| `{config_dir}/templates/` | ✓ If exists | Custom email templates |
| `{config_dir}/themes/` | ✓ If exists | Custom themes |
| `{config_dir}/ssl/` | Optional | SSL certificates (flag: `--include-ssl`) |
| `{data_dir}/` | Optional | Data files (flag: `--include-data`) |

### Admin Credentials in Backup

**Yes, admin credentials are included in the backup (`server.db`).**

| Credential | Included | Format |
|------------|----------|--------|
| Admin username | ✓ | Plain text |
| Admin password | ✓ | Hashed (bcrypt) |
| Admin API token | ✓ | Hashed |
| Admin 2FA secret | ✓ | Encrypted |
| Admin recovery keys | ✓ | Hashed |
| Additional admin accounts | ✓ | Same as above |
| OIDC/LDAP admin mappings | ✓ | Configuration only |

### Backup Format

- Single `.tar.gz` file
- Filename: `airports_backup_YYYY-MM-DD_HHMMSS.tar.gz`
- Includes manifest with version info
- Encrypted option available (`--encrypt`)

**Manifest (`manifest.json`):**
```json
{
  "version": "1.0.0",
  "created_at": "2025-01-15T10:30:00Z",
  "created_by": "administrator",
  "app_version": "1.2.3",
  "contents": [
    "server.yml",
    "server.db",
    "users.db",
    "templates/",
    "ssl/"
  ],
  "encrypted": false,
  "checksum": "sha256:abc123..."
}
```

## Restore Command

```bash
airports --maintenance restore <backup-file>
```

### Restore Behavior

| Scenario | Behavior |
|----------|----------|
| **Restore to same server** | Overwrites current config and database |
| **Restore to new server** | Primary admin must re-authenticate |
| **Version mismatch** | Warning shown, migrations applied if needed |

### Primary Admin Re-Setup on Restore

**When restoring a backup to a NEW server, the primary admin must re-authenticate:**

```
Restore completed. Primary admin re-authentication required.

A new setup token has been generated:

  Setup Token: abc123-xyz789-setup-token

Go to: https://{host}:{port}/admin

Enter the setup token to verify you are the server administrator.
Your existing password and settings will be preserved.
```

**Why Re-Authentication?**
- Prevents stolen backup from granting immediate admin access
- Verifies person restoring has server-level access (can see console)
- Preserves existing admin credentials (just requires re-verification)

**What is Preserved:**
- Admin username
- Admin password (still valid after re-auth)
- Admin 2FA settings
- Admin API token
- All configuration
- All user accounts

**What Requires Re-Setup:**
- Initial setup token verification (one-time)

### Additional Admins on Restore

| Admin Type | Restore Behavior |
|------------|------------------|
| **Primary Admin** | Must re-authenticate with setup token |
| **Additional Admins (local)** | Can log in immediately with existing credentials |
| **OIDC/LDAP Admins** | Can log in if OIDC/LDAP provider accessible |

## Admin Recovery Command

```bash
airports --maintenance setup
```

**Purpose:** Resets admin credentials and generates a new setup token. This is the ONLY way for a server admin to recover access if they have lost their password, API token, AND recovery keys.

### What It Does

| Action | Description |
|--------|-------------|
| **Clears admin password** | Admin password is set to null/empty |
| **Clears admin API token** | Admin API token is invalidated |
| **Generates new setup token** | One-time setup token displayed in console |
| **Preserves everything else** | User accounts, data, configuration unchanged |

### What It Does NOT Do

| Preserved | Description |
|-----------|-------------|
| **User accounts** | All user accounts remain intact |
| **User passwords** | No user credentials are modified |
| **User data** | All user data is preserved |
| **Configuration** | All settings except admin credentials |
| **Database** | No data is deleted or modified |
| **SSL certificates** | Certificates remain valid |

### Usage

```bash
# Stop the service first (recommended)
airports --service stop

# Run setup reset
airports --maintenance setup

# Output:
# ╔══════════════════════════════════════════════════════════════════╗
# ║                     ADMIN CREDENTIALS RESET                      ║
# ╠══════════════════════════════════════════════════════════════════╣
# ║  Admin password and API token have been cleared.                 ║
# ║                                                                  ║
# ║  NEW SETUP TOKEN (copy this now, shown ONCE):                    ║
# ║  ┌────────────────────────────────────────────────────────────┐  ║
# ║  │  setup_a7b9c2d4e6f8g0h1i3j5k7l9m1n3o5p7                    │  ║
# ║  └────────────────────────────────────────────────────────────┘  ║
# ║                                                                  ║
# ║  1. Start the service: airports --service start             ║
# ║  2. Go to: http://{host}:{port}/admin                            ║
# ║  3. Enter the setup token above                                  ║
# ║  4. Create new admin account via setup wizard                    ║
# ╚══════════════════════════════════════════════════════════════════╝

# Start the service
airports --service start
```

### Security Considerations

| Consideration | Requirement |
|---------------|-------------|
| **Requires root/admin** | Must have system-level access to run binary |
| **Console access required** | Setup token only displayed in terminal |
| **One-time token** | Token expires after use or after 24 hours |
| **Logged** | Action logged to audit log (if available) |
| **Service should be stopped** | Recommended to stop service first |

### When to Use

| Scenario | Use `--maintenance setup` |
|----------|---------------------------|
| Admin forgot password | ✓ Yes |
| Admin lost API token | ✓ Yes |
| Admin lost recovery keys | ✓ Yes |
| Admin locked out of 2FA | ✓ Yes (only if no recovery keys) |
| User forgot password | ✗ No (use password reset) |
| User locked out | ✗ No (admin can help via UI) |
| Routine password change | ✗ No (use /admin/profile) |

### Recovery Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    ADMIN RECOVERY FLOW                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Admin locked out (no password, no token, no recovery keys)     │
│                           │                                     │
│                           ▼                                     │
│  Server admin runs: airports --maintenance setup           │
│                           │                                     │
│                           ▼                                     │
│  Admin credentials cleared, new setup token generated           │
│                           │                                     │
│                           ▼                                     │
│  Admin visits /admin and enters setup token                     │
│                           │                                     │
│                           ▼                                     │
│  Setup wizard: Create new admin account                         │
│  (username, password, API token, 2FA optional)                  │
│                           │                                     │
│                           ▼                                     │
│  Admin access restored, new recovery keys issued                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

# PART 27: HEALTH & VERSIONING (NON-NEGOTIABLE)

## Health Checks

### /healthz (HTML)

- Status (healthy/unhealthy)
- Uptime
- Version
- Mode
- Node ID (cluster mode)
- Cluster status (if clustered)
- System resources (optional)

### /api/v1/healthz (JSON)

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "mode": "production",
  "uptime": "2d 5h 30m",
  "timestamp": "2024-01-15T10:30:00Z",
  "node": {
    "id": "node-abc123",
    "hostname": "server-1.example.com"
  },
  "cluster": {
    "enabled": true,
    "status": "connected",
    "nodes": 3,
    "role": "member"
  },
  "checks": {
    "database": "ok",
    "cache": "ok",
    "disk": "ok",
    "cluster": "ok"
  }
}
```

### Single Instance Response

When not in cluster mode:

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "mode": "production",
  "uptime": "2d 5h 30m",
  "timestamp": "2024-01-15T10:30:00Z",
  "node": {
    "id": "standalone",
    "hostname": "server.example.com"
  },
  "cluster": {
    "enabled": false
  },
  "checks": {
    "database": "ok",
    "cache": "ok",
    "disk": "ok"
  }
}
```

### Cluster Health Fields

| Field | Description |
|-------|-------------|
| `node.id` | Unique identifier for this node |
| `node.hostname` | Node hostname |
| `cluster.enabled` | Whether cluster mode is active |
| `cluster.status` | connected, degraded, disconnected |
| `cluster.nodes` | Number of nodes in cluster |
| `cluster.role` | member (all nodes are equal) |
| `checks.cluster` | ok, degraded, error |

## Versioning

### Format

- Stable: Semantic versioning `MAJOR.MINOR.PATCH` (e.g., `1.0.0`)
- Beta: `YYYYMMDDHHMMSS-beta` (e.g., `20251205143022-beta`)
- Daily: `YYYYMMDDHHMMSS` (e.g., `20251218060432`)

### Sources (Priority Order)

1. `release.txt` in project root
2. Git tag (if available)
3. Fallback: `dev`

### --version Output

```
airports v1.0.0
Built: 2024-01-15T10:30:00Z
Go: 1.23
OS/Arch: linux/amd64
```

---

# PART 28: ERROR HANDLING & CACHING (NON-NEGOTIABLE)

## Error Handling

### User-Facing Errors

- Clear, actionable messages
- No stack traces in production
- Appropriate HTTP status codes
- Consistent format

### Error Codes

| Code | Description |
|------|-------------|
| `ERR_VALIDATION` | Input validation failed |
| `ERR_NOT_FOUND` | Resource not found |
| `ERR_UNAUTHORIZED` | Authentication required |
| `ERR_FORBIDDEN` | Permission denied |
| `ERR_INTERNAL` | Server error |
| `ERR_RATE_LIMIT` | Rate limit exceeded |

## Caching

### Cache Drivers

| Driver | Mode |
|--------|------|
| `memory` | Single instance |
| `redis` | Cluster mode |
| `memcached` | Cluster mode |

### Cache Headers

| Content Type | Header |
|--------------|--------|
| Static assets | `Cache-Control: max-age=31536000` |
| API responses | `Cache-Control: no-cache` |
| HTML pages | `Cache-Control: no-store` |

---

# PART 29: I18N & A11Y (NON-NEGOTIABLE)

## Internationalization (i18n)

- UTF-8 everywhere
- Accept-Language header respected
- Default: English (en)
- Extensible translation system

## Accessibility (a11y)

| Requirement | Description |
|-------------|-------------|
| WCAG 2.1 AA | Compliance required |
| Keyboard | Full navigation |
| Screen readers | Full support |
| ARIA labels | Proper usage |
| Color contrast | Proper ratios |
| Focus indicators | Visible |

---

# PART 30: PROJECT-SPECIFIC SECTIONS

## Project Description

**airports** - Global airport location information API with GeoIP integration. Provides data for 29,000+ airports worldwide with search, nearby, and geographic queries.

## Project-Specific API Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/search` | GET | None | Search airports by name, city, country, ICAO, IATA |
| `/api/v1/airport/{code}` | GET | None | Get airport by ICAO or IATA code |
| `/api/v1/airports` | GET | None | List all airports (paginated) |
| `/api/v1/airports.json` | GET | None | Full database export (JSON) |
| `/api/v1/airports.csv` | GET | None | Full database export (CSV) |
| `/api/v1/airports.geojson` | GET | None | Full database export (GeoJSON) |
| `/api/v1/nearby` | GET | None | Find airports near coordinates |
| `/api/v1/countries` | GET | None | List all countries with airport counts |
| `/api/v1/stats` | GET | None | Database statistics |
| `/api/v1/geoip` | GET | None | Lookup requester's IP location |
| `/api/v1/geoip/{ip}` | GET | None | Lookup specific IP location |
| `/api/v1/geoip/airports/nearby` | GET | None | Airports near IP location |
| `/api/v1/openapi` | GET | None | OpenAPI 3.0 specification |
| `/api/v1/graphql` | GET/POST | None | GraphQL endpoint |

### Search Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `q` | string | Search query (name, city, ICAO, IATA) |
| `city` | string | Filter by city name |
| `country` | string | Filter by country code (ISO 2-letter) |
| `limit` | int | Max results (default: 50, max: 500) |
| `offset` | int | Pagination offset |

### Nearby Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `lat` | float | Latitude (required) |
| `lon` | float | Longitude (required) |
| `radius` | int | Search radius in miles (default: 50) |
| `limit` | int | Max results (default: 10) |

## Project-Specific Data Files

| File | Location | Description |
|------|----------|-------------|
| `airports.json` | `src/data/` | 29,000+ airports (8.7MB, embedded) |
| `geolite2-city-ipv4.mmdb` | `{config_dir}/geoip/` | GeoIP city data IPv4 (~50MB) |
| `geolite2-city-ipv6.mmdb` | `{config_dir}/geoip/` | GeoIP city data IPv6 (~40MB) |
| `geo-whois-asn-country.mmdb` | `{config_dir}/geoip/` | Country data (~8MB) |
| `asn.mmdb` | `{config_dir}/geoip/` | ASN data (~5MB) |

### Airport Data Structure

```json
{
  "KJFK": {
    "icao": "KJFK",
    "iata": "JFK",
    "name": "John F Kennedy International Airport",
    "city": "New York",
    "state": "New York",
    "country": "US",
    "elevation": 13,
    "lat": 40.63980103,
    "lon": -73.77890015,
    "tz": "America/New_York"
  }
}
```

## Project-Specific Configuration

```yaml
airports:
  # GeoIP database settings
  geoip:
    enabled: true
    auto_update: true
    update_schedule: "0 3 * * 0"  # Weekly Sunday 3am

  # Search settings
  search:
    default_limit: 50
    max_limit: 500

  # Nearby settings
  nearby:
    default_radius: 50
    max_radius: 500
    default_unit: miles  # miles or km
```

## Notes

- Airport data is embedded in binary (no external file needed)
- GeoIP databases downloaded on first run, updated weekly
- All airport data is public domain (OurAirports)
- GeoIP data under CC BY-SA 4.0 (MaxMind GeoLite2)

---

# PART 31: USER MANAGEMENT (NON-NEGOTIABLE)

## Overview

**Projects can operate in two modes: admin-only or multi-user.**

| Mode | Use Case | Default |
|------|----------|---------|
| **Admin-only** | Simple APIs (jokes, quotes, etc.) - just admin account | YES |
| **Multi-user** | Apps needing user accounts, registration, profiles, API tokens | NO |

## Server Admin vs Regular Users (NON-NEGOTIABLE)

**Server admins are SYSTEM ACCOUNTS responsible for managing the application itself. Regular users are end-users of the application.**

### Key Distinction

| Aspect | Server Admin | Regular User |
|--------|--------------|--------------|
| **Purpose** | Manage server, configuration, other users | Use the application features |
| **Scope** | System-wide administration | Own account and data only |
| **Storage** | `admin_credentials` table | `users` table |
| **Login** | `/auth/login` → `/admin/*` | `/auth/login` → `/user/*` |
| **Access** | Admin panel (`/admin/*`) | User routes (`/user/*`) |
| **Created by** | Setup wizard, existing admin, or OIDC/LDAP group mapping | Registration or admin invitation |

### Account Types Summary

| Account Type | Storage | Login | Access |
|--------------|---------|-------|--------|
| **Server Admin** | Database (`admin_credentials` table) | `/auth/login` | `/admin/*` only |
| **Regular Users** | Database (`users` table) | `/auth/login` | `/user/*` routes |

**Important:** Server admins and regular users are completely separate account types. A server admin is NOT a "privileged user" - they are a different kind of account entirely.

### Server Admin Behavior

| Route | Server Admin Access |
|-------|---------------------|
| `/admin/*` | Full access |
| `/user/*` | NO - treated as guest (redirect to `/admin`) |
| `/auth/login` | Login page |
| `/auth/logout` | Logout |
| Public routes (`/`, `/server/*`, etc.) | Guest view (no user-specific content) |

**Server Admin Setup (Setup Wizard Flow):**

**IMPORTANT: App works perfectly with sane defaults before setup.** Setup wizard is optional and allows customization. Server is fully functional immediately on first run.

### First Run Experience (NON-NEGOTIABLE)

**On first run, the application:**

1. Creates default `server.yml` with sane defaults
2. Creates empty `server.db` database
3. Auto-detects and configures SMTP (if available)
4. Selects random available port (64xxx range)
5. Generates one-time setup token
6. Displays startup information in console
7. **Starts serving immediately** - fully functional

**Console Output (First Run):**

```
╔══════════════════════════════════════════════════════════════════════╗
║                                                                      ║
║   AIRPORTS v1.0.0                                               ║
║                                                                      ║
║   Status: Running (first run - setup available)                      ║
║                                                                      ║
╠══════════════════════════════════════════════════════════════════════╣
║                                                                      ║
║   🌐 Web Interface:                                                   ║
║      http://localhost:64521                                          ║
║      http://192.168.1.100:64521                                      ║
║                                                                      ║
║   🔧 Admin Panel:                                                     ║
║      http://localhost:64521/admin                                    ║
║                                                                      ║
║   🔑 Setup Token (use at /admin):                                     ║
║      abc123-xyz789-setup-token-here                                  ║
║                                                                      ║
║   📧 SMTP: Auto-detected (localhost:25)                               ║
║                                                                      ║
║   ⚠️  Save the setup token! It will not be shown again.               ║
║                                                                      ║
╚══════════════════════════════════════════════════════════════════════╝

[INFO] Server started successfully
[INFO] Listening on 0.0.0.0:64521
[INFO] SMTP auto-detected: localhost:25 (enabled)
```

**If SMTP Not Detected:**

```
║   📧 SMTP: Not detected (email features disabled)                     ║
║      Configure manually at /admin/server/email                       ║
```

### SMTP Auto-Detection (NON-NEGOTIABLE)

**On first run, the application attempts to auto-detect a local SMTP server.**

| Check | Hosts | Ports |
|-------|-------|-------|
| 1 | `localhost` | 25, 587, 465 |
| 2 | `127.0.0.1` | 25, 587, 465 |
| 3 | `172.17.0.1` (Docker host) | 25, 587, 465 |
| 4 | Gateway IP | 25, 587, 465 |

**Auto-Detection Process:**
1. Try each host/port combination
2. Attempt SMTP handshake (EHLO)
3. First successful connection is used
4. Save to `server.yml` and enable email features
5. If all fail → email features disabled (not an error)

**Auto-Detected SMTP Config:**
```yaml
server:
  notifications:
    email:
      enabled: true          # Auto-enabled if detected
      autodetect: true       # Keep trying on restart if currently failed
      smtp:
        host: "localhost"    # Auto-detected
        port: 25             # Auto-detected
        username: ""         # Usually not needed for local
        password: ""
        tls: auto            # Auto-negotiate
```

**SMTP Auto-Detection Rules:**
- Only runs on first start (or if `autodetect: true` and no working SMTP)
- Does NOT override manually configured SMTP
- Silent failure - app works fine without SMTP
- Success enables all email features automatically
- Logged to console and audit log

### App Usability Before Setup

**The app is FULLY FUNCTIONAL before completing the setup wizard.**

| Feature | Available Before Setup? |
|---------|------------------------|
| Public API endpoints | ✓ Yes |
| Public web pages | ✓ Yes |
| Health checks (`/healthz`) | ✓ Yes |
| OpenAPI docs (`/openapi`) | ✓ Yes |
| GraphQL (if applicable) | ✓ Yes |
| Admin panel (`/admin`) | ✓ Yes (requires setup token) |
| Email features | ✓ Yes (if SMTP auto-detected) |
| Scheduled tasks | ✓ Yes (with defaults) |

**What Setup Wizard Provides:**
- Custom admin username/password (instead of generated)
- Customize server name/branding
- Configure optional features (Tor, SSL, multi-user)
- Receive API token for programmatic access

**Why This Matters:**
- Zero-config deployment works immediately
- Docker containers start serving instantly
- Setup can be done later at convenience
- CI/CD pipelines don't need interactive setup

### Setup Flow

On first run, a one-time setup token is generated and displayed in console. Admin setup follows this flow:

| Step | Action |
|------|--------|
| 1 | Server generates one-time setup token (displayed in console ONCE) |
| 2 | User navigates to `/admin` |
| 3 | User enters setup token |
| 4 | Redirect to `/admin/server/setup` (setup wizard) |

**Setup Wizard Steps (`/admin/server/setup`):**

**Step 1: Create Admin Account**
| Field | Default | Notes |
|-------|---------|-------|
| Username | `administrator` | Changeable to anything (username blacklist does NOT apply to admin) |
| Password | Random (generated) | User MUST copy, OR can enter custom + confirm |

**Step 2: API Token**
| Action | Notes |
|--------|-------|
| Auto-generate API token | User MUST copy (shown once) |
| Token is tied to admin account | Used for API access |

**Step 3: Server Configuration**
| Setting | Description |
|---------|-------------|
| Server name | Display name for the instance |
| Domain/FQDN | Primary domain (if known) |
| Mode | Production / Development |
| Timezone | Server timezone |

**Step 4: Optional Services**
| Setting | Description |
|---------|-------------|
| Enable Tor | Enable .onion hidden service |
| Enable SSL | Configure HTTPS |
| Enable Users | Enable multi-user mode |

**Step 5: Complete**
| Action | Notes |
|--------|-------|
| Save configuration | Write to `server.yml` |
| Mark setup complete | Setup token invalidated |
| Redirect to `/admin` | Logged in as admin |

**Setup Token Rules:**
- Generated on first run ONLY
- Displayed in console ONCE (never stored in plain text)
- Single use - invalidated after setup complete
- If lost, must reset database to regenerate

**Server Admin Credentials Storage:**
- Stored in database (`admin_credentials` table) - NEVER in config file
- Managed via `/admin/server/settings` (NOT `/user/profile`)
- NOT in the users table (separate table for isolation)
- Multiple server admin accounts supported (primary admin created during setup)

### Multiple Server Admins (NON-NEGOTIABLE)

**Server admins CAN add additional server admins.**

| Method | Description |
|--------|-------------|
| **Manual creation** | Primary admin creates additional admin accounts via `/admin/users/admins` |
| **OIDC/LDAP group mapping** | Map external identity provider groups to server admin role |

**Admin Hierarchy:**
| Admin Type | Created By | Can Create Admins? | Can Delete Admins? |
|------------|------------|--------------------|--------------------|
| **Primary Admin** | Setup wizard | Yes | Yes (except self) |
| **Additional Admins** | Primary or other admin | Yes | Yes (except self and primary) |
| **OIDC/LDAP Admins** | Group mapping | Yes | Yes (except primary) |

**Rules:**
- Primary admin cannot be deleted (only via `--maintenance setup`)
- All admins have equal permissions (except deletion hierarchy)
- OIDC/LDAP admin access is automatic based on group membership
- Removing user from external group removes admin access on next login
- All admin actions are audited with admin username

### Server Admin Security (NON-NEGOTIABLE)

**ALL security settings that apply to the primary server admin ALSO apply to additional server admins.**

| Security Feature | Applies To |
|------------------|------------|
| Password complexity requirements | All server admins |
| 2FA/MFA requirements | All server admins |
| Session timeout | All server admins |
| API token security | All server admins |
| Audit logging | All server admins |
| Rate limiting | All server admins |
| IP restrictions (if configured) | All server admins |

**No exceptions.** Additional admins do not get weaker security than the primary admin.

### Server Admin Privacy (NON-NEGOTIABLE)

**Server admins CANNOT see other server admin accounts, similar to user privacy.**

| What Admin CAN See | What Admin CANNOT See |
|--------------------|----------------------|
| Own account details | Other admin usernames |
| Own API token (regenerate) | Other admin emails |
| Own 2FA status | Other admin passwords |
| Own session history | Other admin API tokens |
| Total admin count (number only) | Other admin 2FA secrets |
| | Other admin session data |

**Admin Panel (`/admin/users/admins`):**
```
┌─────────────────────────────────────────────────────────────┐
│  Server Administrators                                      │
├─────────────────────────────────────────────────────────────┤
│  Your Account: administrator                                │
│  Total Admins: 3                                            │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  [Add New Admin]                                    │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  Note: For security, you cannot view other admin accounts.  │
│  Each admin manages their own credentials independently.    │
└─────────────────────────────────────────────────────────────┘
```

**Why This Restriction?**
- **Separation of trust**: Compromised admin cannot enumerate other admins
- **Privacy**: Admin credentials are personal, not shared
- **Security**: Prevents admin-to-admin attacks
- **Audit integrity**: Each admin accountable for own actions only

**What Admins CAN Do With Other Admins:**
| Action | Allowed? | Notes |
|--------|----------|-------|
| Know total count | ✓ | Number only, no details |
| See who is logged in | ✓ | Username only (e.g., "administrator logged in") |
| Add new admin (invite) | ✓ | Creates invite, credentials shown once to new admin |
| Remove admin (non-primary) | ✓ | By username only (must know it) |
| View other admin details | ✗ | Privacy by design |
| Reset other admin password | ✗ | Each admin manages own credentials |
| Disable other admin 2FA | ✗ | Use `--maintenance setup` for recovery |

### Adding Additional Server Admins

**Three methods to add server admins:**

| Method | Description | Use Case |
|--------|-------------|----------|
| **Invite** | Generate invite link/code | Manual admin creation |
| **OIDC Group Mapping** | Automatic via group membership | SSO environments |
| **LDAP Group Mapping** | Automatic via group membership | Enterprise environments |

#### Admin Invite Flow

```
Admin Panel (/admin/users/admins)
┌─────────────────────────────────────────────────────────────┐
│  Server Administrators                                      │
├─────────────────────────────────────────────────────────────┤
│  Your Account: administrator                                │
│  Total Admins: 2                                            │
│  Currently Online: administrator, backup-admin              │
│                                                             │
│  [+ Invite New Admin]                                       │
└─────────────────────────────────────────────────────────────┘

Admin clicks "Invite New Admin"
         ↓
┌─────────────────────────────────────────────────────────────┐
│  Invite New Server Admin                                    │
├─────────────────────────────────────────────────────────────┤
│  Username: [                    ]                           │
│                                                             │
│  Invite expires in: [24 hours ▼]                            │
│                                                             │
│  [Cancel]  [Generate Invite]                                │
└─────────────────────────────────────────────────────────────┘

         ↓ (Invite generated)

┌─────────────────────────────────────────────────────────────┐
│  ✅ Admin Invite Created                                     │
├─────────────────────────────────────────────────────────────┤
│  Username: backup-admin                                     │
│                                                             │
│  Invite URL (share with new admin):                         │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ https://app.example.com/admin/invite/abc123xyz...   │    │
│  └─────────────────────────────────────────────────────┘    │
│  [Copy URL]                                                 │
│                                                             │
│  ⚠️  This link will only work ONCE and expires in 24 hours. │
│  The new admin will set their own password on first use.    │
│                                                             │
│  [Done]                                                     │
└─────────────────────────────────────────────────────────────┘
```

**New Admin Setup (via invite link):**

```
New admin visits invite URL
         ↓
┌─────────────────────────────────────────────────────────────┐
│  Complete Admin Setup                                       │
├─────────────────────────────────────────────────────────────┤
│  Username: [backup-admin              ]                     │
│            (you can change this)                            │
│                                                             │
│  Create Password: [                    ]                    │
│  Confirm Password: [                    ]                   │
│                                                             │
│  ☐ Enable Two-Factor Authentication (recommended)           │
│                                                             │
│  [Complete Setup]                                           │
└─────────────────────────────────────────────────────────────┘

         ↓ (Setup complete)

┌─────────────────────────────────────────────────────────────┐
│  ✅ Admin Account Created                                    │
├─────────────────────────────────────────────────────────────┤
│  Your API Token (copy now - shown once):                    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ apt_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx                │    │
│  └─────────────────────────────────────────────────────┘    │
│  [Copy Token]                                               │
│                                                             │
│  ⚠️  Save this token securely. It cannot be retrieved.      │
│                                                             │
│  [Continue to Admin Panel]                                  │
└─────────────────────────────────────────────────────────────┘
```

**Invite Rules:**
- Invite link is single-use (invalidated after first use or expiry)
- Default expiry: 24 hours (configurable: 1h, 6h, 24h, 48h, 7d)
- New admin can set their own username (username blacklist ignored for admins)
- New admin sets their own password and optional 2FA
- API token generated and shown once
- Invite creation logged to audit log

#### OIDC/LDAP Admin Sync

**When a user authenticates via OIDC/LDAP and belongs to a mapped admin group:**

1. User authenticates with OIDC/LDAP provider
2. Server retrieves user's group memberships
3. If user is in `admin_groups` → create/update local admin record
4. Admin credentials synced to local database
5. On next login, even if OIDC/LDAP is down, local credentials work

**Local Sync Table (`admin_credentials`):**
| Field | Description |
|-------|-------------|
| `username` | From OIDC/LDAP claim |
| `password_hash` | Synced from provider (or set locally) |
| `source` | `local`, `oidc:{provider}`, `ldap` |
| `external_id` | Provider's user ID |
| `groups` | Cached group memberships |
| `last_sync` | Last successful sync timestamp |

**Fallback Behavior:**
| Scenario | Behavior |
|----------|----------|
| OIDC/LDAP available | Authenticate with provider, sync to local |
| OIDC/LDAP unavailable | Use cached local credentials |
| User removed from admin group | Next successful OIDC/LDAP login revokes admin |
| Provider permanently down | Local credentials continue to work |

**Sync Frequency:**
- On every login (real-time)
- Background sync every 1 hour (if configured)
- Manual sync via admin panel

### Admin Session Visibility

**Admins can see which other admins are currently logged in (username only).**

```
Admin Panel Header:
┌─────────────────────────────────────────────────────────────┐
│  {app_name} Admin    │  🟢 2 admins online  │  🔔  │  [You] │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼ (click to expand)
                       ┌──────────────────┐
                       │  Admins Online   │
                       ├──────────────────┤
                       │  🟢 administrator │
                       │  🟢 backup-admin  │
                       └──────────────────┘
```

**What IS Shown:**
- Username of logged-in admins
- Online status (green dot)
- Total count

**What is NOT Shown:**
- IP address
- Session duration
- Last activity
- Device/browser info

**Why This Is Allowed:**
- Helps admins know if others are working
- Useful for coordination
- Does not expose sensitive details
- Username is not considered private (admins know each other)

### Additional Admin Recovery

**Additional admins (non-primary) who lose access:**

| Scenario | Recovery Method |
|----------|-----------------|
| Forgot password | Use own recovery keys OR contact primary admin |
| Lost 2FA + recovery keys | Contact primary admin to delete account, re-invite |
| OIDC/LDAP admin locked out | Fix in identity provider, or primary admin removes mapping |

**Primary admin CANNOT:**
- Reset other admin's password directly
- View other admin's credentials
- Disable other admin's 2FA directly

**Primary admin CAN:**
- Delete the additional admin account entirely
- Re-invite them (they set up fresh credentials)

### Regular User Behavior

| Route | Regular User Access |
|-------|---------------------|
| `/admin/*` | NO - 403 Forbidden (unless user has admin role) |
| `/user/*` | Full access to own profile, settings, tokens |
| `/auth/login` | Login page |
| `/auth/logout` | Logout |
| Public routes | Authenticated view (may show user-specific content) |

**Regular User Accounts:**
- Stored in database (users table)
- Managed via `/user/profile`, `/user/settings`
- Can have roles (admin, user, custom)
- Multiple accounts supported

### Username Validation (NON-NEGOTIABLE)

**Username Rules:**
| Rule | Value |
|------|-------|
| Min length | 3 characters |
| Max length | 32 characters |
| Allowed chars | `a-z`, `0-9`, `_`, `-` (lowercase only) |
| Must start with | Letter (`a-z`) |
| Cannot end with | `_` or `-` |
| No consecutive | `__`, `--`, `_-`, `-_` |

**Username Blocklist (case-insensitive):**

```go
var UsernameBlocklist = []string{
    // System & Administrative
    "admin", "administrator", "root", "system", "sysadmin", "superuser",
    "master", "owner", "operator", "manager", "moderator", "mod",
    "staff", "support", "helpdesk", "help", "service", "daemon",

    // Server & Technical
    "server", "host", "node", "cluster", "api", "www", "web", "mail",
    "email", "smtp", "ftp", "ssh", "dns", "proxy", "gateway", "router",
    "firewall", "localhost", "local", "internal", "external", "public",
    "private", "network", "database", "db", "cache", "redis", "mysql",
    "postgres", "mongodb", "elastic", "nginx", "apache", "docker",

    // Application & Service Names
    "app", "application", "bot", "robot", "crawler", "spider", "scraper",
    "webhook", "callback", "cron", "scheduler", "worker", "queue", "job",
    "task", "process", "service", "microservice", "lambda", "function",

    // Authentication & Security
    "auth", "authentication", "login", "logout", "signin", "signout",
    "signup", "register", "password", "passwd", "token", "oauth", "sso",
    "saml", "ldap", "kerberos", "security", "secure", "ssl", "tls",
    "certificate", "cert", "key", "secret", "credential", "session",

    // Roles & Permissions
    "guest", "anonymous", "anon", "user", "users", "member", "members",
    "subscriber", "editor", "author", "contributor", "reviewer", "auditor",
    "analyst", "developer", "dev", "devops", "engineer", "architect",
    "designer", "tester", "qa", "billing", "finance", "legal", "hr",
    "sales", "marketing", "ceo", "cto", "cfo", "coo", "founder", "cofounder",

    // Common Reserved
    "account", "accounts", "profile", "profiles", "settings", "config",
    "configuration", "dashboard", "panel", "console", "portal", "home",
    "index", "main", "default", "null", "nil", "undefined", "void",
    "true", "false", "test", "testing", "debug", "demo", "example",
    "sample", "temp", "temporary", "tmp", "backup", "archive", "log",
    "logs", "audit", "report", "reports", "analytics", "stats", "status",

    // API & Endpoints
    "api", "rest", "graphql", "grpc", "websocket", "ws", "wss", "http",
    "https", "endpoint", "endpoints", "route", "routes", "path", "url",
    "uri", "callback", "hook", "hooks", "event", "events", "stream",

    // Content & Media
    "blog", "news", "article", "articles", "post", "posts", "page", "pages",
    "feed", "rss", "atom", "sitemap", "robots", "favicon", "static",
    "assets", "images", "image", "img", "media", "upload", "uploads",
    "download", "downloads", "file", "files", "document", "documents",

    // Communication
    "contact", "message", "messages", "chat", "notification", "notifications",
    "alert", "alerts", "inbox", "outbox", "sent", "draft", "drafts",
    "spam", "abuse", "report", "flag", "block", "mute", "ban",

    // Commerce & Billing
    "shop", "store", "cart", "checkout", "order", "orders", "invoice",
    "invoices", "payment", "payments", "subscription", "subscriptions",
    "plan", "plans", "pricing", "billing", "refund", "coupon", "discount",

    // Social Features
    "follow", "follower", "followers", "following", "friend", "friends",
    "like", "likes", "share", "shares", "comment", "comments", "reply",
    "mention", "mentions", "tag", "tags", "group", "groups", "team", "teams",
    "community", "communities", "forum", "forums", "channel", "channels",

    // Brand & Legal
    "official", "verified", "trusted", "partner", "affiliate", "sponsor",
    "brand", "trademark", "copyright", "legal", "terms", "privacy",
    "policy", "policies", "tos", "eula", "gdpr", "dmca", "abuse",

    // Offensive / Impersonation Prevention
    "fuck", "shit", "ass", "bitch", "bastard", "damn", "cunt", "dick",
    "penis", "vagina", "sex", "porn", "xxx", "nude", "naked", "nsfw",
    "kill", "murder", "death", "die", "suicide", "hate", "nazi", "hitler",
    "racist", "racism", "terrorist", "terrorism", "isis", "alqaeda",

    // Numbers & Special
    "0", "1", "123", "1234", "12345", "000", "111", "666", "911", "420", "69",

    // Common Spam Patterns
    "info", "noreply", "no-reply", "donotreply", "mailer", "postmaster",
    "webmaster", "hostmaster", "abuse", "spam", "junk", "trash",

    // Project-specific (dynamic)
    "airports", "apimgr",
}
```

**Blocklist Notes:**
- Server admin account is exempt from this blocklist
- Blocklist is checked case-insensitively
- Also blocks usernames that contain blocklisted words as substrings for critical terms (admin, root, system, mod, official, verified)
- Custom blocklist entries can be added via config

### Username & Email Rules (NON-NEGOTIABLE)

**Case Insensitivity:**
- Usernames are case-insensitive: `admin` = `Admin` = `aDmIn`
- Emails are case-insensitive: `me@example.com` = `Me@Example.COM`
- Stored in lowercase, compared in lowercase

**Login Identifier:**
- Users can log in with **either** username OR email
- Both must be unique across all users

**Email Addresses:**
| SMTP Configured | Max Emails | Verification |
|-----------------|------------|--------------|
| No | 1 (primary only) | Not possible |
| Yes | Unlimited | Additional emails must be verified |

**Email Types:**
| Type | Purpose | Used For |
|------|---------|----------|
| **Account Email** | Security & account recovery | Password reset, 2FA recovery, security alerts, login notifications |
| **Notification Email** | Non-security communications | Newsletters, updates, marketing, general notifications |

**Email Rules:**
- Primary email set at registration (becomes account email by default)
- User can designate a different verified email as account email
- Additional emails require verification (SMTP required)
- All emails must be unique across all users
- Unverified emails cannot be used for login
- Account email receives security-sensitive communications ONLY
- Notification email receives everything else (if set, otherwise account email)

### Server Admin Limitations (NON-NEGOTIABLE)

**What Server Admin CAN do:**
| Action | Description |
|--------|-------------|
| Send password reset link | Triggers email to user's account email |
| Disable user's 2FA | After manual identity verification (out-of-band) |
| Disable/suspend account | Block user from logging in |
| Enable/unsuspend account | Restore access |
| View masked email | `j***n@e***.com` (for support identification) |
| View username | For support identification |
| View account status | Active, suspended, 2FA enabled, etc. |
| View last login | Timestamp only |

**What Server Admin CANNOT do:**
| Action | Reason |
|--------|--------|
| View full email addresses | Privacy - only masked version visible |
| View passwords | Passwords are hashed, not stored |
| Set/change user passwords | Only user can set via reset link |
| View recovery keys | Keys are hashed, not stored |
| View 2FA secrets | Secrets are encrypted with user's password |
| Read user's private data | Privacy by design |
| Impersonate without logging | All admin actions are audited |

**Admin Password Reset Flow:**
```
Admin Panel (/admin/users/{id})
┌─────────────────────────────────────────────────────────────┐
│  User: johndoe                                              │
│  Email: j***n@e***.com (masked)                             │
│  Status: Active                                             │
│  2FA: Enabled                                               │
│  Last Login: 2025-01-15 09:00:00                            │
├─────────────────────────────────────────────────────────────┤
│  Actions:                                                   │
│  [Send Password Reset]  [Disable 2FA]  [Suspend Account]    │
└─────────────────────────────────────────────────────────────┘

Admin clicks "Send Password Reset"
         ↓
┌─────────────────────────────────────────────────────────────┐
│  Confirm Action                                             │
├─────────────────────────────────────────────────────────────┤
│  This will send a password reset link to the user's         │
│  account email (j***n@e***.com).                            │
│                                                             │
│  You will NOT see the reset link or new password.           │
│                                                             │
│  Reason (required for audit log):                           │
│  [User requested via support ticket #1234    ]              │
│                                                             │
│  [Cancel]  [Send Reset Link]                                │
└─────────────────────────────────────────────────────────────┘

         ↓ (email sent to user)

User receives: "Password reset requested by administrator.
               Click here to set a new password."
```

**Why These Limitations?**
- **Zero-knowledge design**: Admin cannot access what they don't need
- **Privacy by default**: User data is user's data
- **Audit trail**: All admin actions logged with reason
- **Trust minimization**: Even compromised admin account has limited damage potential

### Error Messages (NON-NEGOTIABLE)

**Specific Errors (OK to reveal - validation only):**
| Scenario | Error Message |
|----------|---------------|
| Blocklisted username | `Username contains blocked word: {word}` |
| Username too short | `Username must be at least 3 characters` |
| Username too long | `Username cannot exceed 32 characters` |
| Invalid characters | `Username can only contain lowercase letters, numbers, underscore, and hyphen` |
| Invalid email format | `Please enter a valid email address` |
| Password too weak | `Password must be at least 8 characters` |

**Generic Errors (NEVER reveal existence):**
| Scenario | Error Message |
|----------|---------------|
| Username/email taken | `Unable to complete registration. [Forgot credentials?](/auth/forgot)` |
| Login failed (any reason) | `Invalid credentials. [Forgot password?](/auth/forgot)` |
| Reset request | `If an account exists, instructions have been sent.` |

**Why Generic Errors?**
- Prevents username/email enumeration attacks
- Attacker cannot determine if account exists
- Same response time for both cases (prevent timing attacks)
- Links to recovery flow instead of revealing information

### 2FA, Passkeys & OIDC (NON-NEGOTIABLE)

**Supported Authentication Methods:**
| Method | Description |
|--------|-------------|
| Password | Standard username/email + password |
| TOTP (2FA) | Time-based one-time passwords (Google Authenticator, Authy, etc.) |
| Passkeys | WebAuthn/FIDO2 passwordless authentication |
| OIDC | External identity providers |

**OIDC Providers (Examples):**
- Self-hosted: Authentik, Authelia, Keycloak, Dex, Zitadel
- Cloud: Auth0, Okta, Azure AD, Google, GitHub, GitLab

**Recovery Keys (CRITICAL):**
| Rule | Description |
|------|-------------|
| **Generated once** | Recovery keys generated when 2FA/passkey enabled |
| **User must copy** | Displayed ONCE, user MUST save them |
| **Hashed storage** | Keys are hashed, NOT stored in plain text |
| **NOT recoverable** | If lost, cannot be retrieved - account recovery required |
| **Single use** | Each recovery key can only be used once |
| **Count** | 10 recovery keys generated |

**Recovery Key Flow:**
```
┌─────────────────────────────────────────────────────────────┐
│  🔑 SAVE YOUR RECOVERY KEYS                                 │
├─────────────────────────────────────────────────────────────┤
│  These keys can be used to access your account if you       │
│  lose access to your 2FA device. Each key can only be       │
│  used once.                                                 │
│                                                             │
│  ⚠️  SAVE THESE NOW - THEY WILL NOT BE SHOWN AGAIN          │
│                                                             │
│  1. a1b2c3d4-e5f6    6. k5l6m7n8-o9p0                       │
│  2. g7h8i9j0-k1l2    7. q1r2s3t4-u5v6                       │
│  3. m3n4o5p6-q7r8    8. w7x8y9z0-a1b2                       │
│  4. s9t0u1v2-w3x4    9. c3d4e5f6-g7h8                       │
│  5. y5z6a7b8-c9d0   10. i9j0k1l2-m3n4                       │
│                                                             │
│  [Download as TXT]  [Copy All]                              │
│                                                             │
│  ☑️ I have saved my recovery keys                            │
│                                                             │
│  [Continue]                                                 │
└─────────────────────────────────────────────────────────────┘
```

**2FA/Passkey Setup Requirements:**
1. User must be logged in
2. User must re-enter password to confirm
3. Recovery keys generated and displayed
4. User must confirm they saved recovery keys (checkbox)
5. Only then is 2FA/passkey activated

### Account Recovery Matrix (NON-NEGOTIABLE)

**What CAN be recovered:**
| User Knows | User Forgot | Recovery Method |
|------------|-------------|-----------------|
| Email | Password | `/auth/forgot` → email link → set new password |
| Username | Password | `/auth/forgot` → email link → set new password |
| Email + Password | 2FA code | `/auth/login` → recovery key → disable/reset 2FA |
| Username + Password | 2FA code | `/auth/login` → recovery key → disable/reset 2FA |

**What CANNOT be recovered:**
| Scenario | Result |
|----------|--------|
| Forgot username AND email | ❌ No recovery - account lost |
| Forgot password + no email access | ❌ No recovery - account lost |
| Lost 2FA + no recovery keys | ❌ Contact admin (manual identity verification) |
| Lost everything | ❌ No recovery - account lost |

**Why no recovery without username/email?**
- We cannot store data that identifies users without credentials
- This is a security feature, not a limitation
- Users MUST remember at least one identifier (username or email)

### Server Admin Recovery (NON-NEGOTIABLE)

The server admin (system administrator with access to the server/binary) has ONE recovery method:

| Scenario | Recovery Method |
|----------|-----------------|
| Admin forgot password | `airports --maintenance setup` |
| Admin lost API token | `airports --maintenance setup` |
| Admin lost recovery keys | `airports --maintenance setup` |
| Admin lost 2FA + no recovery keys | `airports --maintenance setup` |
| Admin lost everything | `airports --maintenance setup` |

**This requires:**
- System-level access to run the binary
- Console access to see the new setup token
- The service should be stopped first

**This does NOT require:**
- Previous password
- Previous API token
- Previous recovery keys
- Email access

See **PART 26: BACKUP & RESTORE → Admin Recovery Command** for full details.

### Recovery Key Usage Flow

**When Recovery Keys Are Used:**
- User has 2FA/passkey enabled
- User lost access to 2FA device (phone lost, authenticator wiped, etc.)
- User still knows username/email AND password

**Flow:**
```
┌─────────────────────────────────────────────────────────────┐
│  /auth/login                                                │
├─────────────────────────────────────────────────────────────┤
│  Username/Email: [john@example.com        ]                 │
│  Password:       [••••••••••••            ]                 │
│                                                             │
│  [Login]                                                    │
└─────────────────────────────────────────────────────────────┘
                            ↓
                    (2FA is enabled)
                            ↓
┌─────────────────────────────────────────────────────────────┐
│  Two-Factor Authentication                                  │
├─────────────────────────────────────────────────────────────┤
│  Enter the 6-digit code from your authenticator app:        │
│                                                             │
│  [      ]                                                   │
│                                                             │
│  [Verify]                                                   │
│                                                             │
│  ─────────────────────────────────────────────────────────  │
│  Lost access to your authenticator?                         │
│  [Use Recovery Key]                                         │
└─────────────────────────────────────────────────────────────┘
                            ↓
                (User clicks "Use Recovery Key")
                            ↓
┌─────────────────────────────────────────────────────────────┐
│  Use Recovery Key                                           │
├─────────────────────────────────────────────────────────────┤
│  Enter one of your recovery keys:                           │
│                                                             │
│  [a1b2c3d4-e5f6                          ]                  │
│                                                             │
│  ⚠️  This key will be invalidated after use.                 │
│                                                             │
│  [Submit]                                                   │
└─────────────────────────────────────────────────────────────┘
                            ↓
                (Valid recovery key entered)
                            ↓
┌─────────────────────────────────────────────────────────────┐
│  Recovery Key Accepted                                      │
├─────────────────────────────────────────────────────────────┤
│  ✅ Recovery key accepted and invalidated.                   │
│  You have X recovery keys remaining.                        │
│                                                             │
│  What would you like to do?                                 │
│                                                             │
│  ○ Disable 2FA temporarily (login with password only)       │
│  ○ Set up new 2FA device now                                │
│                                                             │
│  [Continue]                                                 │
└─────────────────────────────────────────────────────────────┘
                            ↓
        (If "Set up new 2FA" → new recovery keys generated)
                            ↓
┌─────────────────────────────────────────────────────────────┐
│  🔑 NEW RECOVERY KEYS                                        │
├─────────────────────────────────────────────────────────────┤
│  Your old recovery keys have been invalidated.              │
│  Save these new recovery keys:                              │
│                                                             │
│  1. x1y2z3a4-b5c6    6. p5q6r7s8-t9u0                       │
│  2. d7e8f9g0-h1i2    7. v1w2x3y4-z5a6                       │
│  3. j3k4l5m6-n7o8    8. b7c8d9e0-f1g2                       │
│  4. q9r0s1t2-u3v4    9. h3i4j5k6-l7m8                       │
│  5. w5x6y7z8-a9b0   10. n9o0p1q2-r3s4                       │
│                                                             │
│  [Download as TXT]  [Copy All]                              │
│                                                             │
│  ☑️ I have saved my recovery keys                            │
│                                                             │
│  [Complete Setup]                                           │
└─────────────────────────────────────────────────────────────┘
```

**Recovery Key API Flow:**
```
POST /api/v1/auth/login
  → { "identifier": "john@example.com", "password": "..." }
  ← { "requires_2fa": true, "session_token": "temp_xxx" }

POST /api/v1/auth/forgot/recovery
  → { "session_token": "temp_xxx", "recovery_key": "a1b2c3d4-e5f6" }
  ← { "success": true, "remaining_keys": 9 }

POST /api/v1/auth/2fa/disable  (or /setup for new device)
  → { "session_token": "temp_xxx" }
  ← { "success": true, "token": "auth_xxx" }
     OR (if setup)
  ← { "qr_code": "...", "secret": "...", "recovery_keys": [...] }
```

**Recovery Key Rules:**
| Rule | Description |
|------|-------------|
| Single use | Each key invalidated after one use |
| Hashed storage | Keys stored as hashes, cannot be retrieved |
| Regeneration | New 2FA setup = new recovery keys |
| Old keys invalidated | All old keys invalidated when new keys generated |
| Remaining count | Show user how many keys remain |
| Last key warning | ⚠️ Warning when only 1-2 keys remaining |
| Zero keys + lost 2FA | Must contact admin for manual verification |

**OIDC/LDAP Configuration:**
```yaml
server:
  auth:
    oidc:
      enabled: false
      providers:
        - name: authentik
          display_name: "Login with Authentik"
          issuer: "https://auth.example.com/application/o/myapp/"
          client_id: "{client_id}"
          client_secret: "{client_secret}"
          scopes: ["openid", "profile", "email", "groups"]
          # Auto-create user on first login
          auto_register: true
          # Map OIDC claims to user fields
          claims_mapping:
            username: "preferred_username"
            email: "email"
            name: "name"
            groups: "groups"  # Claim containing group memberships
          # Map external groups to server admin role
          admin_groups:
            - "admins"              # Users in this group become server admins
            - "server-admins"
            - "app-administrators"
          # Map external groups to user roles (if multi-user enabled)
          role_mapping:
            admin: ["admins", "app-administrators"]
            moderator: ["moderators", "support-staff"]
            user: ["users", "members"]

    ldap:
      enabled: false
      server: "ldap://ldap.example.com:389"
      bind_dn: "cn=readonly,dc=example,dc=com"
      bind_password: "{ldap_password}"
      base_dn: "dc=example,dc=com"
      user_filter: "(uid={username})"
      # Map LDAP attributes to user fields
      attributes:
        username: "uid"
        email: "mail"
        name: "cn"
        groups: "memberOf"
      # Map LDAP groups to server admin role
      admin_groups:
        - "cn=admins,ou=groups,dc=example,dc=com"
        - "cn=server-admins,ou=groups,dc=example,dc=com"
      # Map LDAP groups to user roles (if multi-user enabled)
      role_mapping:
        admin: ["cn=admins,ou=groups,dc=example,dc=com"]
        moderator: ["cn=moderators,ou=groups,dc=example,dc=com"]
        user: ["cn=users,ou=groups,dc=example,dc=com"]
```

### Server Admin Group Mapping (NON-NEGOTIABLE)

**Server admins can map external identity provider groups to the server admin role.**

| Provider | Configuration | Description |
|----------|---------------|-------------|
| **OIDC** | `admin_groups` list | Group names from the `groups` claim |
| **LDAP** | `admin_groups` list | Full DN of groups (e.g., `cn=admins,ou=groups,dc=example,dc=com`) |

**How Group Mapping Works:**

1. User authenticates via OIDC/LDAP
2. Server retrieves user's group memberships from identity provider
3. If user belongs to ANY group in `admin_groups` → grant server admin access
4. Admin access persists for session duration
5. On next login, group membership is re-evaluated
6. If user is removed from all `admin_groups` → admin access revoked

**Admin Panel (`/admin/security/auth`):**

| Element | Type | Description |
|---------|------|-------------|
| OIDC Providers | Section | List of configured OIDC providers |
| LDAP Configuration | Section | LDAP server settings |
| Admin Groups | Tag input | Groups that grant server admin access |
| Role Mapping | Table | Map external groups to application roles |
| Test Connection | Button | Test OIDC/LDAP connectivity |
| Test Group Mapping | Button | Test user's groups and resulting role |

**Sane Defaults:**
- `admin_groups`: Empty (no external groups granted admin by default)
- `role_mapping`: Empty (users get default role unless mapped)
- `auto_register`: `false` (users must be pre-created unless enabled)
- Group membership checked on every login (not cached)

### Why This Separation?

| Reason | Description |
|--------|-------------|
| **Security** | Server admin has system-level access, not app-level |
| **Simplicity** | Admin-only mode doesn't need user management |
| **Isolation** | Server admin credentials separate from user data |
| **Recovery** | Can access admin even if database is corrupted |

## Configuration

```yaml
server:
  users:
    # Enable multi-user mode (default: disabled = admin-only)
    enabled: false

    registration:
      # Allow public registration
      enabled: false
      # Require email verification before account is active
      require_email_verification: true
      # Admin must approve new users
      require_approval: false
      # Allowed email domains (empty = all allowed)
      allowed_domains: []
      # Blocked email domains
      blocked_domains: []

    roles:
      # Available roles
      available:
        - admin
        - user
      # Default role for new users
      default: user

    tokens:
      # Allow users to generate API tokens
      enabled: true
      # Maximum tokens per user
      max_per_user: 5
      # Token expiration (0 = never)
      expiration_days: 0

    profile:
      # Allow users to upload avatars
      allow_avatar: true
      # Allow users to set display name
      allow_display_name: true
      # Allow users to set bio
      allow_bio: true

    auth:
      # Session duration
      session_duration: 30d
      # Require 2FA for all users
      require_2fa: false
      # Allow 2FA (user choice)
      allow_2fa: true
      # Password requirements
      password_min_length: 8
      password_require_uppercase: false
      password_require_number: false
      password_require_special: false

    limits:
      # Rate limits per user (0 = use global)
      requests_per_minute: 0
      requests_per_day: 0
```

## User Roles & Permissions

| Role | Description | Default Permissions |
|------|-------------|---------------------|
| `admin` | Full access | All permissions |
| `user` | Standard user | Read, own profile, own API tokens |

### Custom Roles

Projects can define custom roles with specific permissions:

```yaml
server:
  users:
    roles:
      available:
        - admin
        - moderator
        - user
        - readonly
      default: user
      permissions:
        moderator:
          - read
          - write
          - moderate
        readonly:
          - read
```

## User Features

### Registration Flow

```
1. User submits registration form
   ├─ If require_email_verification: Send verification email
   │   └─ User clicks link → account verified
   ├─ If require_approval: Admin notified
   │   └─ Admin approves → account active
   └─ If neither: Account immediately active

2. User can now log in
```

### Authentication Methods

| Method | Use For |
|--------|---------|
| Session (cookie) | Web interface |
| API token | API access (passed as Bearer token in Authorization header) |

### Password Reset Flow

```
1. User requests password reset
2. Email sent with reset link (expires in 1 hour)
3. User clicks link, sets new password
4. All existing sessions invalidated
5. User must log in with new password
```

### Two-Factor Authentication (2FA)

| Feature | Description |
|---------|-------------|
| TOTP | Time-based one-time passwords (Google Authenticator, etc.) |
| Backup codes | One-time use recovery codes |
| Remember device | Optional "trust this device" for 30 days |

## User Profile

| Field | Type | Configurable |
|-------|------|--------------|
| Email | Required | No (always required) |
| Display name | Optional | `profile.allow_display_name` |
| Avatar | Optional | `profile.allow_avatar` |
| Bio | Optional | `profile.allow_bio` |
| Timezone | Optional | Always available |
| Language | Optional | Always available |

## API Tokens

| Feature | Description |
|---------|-------------|
| Generate | User can create API tokens from profile |
| Name/Label | User can name tokens for identification |
| Permissions | Optional: limit token to specific scopes |
| Expiration | Optional: set expiry date |
| Last used | Track when token was last used |
| Revoke | User can delete tokens anytime |

### API Token Format

```
airports_{random_32_chars}

Example: jokes_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
```

## Admin Panel

### /admin/users (User Management)

| Element | Type | Description |
|---------|------|-------------|
| User list | Table | All users with search/filter |
| Create user | Button | Manually create user account |
| Edit user | Button | Modify user details |
| Delete user | Button | Remove user (confirmation required) |
| Impersonate | Button | Log in as user (admin only) |
| Disable/Enable | Toggle | Temporarily disable account |
| Reset password | Button | Send password reset email |
| Revoke sessions | Button | Log user out everywhere |

### /admin/users/{id} (User Detail)

| Section | Contents |
|---------|----------|
| Profile | Email, name, avatar, bio, role |
| Security | 2FA status, sessions, password reset |
| API Tokens | List of user's API tokens |
| Activity | Login history, API usage |
| Limits | Per-user rate limits |

### /admin/roles (Role Management)

| Element | Type | Description |
|---------|------|-------------|
| Role list | Table | All roles |
| Create role | Button | Define new role |
| Edit permissions | Checkboxes | Set role permissions |
| Delete role | Button | Remove role (reassign users first) |

### /admin/invites (Invitation Codes)

| Element | Type | Description |
|---------|------|-------------|
| Generate invite | Button | Create invitation code/link |
| Invite list | Table | All invites with status |
| Expiration | Date picker | When invite expires |
| Max uses | Number | How many times invite can be used |
| Role | Dropdown | What role invited users get |
| Revoke | Button | Disable invite |

## Route Standards (NON-NEGOTIABLE)

**All routes MUST follow these standards:**

| Rule | Description |
|------|-------------|
| **Scoped** | Routes grouped by scope: `/auth`, `/user`, `/org`, `/admin` |
| **Mirrored** | Web (`/`) and API (`/api/v1/`) use same structure |
| **Intuitive** | Simple, predictable paths |
| **Params over queries** | Use path params, limit query params to defined cases |
| **Duplicated when needed** | Same resource may exist in multiple scopes |

### Response Formats

| Route | Default | Options |
|-------|---------|---------|
| `/` (web) | HTML | - |
| `/api/v1/` | JSON (`application/json`) | JSON, Text |
| `/api/v1/**/*.txt` | Text (`text/plain`) | - |

### Scopes

| Scope | Web | API | Description |
|-------|-----|-----|-------------|
| Public | `/` | `/api/v1/` | Public resources, unauthenticated |
| Auth | `/auth/` | `/api/v1/auth/` | Authentication flows |
| User | `/user/` | `/api/v1/user/` | Current user's resources |
| Org | `/org/` | `/api/v1/org/` | Organization resources (if applicable) |
| Admin | `/admin/` | `/api/v1/admin/` | Admin/server management |

## Web Routes

### Public (`/`)

| Path | Description |
|------|-------------|
| `/` | Home page |
| `/healthz` | Health check |
| `/openapi` | Swagger UI |
| `/graphql` | GraphiQL interface |

### Server (`/server/`)

| Path | Description |
|------|-------------|
| `/server/about` | About the application |
| `/server/privacy` | Privacy policy |
| `/server/contact` | Contact form |
| `/server/help` | Help / documentation |

### Auth (`/auth/`)

| Path | Description |
|------|-------------|
| `/auth/login` | Login form |
| `/auth/logout` | Logout |
| `/auth/register` | Registration form |
| `/auth/forgot` | Account recovery (password reset or recovery key) |
| `/auth/forgot/{token}` | Set new password (from email link) |
| `/auth/verify/{token}` | Email verification |

### User (`/user/`)

| Path | Description |
|------|-------------|
| `/user/profile` | View/edit profile |
| `/user/settings` | Account settings |
| `/user/tokens` | Manage API tokens |
| `/user/security` | 2FA, sessions |
| `/user/security/sessions` | Active sessions |
| `/user/security/2fa` | Two-factor settings |

### Admin (`/admin/`)

| Path | Description |
|------|-------------|
| `/admin` | Dashboard |
| `/admin/profile` | Admin profile (password, API token, icon) |
| `/admin/server/setup` | Initial setup wizard |
| `/admin/server/settings` | Server settings |
| `/admin/server/branding` | Branding & SEO |
| `/admin/server/ssl` | SSL/TLS settings |
| `/admin/server/tor` | Tor hidden service |
| `/admin/server/web` | Web settings (robots.txt, security.txt) |
| `/admin/server/pages` | Standard pages (about, privacy, contact) |
| `/admin/server/email` | Email/SMTP settings |
| `/admin/server/email/templates` | Email templates |
| `/admin/server/notifications` | Notification settings |
| `/admin/server/scheduler` | Scheduled tasks |
| `/admin/server/backup` | Backup & restore |
| `/admin/server/logs` | Log viewer |
| `/admin/users` | User management |
| `/admin/users/{id}` | User detail |
| `/admin/roles` | Role management |
| `/admin/invites` | Invitation codes |

## API Routes

### Public (`/api/v1/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/healthz` | GET | Health check |
| `/api/v1/openapi.json` | GET | OpenAPI spec (JSON only) |

### Server (`/api/v1/server/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/server/about` | GET | About information |
| `/api/v1/server/privacy` | GET | Privacy policy |
| `/api/v1/server/contact` | POST | Submit contact form |
| `/api/v1/server/help` | GET | Help content |

### Auth (`/api/v1/auth/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/auth/register` | POST | Register new user |
| `/api/v1/auth/login` | POST | User login |
| `/api/v1/auth/logout` | POST | User logout |
| `/api/v1/auth/forgot` | POST | Request account recovery (sends email) |
| `/api/v1/auth/forgot/reset` | POST | Set new password (with token from email) |
| `/api/v1/auth/forgot/recovery` | POST | Use recovery key (for 2FA bypass) |
| `/api/v1/auth/verify` | POST | Verify email address |
| `/api/v1/auth/refresh` | POST | Refresh session/token |

### User (`/api/v1/user/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/user/profile` | GET | Get own profile |
| `/api/v1/user/profile` | PATCH | Update own profile |
| `/api/v1/user/password` | POST | Change password |
| `/api/v1/user/tokens` | GET | List own API tokens |
| `/api/v1/user/tokens` | POST | Create API token |
| `/api/v1/user/tokens/{id}` | GET | Get token details |
| `/api/v1/user/tokens/{id}` | DELETE | Revoke API token |
| `/api/v1/user/sessions` | GET | List active sessions |
| `/api/v1/user/sessions/{id}` | DELETE | Revoke session |
| `/api/v1/user/2fa` | GET | Get 2FA status |
| `/api/v1/user/2fa/enable` | POST | Enable 2FA |
| `/api/v1/user/2fa/disable` | POST | Disable 2FA |
| `/api/v1/user/2fa/backup-codes` | POST | Generate backup codes |

### Admin - Users (`/api/v1/admin/users/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/users` | GET | List all users |
| `/api/v1/admin/users` | POST | Create user |
| `/api/v1/admin/users/{id}` | GET | Get user details |
| `/api/v1/admin/users/{id}` | PATCH | Update user |
| `/api/v1/admin/users/{id}` | DELETE | Delete user |
| `/api/v1/admin/users/{id}/disable` | POST | Disable user |
| `/api/v1/admin/users/{id}/enable` | POST | Enable user |
| `/api/v1/admin/users/{id}/impersonate` | POST | Get impersonation token |

### Admin - Roles (`/api/v1/admin/roles/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/roles` | GET | List roles |
| `/api/v1/admin/roles` | POST | Create role |
| `/api/v1/admin/roles/{id}` | GET | Get role details |
| `/api/v1/admin/roles/{id}` | PATCH | Update role |
| `/api/v1/admin/roles/{id}` | DELETE | Delete role |

### Admin - Invites (`/api/v1/admin/invites/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/invites` | GET | List invites |
| `/api/v1/admin/invites` | POST | Create invite |
| `/api/v1/admin/invites/{id}` | GET | Get invite details |
| `/api/v1/admin/invites/{id}` | DELETE | Revoke invite |

### Admin - Server (`/api/v1/admin/server/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/setup` | GET | Get setup status (is_complete, current_step) |
| `/api/v1/admin/server/setup/verify` | POST | Verify setup token |
| `/api/v1/admin/server/setup/account` | POST | Create admin account (Step 1) |
| `/api/v1/admin/server/setup/config` | POST | Save server config (Step 3) |
| `/api/v1/admin/server/setup/services` | POST | Configure services (Step 4) |
| `/api/v1/admin/server/setup/complete` | POST | Complete setup wizard (Step 5) |
| `/api/v1/admin/server/settings` | GET | Get server settings |
| `/api/v1/admin/server/settings` | PATCH | Update server settings |
| `/api/v1/admin/server/status` | GET | Server status |
| `/api/v1/admin/server/health` | GET | Detailed health |
| `/api/v1/admin/server/stats` | GET | Statistics |
| `/api/v1/admin/server/restart` | POST | Restart server |

### Admin - Profile (`/api/v1/admin/profile/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/profile` | GET | Get admin profile |
| `/api/v1/admin/profile` | PATCH | Update admin profile (username, icon) |
| `/api/v1/admin/profile/password` | POST | Change admin password |
| `/api/v1/admin/profile/token` | GET | Get current API token (masked) |
| `/api/v1/admin/profile/token` | POST | Regenerate API token (returns new token ONCE) |

### Admin - Branding (`/api/v1/admin/server/branding/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/branding` | GET | Get branding settings |
| `/api/v1/admin/server/branding` | PATCH | Update branding |

### Admin - SSL (`/api/v1/admin/server/ssl/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/ssl` | GET | Get SSL settings |
| `/api/v1/admin/server/ssl` | PATCH | Update SSL settings |
| `/api/v1/admin/server/ssl/renew` | POST | Force certificate renewal |

### Admin - Tor (`/api/v1/admin/server/tor/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/tor` | GET | Get Tor status |
| `/api/v1/admin/server/tor` | PATCH | Update Tor settings |
| `/api/v1/admin/server/tor/regenerate` | POST | Regenerate .onion address |
| `/api/v1/admin/server/tor/vanity` | GET | Get vanity generation status |
| `/api/v1/admin/server/tor/vanity` | POST | Start vanity generation |
| `/api/v1/admin/server/tor/vanity` | DELETE | Cancel vanity generation |
| `/api/v1/admin/server/tor/vanity/apply` | POST | Apply vanity address |
| `/api/v1/admin/server/tor/import` | POST | Import external keys |

### Admin - Web (robots.txt, security.txt) (`/api/v1/admin/server/web/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/web` | GET | Get web settings |
| `/api/v1/admin/server/web` | PATCH | Update web settings |
| `/api/v1/admin/server/web/robots` | GET | Get robots.txt config |
| `/api/v1/admin/server/web/robots` | PATCH | Update robots.txt |
| `/api/v1/admin/server/web/robots/preview` | GET | Preview robots.txt |
| `/api/v1/admin/server/web/security` | GET | Get security.txt config |
| `/api/v1/admin/server/web/security` | PATCH | Update security.txt |
| `/api/v1/admin/server/web/security/preview` | GET | Preview security.txt |

### Admin - Pages (`/api/v1/admin/server/pages/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/pages` | GET | Get all page settings |
| `/api/v1/admin/server/pages/about` | GET | Get about page content |
| `/api/v1/admin/server/pages/about` | PATCH | Update about page content |
| `/api/v1/admin/server/pages/privacy` | GET | Get privacy policy content |
| `/api/v1/admin/server/pages/privacy` | PATCH | Update privacy policy content |
| `/api/v1/admin/server/pages/contact` | GET | Get contact page settings |
| `/api/v1/admin/server/pages/contact` | PATCH | Update contact page settings |
| `/api/v1/admin/server/pages/help` | GET | Get help page content |
| `/api/v1/admin/server/pages/help` | PATCH | Update help page content |

### Admin - Email (`/api/v1/admin/server/email/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/email` | GET | Get email settings |
| `/api/v1/admin/server/email` | PATCH | Update email settings |
| `/api/v1/admin/server/email/test` | POST | Send test email |
| `/api/v1/admin/server/email/templates` | GET | List email templates |
| `/api/v1/admin/server/email/templates/{name}` | GET | Get template |
| `/api/v1/admin/server/email/templates/{name}` | PUT | Update template |
| `/api/v1/admin/server/email/templates/{name}/reset` | POST | Reset to default |
| `/api/v1/admin/server/email/templates/{name}/preview` | POST | Preview template |

### Admin - Scheduler (`/api/v1/admin/server/scheduler/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/scheduler` | GET | List scheduled tasks |
| `/api/v1/admin/server/scheduler/{id}` | GET | Get task details |
| `/api/v1/admin/server/scheduler/{id}` | PATCH | Update task |
| `/api/v1/admin/server/scheduler/{id}/run` | POST | Run task now |
| `/api/v1/admin/server/scheduler/{id}/enable` | POST | Enable task |
| `/api/v1/admin/server/scheduler/{id}/disable` | POST | Disable task |

### Admin - Backup (`/api/v1/admin/server/backup/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/backup` | GET | List backups |
| `/api/v1/admin/server/backup` | POST | Create backup |
| `/api/v1/admin/server/backup/{id}` | GET | Get backup details |
| `/api/v1/admin/server/backup/{id}` | DELETE | Delete backup |
| `/api/v1/admin/server/backup/{id}/download` | GET | Download backup file |
| `/api/v1/admin/server/backup/restore` | POST | Restore from backup |

### Admin - Logs (`/api/v1/admin/server/logs/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/logs` | GET | List log files |
| `/api/v1/admin/server/logs/{type}` | GET | Get log entries |
| `/api/v1/admin/server/logs/{type}/download` | GET | Download log file |

## Email Templates (User-Related)

| Template | Purpose |
|----------|---------|
| `user_welcome` | Welcome email after registration |
| `user_verify_email` | Email verification link |
| `user_password_reset` | Password reset link |
| `user_password_changed` | Confirmation of password change |
| `user_2fa_enabled` | Confirmation of 2FA enabled |
| `user_new_login` | Alert for login from new device/location |
| `user_invite` | Invitation email |
| `user_account_disabled` | Account has been disabled |

## Database Architecture (NON-NEGOTIABLE)

**Applications use TWO separate SQLite databases by default.**

### Database Files

| Database | File | Purpose |
|----------|------|---------|
| **Server DB** | `{data_dir}/db/server.db` | Admin credentials, server state, scheduler |
| **Users DB** | `{data_dir}/db/users.db` | User accounts, tokens, sessions (multi-user mode) |

### Why Two Databases?

| Reason | Description |
|--------|-------------|
| **Isolation** | Admin credentials separate from user data |
| **Security** | Compromised user DB doesn't expose admin |
| **Backup** | Can backup/restore independently |
| **Logical separation** | Server config vs user data |

### Database Modes

| Mode | Server DB | Users DB | Use Case |
|------|-----------|----------|----------|
| **Single Instance** | Local SQLite | Local SQLite | Default, simple deployment |
| **Cluster** | Remote (shared) | Remote (shared) | Multi-node, load distribution |
| **Mixed Mode** | Any supported | Any supported | Heterogeneous backend infrastructure |

#### Clustering vs High Availability (IMPORTANT TERMINOLOGY)

**These are NOT interchangeable terms.**

| Term | Meaning |
|------|---------|
| **Clustering** | Multiple application nodes sharing state via a common database and Valkey/Redis. Enables horizontal scaling and load distribution. All nodes are equal peers. |
| **High Availability (HA)** | Fault tolerance for the database backend itself (e.g., PostgreSQL replication, Galera for MariaDB). This is OUTSIDE the application's scope. |

**The application provides clustering (multi-node). Database HA is your infrastructure's responsibility.**

```
┌─────────────────────────────────────────────────────────────────┐
│                      APPLICATION LAYER                          │
│                                                                 │
│   ┌─────────┐     ┌─────────┐     ┌─────────┐                 │
│   │ Node 1  │     │ Node 2  │     │ Node 3  │  ← Clustering   │
│   └────┬────┘     └────┬────┘     └────┬────┘                 │
│        │               │               │                       │
│        └───────────────┼───────────────┘                       │
│                        │                                        │
│              ┌─────────▼─────────┐                             │
│              │  Valkey/Redis     │  ← Cluster coordination     │
│              └─────────┬─────────┘                             │
│                        │                                        │
└────────────────────────┼────────────────────────────────────────┘
                         │
┌────────────────────────┼────────────────────────────────────────┐
│                        ▼              INFRASTRUCTURE LAYER      │
│              ┌───────────────────┐                              │
│              │ Database Backend  │  ← HA is YOUR responsibility│
│              │ (PostgreSQL/MySQL)│                              │
│              └───────────────────┘                              │
│                                                                 │
│   Examples of HA (not our concern):                            │
│   - PostgreSQL: Patroni, pgpool, streaming replication          │
│   - MySQL/MariaDB: Galera Cluster, Group Replication           │
│   - Managed: AWS RDS Multi-AZ, CloudSQL HA                     │
└─────────────────────────────────────────────────────────────────┘
```

### Single Instance (Default)

Both databases are local SQLite files:

```yaml
server:
  database:
    driver: sqlite
    # Both databases stored locally
    # {data_dir}/db/server.db
    # {data_dir}/db/users.db
```

### Cluster Mode (Remote Database)

**When clustering, BOTH databases MUST use the same remote database.**

All nodes need shared access to:
- Admin credentials (same login works on any node)
- Server settings (consistent configuration)
- User accounts (users can access any node)
- Sessions (no re-login when switching nodes)
- API tokens (tokens work on any node)

```yaml
server:
  database:
    driver: postgres
    host: db.example.com
    port: 5432
    name: myapp
    username: myapp
    password: ${DB_PASSWORD}
    sslmode: require

    # All tables go in same database, different schemas/prefixes:
    # - server_* tables (admin_credentials, admin_sessions, scheduler_state)
    # - user_* tables (users, user_tokens, user_sessions, invites)
```

### Table Prefixes in Shared Database

| Prefix | Tables |
|--------|--------|
| `server_` | `server_admin_credentials`, `server_admin_sessions`, `server_scheduler_state` |
| `user_` | `user_accounts`, `user_tokens`, `user_sessions`, `user_invites` |

### Cluster Requirements

| Requirement | Description |
|-------------|-------------|
| **Shared database** | All nodes connect to same PostgreSQL/MySQL |
| **Same credentials** | Admin can log into any node |
| **Session sharing** | User sessions valid across all nodes |
| **Token validation** | API tokens work on any node |
| **Config sync** | Settings changes propagate to all nodes |

### Cluster Initialization (NON-NEGOTIABLE)

**The FIRST node is the source of truth. Subsequent nodes inherit from the remote database.**

#### First Node (Primary Setup)

```
1. Deploy first node
         │
         ▼
2. First run creates local SQLite databases
   - Admin credentials auto-generated
   - Server settings initialized
         │
         ▼
3. Configure application via admin panel
   - Set branding, SSL, etc.
   - Create users (if multi-user mode)
         │
         ▼
4. Ready to migrate to cluster?
         │
         ▼
5. Setup remote database (PostgreSQL/MySQL)
         │
         ▼
6. Update config: driver: postgres (or use Web UI)
         │
         ▼
7. Restart - app auto-migrates local data to remote
         │
         ▼
8. Now using remote DB
   (First node is source of truth)
```

#### Additional Nodes (Inherit from Remote)

```
1. Deploy new node
         │
         ▼
2. Configure with SAME remote database connection
   driver: postgres
   host: db.example.com (same as first node)
         │
         ▼
3. First run detects existing data in remote DB
         │
         ▼
4. SKIP initialization - inherit everything:
   - Admin credentials (same login)
   - Server settings (same config)
   - Users (same accounts)
   - Sessions (shared)
         │
         ▼
5. Node joins cluster automatically
```

#### Node Behavior

| Scenario | Behavior |
|----------|----------|
| **First node + SQLite** | Create admin, init settings |
| **First node + empty remote DB** | Create admin, init settings (becomes source of truth) |
| **New node + remote DB with data** | Skip init, inherit everything from DB |
| **Any node + settings change** | Write to DB, all nodes see change |

#### Migration Methods

**Migration to remote database is done via config file OR admin web UI - NO CLI commands.**

**Method 1: Config File**

```yaml
# server.yml - change driver and add connection details
server:
  database:
    driver: postgres
    host: db.example.com
    port: 5432
    name: myapp
    username: myapp
    password: ${DB_PASSWORD}
    sslmode: require
```

On restart, the application:
1. Detects driver change from `sqlite` to `postgres`
2. Connects to remote database
3. If empty: migrates local SQLite data automatically
4. If has data: uses existing data (node joins cluster)

**Method 2: Admin Web UI**

`/admin/server/database`

| Element | Type | Description |
|---------|------|-------------|
| Current Driver | Read-only | Shows current database driver |
| Driver | Dropdown | sqlite, postgres, mysql |
| Host | Text input | Database host (shown when not sqlite) |
| Port | Number input | Database port |
| Database Name | Text input | Database name |
| Username | Text input | Database username |
| Password | Password input | Database password |
| SSL Mode | Dropdown | disable, require, verify-full |
| Test Connection | Button | Verify connection before saving |
| Migrate & Switch | Button | Migrate data and switch to remote |

**Migration Process (automatic on driver change):**

```
1. Test connection to remote database
         │
         ▼
2. If remote DB empty:
   - Create tables
   - Export local SQLite data
   - Import to remote
         │
         ▼
3. If remote DB has data:
   - Verify schema compatibility
   - Skip migration (join existing cluster)
         │
         ▼
4. Switch to remote driver
         │
         ▼
5. Local SQLite files preserved as backup
```

### Storage Backend Sync (Bidirectional)

**Data always syncs TOWARD the new storage backend before switching.**

| From | To | Sync Action |
|------|-----|-------------|
| **SQLite → Remote** | Push to remote | Export SQLite data → Import to remote DB → Switch to remote |
| **Remote → SQLite** | Pull to local | Export remote data → Import to SQLite files → Disconnect remote → Switch to SQLite |
| **SQLite → YAML** | Export to file | Export `server.db` settings → Write to `server.yml` |
| **YAML → SQLite** | Import to DB | Read `server.yml` → Import settings to `server.db` |
| **YAML → Remote** | Import to DB | Read `server.yml` → Import settings to remote DB |

#### Remote → SQLite (Downgrade to Local)

When switching from remote database back to local SQLite:

```
1. User selects "Switch to SQLite" in Web UI
         │
         ▼
2. Export ALL data from remote database
         │
         ▼
3. Create/update local SQLite files:
   - {data_dir}/db/server.db
   - {data_dir}/db/users.db
         │
         ▼
4. Import remote data to SQLite
         │
         ▼
5. Verify data integrity (row counts match)
         │
         ▼
6. Disconnect from remote database
         │
         ▼
7. Update config: driver: sqlite
         │
         ▼
8. Now using local SQLite (standalone mode)
```

**Use case:** Converting cluster node back to standalone instance.

#### SQLite ↔ YAML Sync

Settings can be stored in database OR config file. Sync between them:

**SQLite → YAML (Export settings to file):**

```
Admin Web UI: /admin/server/settings → "Export to YAML"
         │
         ▼
Read all settings from server.db
         │
         ▼
Write to server.yml (preserves comments, formatting)
         │
         ▼
Settings now in both DB and YAML (DB is source of truth)
```

**YAML → SQLite (Import settings from file):**

```
On startup: YAML file detected with changes
         │
         ▼
Read settings from server.yml
         │
         ▼
Import to server.db (overwrites DB values)
         │
         ▼
DB is now source of truth
```

#### Sync Priority (Source of Truth)

| Mode | Source of Truth | Behavior |
|------|-----------------|----------|
| **Running** | Database | DB values used, YAML ignored |
| **Startup (YAML changed)** | YAML | YAML imported to DB, then DB is source |
| **Startup (YAML unchanged)** | Database | DB values used |
| **Cluster** | Remote Database | All nodes read from shared DB |

### Mixed Mode (Heterogeneous Database Backends)

**Mixed Mode allows different nodes in a cluster to connect to different database backends.**

This is useful when:
- Migrating from one database to another (e.g., MySQL → PostgreSQL)
- Geographic distribution with local database preferences
- Testing new database backend before full migration

#### Mixed Mode Configuration

Each node specifies its own database connection:

**Node 1 (PostgreSQL):**
```yaml
server:
  database:
    driver: postgres
    host: pg.us-east.example.com
    port: 5432
    name: myapp
    username: myapp
    password: ${DB_PASSWORD}
```

**Node 2 (MariaDB):**
```yaml
server:
  database:
    driver: mysql
    host: maria.eu-west.example.com
    port: 3306
    name: myapp
    username: myapp
    password: ${DB_PASSWORD}
```

**Node 3 (PostgreSQL - different server):**
```yaml
server:
  database:
    driver: postgres
    host: pg.ap-south.example.com
    port: 5432
    name: myapp
    username: myapp
    password: ${DB_PASSWORD}
```

#### Mixed Mode Requirements

| Requirement | Description |
|-------------|-------------|
| **Valkey/Redis REQUIRED** | State synchronization between heterogeneous backends |
| **Schema compatibility** | All databases MUST have identical schema |
| **Data sync** | Application handles cross-database replication via Valkey |
| **Same schema version** | All nodes must run same application version |

#### Mixed Mode Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                        CLUSTER NODES                                │
│                                                                     │
│  ┌──────────┐        ┌──────────┐        ┌──────────┐             │
│  │  Node 1  │        │  Node 2  │        │  Node 3  │             │
│  │ (US-East)│        │ (EU-West)│        │(AP-South)│             │
│  └────┬─────┘        └────┬─────┘        └────┬─────┘             │
│       │                   │                   │                    │
│       └───────────────────┼───────────────────┘                    │
│                           │                                         │
│                  ┌────────▼────────┐                               │
│                  │  Valkey/Redis   │  ← Sync layer (REQUIRED)      │
│                  │    Cluster      │                               │
│                  └────────┬────────┘                               │
│                           │                                         │
└───────────────────────────┼─────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│  PostgreSQL  │   │   MariaDB    │   │  PostgreSQL  │
│  (US-East)   │   │  (EU-West)   │   │  (AP-South)  │
└──────────────┘   └──────────────┘   └──────────────┘
```

#### Mixed Mode Sync via Valkey

**All state changes are broadcast through Valkey/Redis:**

```
Node 1 writes user record
         │
         ▼
Local PostgreSQL INSERT
         │
         ▼
Publish to Valkey: "user.created" + payload
         │
         ├──────────────────────────────────────┐
         │                                      │
         ▼                                      ▼
Node 2 receives                          Node 3 receives
         │                                      │
         ▼                                      ▼
Local MariaDB INSERT                     Local PostgreSQL INSERT
```

**The application handles:**
- Publishing changes to Valkey after local writes
- Subscribing to changes from other nodes
- Applying remote changes to local database
- Conflict resolution (last-write-wins by timestamp)

### Node Management (NON-NEGOTIABLE)

**Simple node management via admin web UI. No CLI commands, no manual config.**

#### Adding a Node

**On existing node:**

1. Go to `/admin/server/nodes/add`
2. Click "Generate Token" button
3. System generates:
   - Token: `node_{random_32_chars}`
   - URL: `{proto}://{fqdn}` (of this node)
4. Display token and URL to admin (copy buttons)

**On new server:**

1. Go to `/admin/server/nodes/add`
2. Enter:
   - URL from existing node
   - Token from existing node
3. Click "Join Cluster"
4. New node automatically:
   - Connects to existing node
   - Gets database connection details
   - Gets all configuration
   - Joins cluster

```
Existing Node                          New Node
     │                                      │
     ▼                                      │
/admin/server/nodes/add                     │
     │                                      │
     ▼                                      │
Generate Token                              │
  → node_abc123...                          │
  → https://node1.example.com               │
     │                                      │
     └──────── Share token + URL ──────────►│
                                            │
                                            ▼
                                   /admin/server/nodes/add
                                            │
                                            ▼
                                   Enter URL + Token
                                            │
                                            ▼
                                   Click "Join Cluster"
                                            │
                                            ▼
                                   Auto-configured!
```

#### Join Cluster Flow (Technical)

**Step-by-step process when new node joins cluster:**

```
NEW NODE                                    EXISTING NODE
    │                                            │
    │  1. POST /api/v1/cluster/join              │
    │     Body: { "token": "node_xxx" }          │
    │─────────────────────────────────────────►  │
    │                                            │
    │                                            ▼
    │                                   2. Validate token:
    │                                      - Token exists?
    │                                      - Not expired (15 min)?
    │                                      - Not already used?
    │                                      - Not in 90-day lockout?
    │                                            │
    │                                            ▼
    │                                   3. Mark token as used
    │                                            │
    │  4. Response: cluster bootstrap data       │
    │  ◄─────────────────────────────────────────│
    │                                            │
    ▼                                            │
5. Receive bootstrap data:                       │
   - Database connection (encrypted)             │
   - Encryption key for secrets                  │
   - Cluster ID                                  │
    │                                            │
    ▼                                            │
6. Connect to remote database                    │
    │                                            │
    ▼                                            │
7. Read all configuration from database          │
   - Server settings                             │
   - Branding/SEO                                │
   - SSL settings                                │
   - All other config                            │
    │                                            │
    ▼                                            │
8. Register self in nodes table                  │
   - node_id: {hostname}                         │
   - status: online                              │
   - joined_at: now()                            │
    │                                            │
    ▼                                            │
9. Write minimal server.yml                      │
   - Database connection only                    │
    │                                            │
    ▼                                            │
10. Start heartbeat                              │
    │                                            │
    ▼                                            │
11. Node is now part of cluster                  │
```

#### Bootstrap Data Structure

**Response from existing node (`POST /api/v1/cluster/join`):**

```json
{
  "success": true,
  "cluster": {
    "id": "cluster_abc123",
    "name": "MyApp Cluster"
  },
  "database": {
    "driver": "postgres",
    "host": "db.example.com",
    "port": 5432,
    "name": "myapp",
    "username": "myapp",
    "password_encrypted": "base64_encrypted_password",
    "sslmode": "require"
  },
  "encryption": {
    "key_encrypted": "base64_encrypted_key"
  },
  "node": {
    "suggested_id": "server-2"
  }
}
```

#### Security: Encrypted Transfer

**Sensitive data is encrypted during transfer using token-derived key:**

```go
// Token is used to derive encryption key for bootstrap data
func deriveKeyFromToken(token string) []byte {
    // Use Argon2id to derive encryption key from token
    salt := sha256.Sum256([]byte("cluster_join_" + token))
    return argon2.IDKey([]byte(token), salt[:16], 3, 64*1024, 4, 32)
}

// Existing node encrypts sensitive data
func encryptBootstrapData(data []byte, token string) []byte {
    key := deriveKeyFromToken(token)
    // AES-256-GCM encryption
    return aesGcmEncrypt(data, key)
}

// New node decrypts using same token
func decryptBootstrapData(encrypted []byte, token string) []byte {
    key := deriveKeyFromToken(token)
    return aesGcmDecrypt(encrypted, key)
}
```

**What's encrypted:**
- Database password
- Encryption key for secrets (TOTP secrets, etc.)

**What's NOT encrypted (not sensitive):**
- Database host/port/name
- Cluster ID
- Node suggestions

#### Database Tables for Clustering

**Nodes Table (in remote database):**

| Column | Type | Description |
|--------|------|-------------|
| `id` | String | Node ID (default: hostname) |
| `hostname` | String | Server hostname |
| `ip_address` | String | Node IP address |
| `version` | String | Application version |
| `status` | String | online, offline, degraded |
| `last_heartbeat` | Timestamp | Last heartbeat time |
| `joined_at` | Timestamp | When node joined cluster |
| `system_info` | JSON | CPU, memory, disk stats |

**Join Tokens Table (in remote database):**

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `token_hash` | String | SHA-256 hash of token |
| `created_by` | String | Node ID that created token |
| `created_at` | Timestamp | Creation time |
| `expires_at` | Timestamp | Expiration (created_at + 15 min) |
| `used_at` | Timestamp | When token was used (NULL if unused) |
| `used_by` | String | Node ID that used token |
| `lockout_until` | Timestamp | Cannot reuse until (used_at + 90 days) |

#### Heartbeat

**Nodes send heartbeat every 30 seconds:**

```go
func startHeartbeat(ctx context.Context, db *sql.DB, nodeID string) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            updateHeartbeat(db, nodeID)
        }
    }
}

func updateHeartbeat(db *sql.DB, nodeID string) {
    db.Exec(`
        UPDATE nodes
        SET last_heartbeat = NOW(),
            status = 'online',
            system_info = $1
        WHERE id = $2
    `, getSystemInfo(), nodeID)
}
```

**Node status determination:**

| Last Heartbeat | Status |
|----------------|--------|
| < 1 minute ago | online |
| 1-5 minutes ago | degraded |
| > 5 minutes ago | offline |

#### Error Handling

| Error | Response | Action |
|-------|----------|--------|
| Invalid token | 401 Unauthorized | Show error, ask to retry |
| Token expired | 401 Token Expired | Generate new token on existing node |
| Token already used | 401 Token Already Used | Generate new token |
| Token in lockout | 401 Token Locked | Wait 90 days or generate new token |
| Database connection failed | 500 Database Error | Show error, check DB settings |
| Network error | Connection Failed | Retry with backoff |

#### Token Rules

| Rule | Value |
|------|-------|
| Format | `node_{random_32_chars}` |
| Valid for | 15 minutes |
| Usage | One-time only |
| Reuse lockout | 90 days (prevents replay attacks) |

#### Removing a Node

**A node can only remove ITSELF from the cluster.**

1. Go to `/admin/server/nodes/remove` (on the node to remove)
2. Click "Remove from Cluster"
3. Confirmation modal: "Are you sure you want to remove {nodename} from the cluster?"
   - [Yes, Remove] [Cancel]
4. On confirmation:
   - Node disconnects from cluster
   - Node reverts to standalone mode (local SQLite)
   - Node creates fresh local databases
   - Other nodes see removal, adjust node count

**Cannot remove other nodes** - prevents accidental/malicious removal.

#### Viewing Nodes

**`/admin/server/nodes`** - Node list

| Column | Description |
|--------|-------------|
| Node Name | Hostname (clickable link) |
| Status | online, offline, degraded |
| Uptime | Time since last restart |
| Version | Application version |
| Last Seen | Last heartbeat time |

**`/admin/server/nodes/{node}`** - Node detail

| Section | Contents |
|---------|----------|
| **Info** | Node name, hostname, IP, version, uptime |
| **Status** | Connection status, last heartbeat |
| **System** | CPU usage, memory usage, disk usage |
| **Database** | Connection status, query latency |
| **Tor** | Hidden service status, .onion address |

#### Node Identity

| Setting | Default | Changeable |
|---------|---------|------------|
| Node ID | `{hostname}` | Yes, via `/admin/server/nodes/settings` |
| Display Name | `{hostname}` | Yes |

#### First Node Behavior

**Generating the first token automatically converts to cluster mode.**

```
Single Instance (SQLite)
         │
         ▼
Admin goes to /admin/server/nodes/add
         │
         ▼
Clicks "Generate Token"
         │
         ▼
System prompts: "Enable cluster mode?"
  - Requires remote database connection
  - Enter PostgreSQL/MySQL details
         │
         ▼
On confirm:
  - Migrates local data to remote DB
  - Generates join token
  - Node is now cluster-ready
```

#### Admin Panel Routes

| Route | Description |
|-------|-------------|
| `/admin/server/nodes` | List all nodes |
| `/admin/server/nodes/add` | Add node (generate token OR join) |
| `/admin/server/nodes/remove` | Remove THIS node from cluster |
| `/admin/server/nodes/settings` | Node identity settings |
| `/admin/server/nodes/{node}` | View specific node details |

#### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/nodes` | GET | List all nodes |
| `/api/v1/admin/server/nodes/token` | POST | Generate join token |
| `/api/v1/admin/server/nodes/join` | POST | Join cluster with token |
| `/api/v1/admin/server/nodes/leave` | POST | Remove this node from cluster |
| `/api/v1/admin/server/nodes/{node}` | GET | Get node details |
| `/api/v1/admin/server/nodes/settings` | GET | Get node identity settings |
| `/api/v1/admin/server/nodes/settings` | PATCH | Update node identity |

### Supported Remote Databases

| Database | Driver | Pure Go | Notes |
|----------|--------|---------|-------|
| PostgreSQL | `github.com/lib/pq` | YES | Recommended for production |
| MySQL/MariaDB | `github.com/go-sql-driver/mysql` | YES | Widely supported |
| SQLite | `modernc.org/sqlite` | YES | Single instance only |

### Schema Migration

When using remote database, the same tables are created but with appropriate types:

| SQLite Type | PostgreSQL Type | MySQL Type |
|-------------|-----------------|------------|
| TEXT | TEXT | VARCHAR(255) / TEXT |
| INTEGER | INTEGER / BIGINT | INT / BIGINT |
| BOOLEAN | BOOLEAN | TINYINT(1) |
| TIMESTAMP | TIMESTAMPTZ | DATETIME |
| UUID | UUID | CHAR(36) |
| JSON | JSONB | JSON |

## Database Schema

### Server Database Tables (server.db)

#### Admin Credentials Table

**Stores server admin credentials - NEVER in config file.**

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER | Primary key (always 1 - single admin) |
| `username` | String | Admin username |
| `email` | String | Admin email |
| `password_hash` | String | Argon2id hash (PHC format) |
| `token_hash` | String | SHA-256 hash of API token |
| `token_prefix` | String | First 8 chars of token (for identification) |
| `totp_secret` | String | 2FA secret (encrypted, optional) |
| `totp_enabled` | Boolean | 2FA enabled |
| `created_at` | Timestamp | Account creation |
| `updated_at` | Timestamp | Last update |
| `last_login_at` | Timestamp | Last login |

**Notes:**
- Only ONE row ever exists (id=1)
- Created via setup wizard on first run
- Setup token displayed in console ONCE, used to access `/admin/server/setup`
- Admin password and API token created during setup wizard (user must copy)

#### Admin Sessions Table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `token_hash` | String | SHA-256 hash of session token |
| `ip_address` | String | Client IP |
| `user_agent` | String | Browser/client info |
| `location` | String | GeoIP location |
| `expires_at` | Timestamp | Session expiry |
| `created_at` | Timestamp | Session start |

#### Scheduler State Table

| Column | Type | Description |
|--------|------|-------------|
| `task_id` | String | Unique task identifier |
| `last_run` | Timestamp | Last execution time |
| `next_run` | Timestamp | Next scheduled time |
| `last_result` | String | success/failure |
| `last_error` | Text | Error message if failed |
| `run_count` | Integer | Total runs |
| `enabled` | Boolean | Task enabled |

### Users Database Tables (users.db)

#### Users Table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `email` | String | Unique, required |
| `password_hash` | String | Argon2id hash (PHC format) |
| `display_name` | String | Optional |
| `avatar_url` | String | Optional |
| `bio` | Text | Optional |
| `role` | String | User role |
| `email_verified` | Boolean | Email verified status |
| `approved` | Boolean | Admin approved (if required) |
| `disabled` | Boolean | Account disabled |
| `totp_secret` | String | 2FA secret (encrypted) |
| `totp_enabled` | Boolean | 2FA enabled |
| `timezone` | String | User timezone |
| `language` | String | User language |
| `created_at` | Timestamp | Account creation |
| `updated_at` | Timestamp | Last update |
| `last_login_at` | Timestamp | Last login |

#### User Tokens Table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `user_id` | UUID | Foreign key to users |
| `name` | String | User-defined label |
| `token_hash` | String | SHA-256 hash of API token |
| `token_prefix` | String | First 8 chars (for identification) |
| `scopes` | JSON | Optional permission scopes |
| `expires_at` | Timestamp | Optional expiration |
| `last_used_at` | Timestamp | Last usage |
| `created_at` | Timestamp | Creation time |

#### User Sessions Table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `user_id` | UUID | Foreign key to users |
| `token_hash` | String | SHA-256 hash of session token |
| `ip_address` | String | Client IP |
| `user_agent` | String | Browser/client info |
| `location` | String | GeoIP location |
| `expires_at` | Timestamp | Session expiry |
| `created_at` | Timestamp | Session start |

#### Invites Table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `code` | String | Unique invite code |
| `role` | String | Role for invited users |
| `max_uses` | Integer | Maximum uses (0 = unlimited) |
| `use_count` | Integer | Current use count |
| `expires_at` | Timestamp | Expiration |
| `created_by` | UUID | Admin who created |
| `created_at` | Timestamp | Creation time |

---

# PART 32: TOR HIDDEN SERVICE (NON-NEGOTIABLE)

## Overview

**ALL projects MUST have built-in Tor hidden service support.**

Tor integration uses **external Tor binary** via `github.com/cretz/bine`. This maintains `CGO_ENABLED=0` compatibility for static binaries while providing full Tor hidden service functionality.

## Configuration

```yaml
server:
  tor:
    enabled: true  # Default: enabled
    # Path to Tor binary (auto-detected if empty)
    binary: ""
```

**Notes:**
- Uses external Tor binary (not embedded) for CGO_ENABLED=0 compatibility
- Enabled by default - privacy out of the box
- .onion address derived from keys in `{data_dir}/tor/site/`
- Application manages Tor process lifecycle

## Tor Process Management (NON-NEGOTIABLE)

**The application MUST start its OWN dedicated Tor process. NEVER use system Tor.**

This prevents conflicts with any existing Tor installation on the system.

```
1. Find Tor binary:
   ├─ Check config `server.tor.binary` path
   ├─ Check PATH for `tor` executable
   ├─ Check common locations:
   │   ├─ Linux: /usr/bin/tor, /usr/local/bin/tor
   │   ├─ macOS: /usr/local/bin/tor, /opt/homebrew/bin/tor
   │   ├─ Windows: C:\Program Files\Tor\tor.exe
   │   └─ BSD: /usr/local/bin/tor
   └─ NOT FOUND: Log warning, disable Tor features, continue without Tor

2. Start DEDICATED Tor process:
   ├─ Use application's own DataDir: `{data_dir}/tor/`
   ├─ Use random available ControlPort (not 9051)
   ├─ Use random available SocksPort (not 9050)
   ├─ Completely isolated from system Tor
   ├─ Wait for bootstrap completion
   └─ Create hidden service via ADD_ONION

3. On application shutdown:
   └─ Terminate the dedicated Tor process
```

### Why Dedicated Tor Process?

| Reason | Description |
|--------|-------------|
| **No conflicts** | System Tor uses 9050/9051, we use random ports |
| **Isolation** | Our DataDir is separate from system Tor |
| **Clean shutdown** | We control the process lifecycle |
| **No permissions issues** | Don't need access to system Tor control |
| **Predictable behavior** | Always same configuration |

## Implementation

### Library

Use `github.com/cretz/bine` (pure Go, CGO_ENABLED=0 compatible):

```go
import (
    "github.com/cretz/bine/tor"
)

func startDedicatedTor(ctx context.Context, localPort int) (*tor.Tor, *tor.OnionService, error) {
    // Start OUR OWN Tor process - completely separate from system Tor
    t, err := tor.Start(ctx, &tor.StartConf{
        // Our own data directory - isolated from system Tor
        DataDir: paths.GetDataDir() + "/tor",

        // Let bine pick available ports (avoids conflict with system Tor 9050/9051)
        // These are set automatically to available ports
        NoAutoSocksPort: false,

        // Optional: specify path if not in PATH
        // ExePath: "/usr/bin/tor",

        // Debug output (development only)
        // DebugWriter: os.Stderr,
    })
    if err != nil {
        return nil, nil, fmt.Errorf("failed to start dedicated tor: %w", err)
    }

    // Wait for Tor to bootstrap
    dialCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
    defer cancel()
    if err := t.EnableNetwork(dialCtx, true); err != nil {
        t.Close()
        return nil, nil, fmt.Errorf("failed to enable tor network: %w", err)
    }

    // Create hidden service
    onion, err := t.Listen(ctx, &tor.ListenConf{
        RemotePorts: []int{80},
        LocalPort:   localPort,
    })
    if err != nil {
        t.Close()
        return nil, nil, fmt.Errorf("failed to create onion service: %w", err)
    }

    // onion.ID contains the .onion address (without .onion suffix)
    log.Printf("Tor hidden service started: %s.onion", onion.ID)
    return t, onion, nil
}

// Shutdown cleanly terminates our dedicated Tor process
func shutdownTor(t *tor.Tor) error {
    if t != nil {
        return t.Close()
    }
    return nil
}
```

### Port Allocation

| Port | System Tor | Our Tor |
|------|------------|---------|
| SocksPort | 9050 | Random available |
| ControlPort | 9051 | Random available |
| DataDir | `/var/lib/tor` | `{data_dir}/tor/` |

**bine automatically selects available ports**, ensuring no conflict with system Tor.

### Tor Configuration Optimizations (NON-NEGOTIABLE)

**Tor is used ONLY for hidden services. Optimize accordingly.**

```go
// Optimized torrc settings for hidden-service-only mode
func getTorConfig() string {
    return `
# Hidden service only - not a relay or exit
SocksPort 0
# No SOCKS proxy needed - we're server only

# Disable unused features
ExitRelay 0
ExitPolicy reject *:*
# Never act as exit node

# Don't relay traffic for others
ORPort 0
DirPort 0

# Reduce circuit building (we only need service circuits)
MaxCircuitDirtiness 600
# Keep circuits longer

# Reduce bandwidth for Tor overhead
BandwidthRate 1 MB
BandwidthBurst 2 MB

# Hidden service optimizations
HiddenServiceSingleHopMode 0
# Keep full anonymity (3 hops)

# Faster startup
FetchDirInfoEarly 1
FetchDirInfoExtraEarly 1

# Reduce memory usage
DisableDebuggerAttachment 1
`
}
```

| Setting | Value | Reason |
|---------|-------|--------|
| `SocksPort 0` | Disabled | Not browsing, server only |
| `ExitRelay 0` | Disabled | Not an exit node |
| `ORPort 0` | Disabled | Not relaying traffic |
| `DirPort 0` | Disabled | Not a directory server |
| `ExitPolicy reject *:*` | Block all | Extra safety |
| `MaxCircuitDirtiness 600` | 10 minutes | Keep circuits longer |

### Tor Process Lifecycle (NON-NEGOTIABLE)

**The application MUST fully manage the Tor process lifecycle.**

| Event | Action |
|-------|--------|
| **App Start** | Find Tor binary → Start dedicated Tor process → Wait for bootstrap → Create hidden service |
| **App Running** | Monitor Tor process, restart if crashed |
| **App Shutdown** | Terminate Tor process gracefully (SIGTERM) |
| **App Crash** | Tor process should terminate (child process dies with parent) |
| **SIGTERM/SIGINT** | Graceful shutdown: stop Tor, then exit |
| **SIGHUP** | Reload config, restart Tor if settings changed |

### Tor Restart Triggers (NON-NEGOTIABLE)

**Tor MUST be restarted when these events occur:**

| Trigger | Action | Notes |
|---------|--------|-------|
| **Regenerate .onion address** | Stop Tor → Delete keys → Start Tor | New random address |
| **Apply vanity address** | Stop Tor → Replace keys → Start Tor | Use generated vanity keys |
| **Import external keys** | Stop Tor → Replace keys → Start Tor | Use imported keys |
| **Enable Tor** | Start Tor | Was disabled |
| **Disable Tor** | Stop Tor | User disabled |
| **Tor process crash** | Restart Tor | Auto-recovery |
| **Tor unresponsive** | Stop Tor → Start Tor | Health check failed |
| **Config change** | Stop Tor → Start Tor | Settings changed |

### Restart Implementation

```go
// TorManager handles all Tor lifecycle operations
type TorManager struct {
    mu        sync.Mutex
    tor       *tor.Tor
    onion     *tor.OnionService
    dataDir   string
    localPort int
    ctx       context.Context
    cancel    context.CancelFunc
}

// Restart stops and starts Tor (used for config changes, recovery)
func (tm *TorManager) Restart() error {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    // Stop existing
    if tm.tor != nil {
        tm.tor.Close()
        tm.tor = nil
        tm.onion = nil
    }

    // Start fresh
    return tm.startLocked()
}

// RegenerateAddress creates a new random .onion address
func (tm *TorManager) RegenerateAddress() (string, error) {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    // Stop Tor
    if tm.tor != nil {
        tm.tor.Close()
        tm.tor = nil
        tm.onion = nil
    }

    // Delete existing keys
    keysDir := filepath.Join(tm.dataDir, "site")
    if err := os.RemoveAll(keysDir); err != nil {
        return "", fmt.Errorf("failed to remove old keys: %w", err)
    }

    // Start Tor - new keys will be generated
    if err := tm.startLocked(); err != nil {
        return "", err
    }

    return tm.onion.ID + ".onion", nil
}

// ApplyKeys stops Tor, replaces keys, and restarts
func (tm *TorManager) ApplyKeys(privateKey []byte) (string, error) {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    // Stop Tor
    if tm.tor != nil {
        tm.tor.Close()
        tm.tor = nil
        tm.onion = nil
    }

    // Write new keys
    keysDir := filepath.Join(tm.dataDir, "site")
    os.MkdirAll(keysDir, 0700)
    keyPath := filepath.Join(keysDir, "hs_ed25519_secret_key")
    if err := os.WriteFile(keyPath, privateKey, 0600); err != nil {
        return "", fmt.Errorf("failed to write key: %w", err)
    }

    // Start Tor with new keys
    if err := tm.startLocked(); err != nil {
        return "", err
    }

    return tm.onion.ID + ".onion", nil
}

// SetEnabled enables or disables Tor
func (tm *TorManager) SetEnabled(enabled bool) error {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    if enabled {
        if tm.tor == nil {
            return tm.startLocked()
        }
    } else {
        if tm.tor != nil {
            tm.tor.Close()
            tm.tor = nil
            tm.onion = nil
        }
    }
    return nil
}
```

### Signal Handling with Tor

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start Tor
    torProcess, onion, err := startDedicatedTor(ctx, localPort)
    if err != nil {
        log.Printf("Warning: Tor disabled - %v", err)
        // Continue without Tor
    }

    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigChan
        log.Println("Shutting down...")

        // Stop Tor FIRST
        if torProcess != nil {
            log.Println("Stopping Tor process...")
            torProcess.Close()
        }

        // Then cancel context for other goroutines
        cancel()
    }()

    // Run server...
}
```

### Tor Process Monitoring

```go
// Monitor Tor and restart if it crashes
func monitorTor(ctx context.Context, torProcess *tor.Tor, restartFunc func() (*tor.Tor, error)) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Check if Tor is still responsive
            if torProcess != nil {
                // Ping control connection
                if _, err := torProcess.Control.GetInfo("version"); err != nil {
                    log.Println("Tor process unresponsive, restarting...")
                    torProcess.Close()
                    newTor, err := restartFunc()
                    if err != nil {
                        log.Printf("Failed to restart Tor: %v", err)
                    } else {
                        torProcess = newTor
                    }
                }
            }
        }
    }
}
```

### Binary Size

No impact on binary size - Tor is external. Application binary remains small and static.

### Data Storage

| Data | Location |
|------|----------|
| Tor data directory | `{data_dir}/tor/` |
| Hidden service keys | `{data_dir}/tor/site/` |
| Tor process PID | `{data_dir}/tor/tor.pid` |

## Admin Panel

### /admin/server/tor (Web UI)

| Element | Type | Description |
|---------|------|-------------|
| Enable Tor | Toggle switch | Turn hidden service on/off |
| Status | Indicator | ● Connected / ○ Disconnected / ⚠ Error |
| .onion Address | Read-only text | Full address with copy button |
| Regenerate Address | Button | Creates new random .onion (requires confirmation modal) |
| Vanity Prefix | Text input | Desired prefix (max 6 characters) |
| Generate Vanity | Button | Starts background generation |
| Vanity Status | Progress indicator | Shows when generating in background |
| Import Keys | File upload | Import externally generated keys |

**Status Card Example:**
```
┌─────────────────────────────────────────────────────────┐
│ Tor Hidden Service                                      │
│                                                         │
│ Status: ● Connected                                     │
│ Address: abcd1234...wxyz.onion                  [Copy]  │
│                                                         │
│ [Regenerate Address]                                    │
├─────────────────────────────────────────────────────────┤
│ Vanity Address                                          │
│                                                         │
│ Prefix: [______] (max 6 chars)  [Generate]              │
│                                                         │
│ ⏳ Generating: "jokes" - 2h 15m elapsed...              │
│    [Cancel]                                             │
├─────────────────────────────────────────────────────────┤
│ Import External Keys                      [Import Keys] │
│ ⓘ Help: How to generate longer vanity addresses        │
└─────────────────────────────────────────────────────────┘
```

### Vanity Address Generation

**Built-in generation (max 6 characters):**

| Prefix Length | Approximate Time |
|---------------|------------------|
| 1-4 chars | Seconds to minutes |
| 5 chars | Minutes to hours |
| 6 chars | Hours to days |

**Behavior:**
- Generation runs in background
- Current .onion address remains active while generating
- Notification sent when vanity address is ready
- User clicks notification or "Apply" button to activate
- Old keys deleted, new vanity keys activated
- Tor restarts with new address

### External Vanity Generation (7+ characters)

For prefixes longer than 6 characters, use external tools with GPU acceleration. The admin panel includes documentation (expandable help section):

**Using mkp224o (CPU):**
```bash
# Install
git clone https://github.com/cathugger/mkp224o
cd mkp224o && ./autogen.sh && ./configure && make

# Generate (example: 7-char prefix "myapp12")
./mkp224o -d ./keys myapp12

# Output: ./keys/myapp12xxxxx.onion/
#   ├── hostname        # Your .onion address
#   ├── hs_ed25519_public_key
#   └── hs_ed25519_secret_key
```

**Using mkp224o (GPU - much faster):**
```bash
# With CUDA support
./configure --enable-cuda
make

# Generate
./mkp224o -d ./keys myapp12
```

**Importing keys:**
1. Generate keys using mkp224o or similar tool
2. In admin panel, click "Import Keys"
3. Upload `hs_ed25519_secret_key` file (or zip containing both key files)
4. Confirm to replace current address
5. Tor restarts with imported keys

**Time estimates for longer prefixes:**

| Prefix Length | CPU Time | GPU Time |
|---------------|----------|----------|
| 7 chars | Days to weeks | Hours to days |
| 8 chars | Weeks to months | Days to weeks |
| 9+ chars | Months to years | Weeks to months |

**Security Notes:**
- .onion address shown only after admin authentication
- "Regenerate Address" requires confirmation modal (destructive - old address stops working)
- Address regeneration logged to audit log
- Imported keys should be generated on a trusted machine
- Delete source key files after successful import

### API Endpoints

**See PART 28 for full API route definitions under `/api/v1/admin/server/tor/`**

### Response Format

```json
{
  "enabled": true,
  "status": "connected",
  "onion_address": "abcd1234efgh5678ijkl9012mnop3456qrst7890uvwx.onion",
  "uptime": "2d 5h 30m"
}
```

## Behavior

| Scenario | Behavior |
|----------|----------|
| First run | Tor starts, generates .onion address, saves to config |
| Subsequent runs | Tor starts, uses existing .onion address |
| Disabled in config | Tor does not start, no .onion available |
| Regenerate address | Old keys deleted, new .onion generated, config updated |
| Network issues | Tor retries connection automatically |

## CLI

The `--status` command includes Tor and cluster status:

### Single Instance

```
$ myapp --status

Server Status: Running
  Port: 8080
  Mode: production
  Uptime: 2d 5h 30m

Node: standalone
Cluster: disabled

Tor Hidden Service: Connected
  Address: abcd1234...wxyz.onion
```

### Cluster Mode

```
$ myapp --status

Server Status: Running
  Port: 8080
  Mode: production
  Uptime: 2d 5h 30m

Node: node-abc123
  Hostname: server-1.example.com

Cluster: connected
  Status: healthy
  Nodes: 3
  Database: postgres://db.example.com/myapp

Tor Hidden Service: Connected
  Address: abcd1234...wxyz.onion
```

### Status Fields

| Field | Description |
|-------|-------------|
| Node | Node ID (standalone or unique ID) |
| Hostname | Server hostname |
| Cluster | disabled, connected, degraded, disconnected |
| Nodes | Number of nodes in cluster |
| Database | Database connection info (driver://host/db) |

---

# PART 33: AI ASSISTANT RULES (NON-NEGOTIABLE)

**These rules govern how AI assistants work on projects using this specification.**

## CRITICAL: Specification Drift is Unacceptable

**AI assistants drift from specifications over time.** This means:
- Gradually deviating from documented requirements
- Inventing new patterns not in the spec
- Forgetting constraints after working for a while
- "Improving" things that don't need improvement

**This drift is the #1 cause of specification violations. Combat it actively.**

## Template File vs AI.md

| File | Action | Description |
|------|--------|-------------|
| **Template file** | **NEVER MODIFY** | Read-only master (may be named TEMPLATE.md, SPEC.md, etc.) |
| **AI.md** | Create/Update | Project-specific specification |
| **TODO.AI.md** | Create/Update | Task tracking |

**How to identify the template file:**
- Location varies: project root, org root, different org, ~/, or user-specified
- Contains unreplaced `airports`, `apimgr` variables
- Contains "HOW TO USE THIS TEMPLATE" section
- Contains multiple "PART X:" numbered sections

**Workflow:**
1. If `AI.md` doesn't exist → Copy from template, replace variables
2. If `AI.md` exists → Read it, update if outdated
3. If old spec files exist in project repo → Merge into `AI.md`, DELETE old files

## Mandatory Compliance Schedule

| When | Action | Purpose |
|------|--------|---------|
| **Session start** | Read AI.md completely | Understand full context |
| **Before EACH task** | Re-read relevant PART(s) | Prevent drift |
| **Every 3-5 changes** | Stop, verify against spec | Catch drift early |
| **Before task completion** | Full compliance check | Ensure correctness |
| **When uncertain** | Re-read spec or ASK | Never guess |

**You MUST re-read the spec before implementing. Do NOT rely on memory.**

## Before Starting Work

1. **Identify template vs AI.md** - template is read-only, AI.md is project spec
2. **NEVER modify template file** - location varies, but it's always read-only
3. **Read AI.md COMPLETELY** - not just parts you think are relevant
4. **If AI.md missing** - create from template, replace all variables
5. **If AI.md outdated** - update to reflect current template
6. **Check TODO.AI.md** - see pending tasks and their priority
7. **Verify understanding** - if ANYTHING is unclear, ASK first
8. **Never assume** - when in doubt, ask the user

## During Work

1. **Re-read spec before EACH implementation** - every single time
2. **Follow spec EXACTLY** - no "improvements" without explicit permission
3. **Check yourself every 3-5 changes** - am I drifting?
4. **Update TODO.AI.md** as tasks are completed
5. **Test your changes** - don't commit untested code
6. **Keep changes focused** - one feature/fix per task
7. **If uncertain** - STOP, re-read spec, or ASK

## After Work

1. **Update AI.md** if architecture or rules changed
2. **Update TODO.AI.md** with any new tasks discovered
3. **Verify compliance** - check against the FINAL CHECKPOINT
4. **NEVER modify template file** - changes go in AI.md only

## Common Drift Patterns to Avoid

| Drift Pattern | What You Think | What You Should Do |
|---------------|----------------|-------------------|
| Pattern improvement | "This pattern is better" | Use the pattern in spec |
| Format change | "This format is cleaner" | Use the format in spec |
| Memory reliance | "I remember it says..." | Re-read the actual spec |
| Assumption | "Obviously they want..." | Check spec, ask if unclear |
| Feature addition | "This would be helpful" | Only do what's requested |
| Spec correction | "The spec is wrong here" | Ask before any deviation |

## Attribution

| Rule | Description |
|------|-------------|
| **NEVER** | Include AI attribution in code, comments, commits, or documentation |
| **NEVER** | "authored by Claude", "generated by AI", "Co-Authored-By: Claude" |
| **ALWAYS** | Code should appear as if written by a human developer |

## Tool Access

| Access | Description |
|--------|-------------|
| Full | All tools available |
| **EXCEPT** | `git push`, `git commit` - require explicit user request |
| Allowed | Stage files, create branches, check status, diff |
| Required | User must explicitly request commits and pushes |

## Prohibited Actions

| Action | Reason |
|--------|--------|
| **Modifying TEMPLATE.md** | **Read-only master template - NEVER modify** |
| `git push` without asking | May push unreviewed code |
| `git commit` without asking | User should review changes first |
| Deleting files without confirmation | Destructive action |
| Changing NON-NEGOTIABLE sections | Specification violation |
| Skipping validation | Security requirement |
| Hardcoding secrets | Security vulnerability |
| Using deprecated APIs | Maintainability issue |

## Code Style Rules (NON-NEGOTIABLE)

### Comment Placement

**Comments MUST always be placed ABOVE the code they describe. NEVER inline or below.**

| Placement | Allowed |
|-----------|---------|
| Above code | YES |
| Inline (same line) | NO |
| Below code | NO |

**Correct:**
```go
// Calculate the total price including tax
total := price * (1 + taxRate)

// User configuration options
type Config struct {
    // Server port number
    Port int
    // Enable debug mode
    Debug bool
}
```

**Incorrect:**
```go
total := price * (1 + taxRate) // Calculate total price - WRONG

total := price * (1 + taxRate)
// Calculate total price - WRONG (below)

type Config struct {
    Port int  // Server port - WRONG (inline)
    Debug bool // Debug mode - WRONG (inline)
}
```

**YAML comments - same rule:**
```yaml
# Enable multi-user mode
enabled: false

# Allow public registration
registration:
  enabled: false
```

### Code Quality Rules

| Rule | Description |
|------|-------------|
| **No magic numbers** | Use named constants |
| **No hardcoded strings** | Use constants or config |
| **Error handling** | Always handle errors, never ignore |
| **Input validation** | Validate ALL user input |
| **SQL injection** | Use parameterized queries only |
| **XSS prevention** | Escape all output in templates |
| **CSRF protection** | All forms must have CSRF tokens |

### File Organization

| Rule | Description |
|------|-------------|
| **One package per directory** | Standard Go convention |
| **Meaningful names** | `user.go` not `u.go` |
| **Group related code** | Keep related functions together |
| **Separate concerns** | Don't mix handlers with business logic |

## Handling Ambiguity

When the specification is unclear:

1. **Check if clarified elsewhere** - search the full spec
2. **Look for similar patterns** - how are similar features handled?
3. **Ask the user** - don't guess
4. **Document the decision** - add to AI.md for future reference

## Common Mistakes to Avoid

| Mistake | Correct Approach |
|---------|------------------|
| Implementing without reading spec | Read relevant PART first |
| Assuming default values | Check spec for defined defaults |
| Using .yaml instead of .yml | Always use `server.yml` |
| Inline comments | Comments above code only |
| Skipping admin panel | ALL settings need admin UI |
| Forgetting mobile-first | Start with mobile, expand to desktop |
| Using JavaScript alerts | Use proper notification system |
| Inline CSS | Use CSS files/classes only |

---

# FINAL CHECKPOINT: COMPLIANCE CHECKLIST

**Before starting ANY work, verify you have read and understood:**

## Core Requirements

- [ ] **NEVER modify TEMPLATE.md** - it is read-only
- [ ] AI.md is the project specification (create from TEMPLATE.md if missing)
- [ ] AI.md must be kept in sync with project state and TEMPLATE.md updates
- [ ] TODO.AI.md required for more than 2 tasks
- [ ] Migrate old files: `CLAUDE.md`, `SPEC.md` → merge into `AI.md`, DELETE old
- [ ] Never assume or guess - ask questions

## Development

- [ ] All 4 OSes supported (Linux, macOS, BSD, Windows)
- [ ] Both architectures supported (AMD64, ARM64)
- [ ] CGO_ENABLED=0 for static binaries
- [ ] Single static binary with embedded assets

## Configuration

- [ ] Config file is `server.yml` (not .yaml)
- [ ] Boolean handling accepts all truthy/falsy values
- [ ] Sane defaults for everything

## Frontend

- [ ] Frontend required for ALL projects
- [ ] NO inline CSS
- [ ] NO JavaScript alerts
- [ ] Dark theme (Dracula) is default
- [ ] Mobile-first responsive design

## API

- [ ] All 3 API types: REST, Swagger, GraphQL (Swagger & GraphQL in sync)
- [ ] Standard endpoints exist (/healthz, /openapi, /openapi.json, /graphql, /admin)
- [ ] OpenAPI uses JSON only (no YAML)
- [ ] Versioned API: /api/v1

## Admin Panel

- [ ] Full admin panel required
- [ ] Web interface (/admin) with session auth
- [ ] REST API (/api/v1/admin) with bearer token

## CLI

- [ ] All standard commands implemented
- [ ] --help, --version, --status work without privileges
- [ ] --update command with check/yes/branch subcommands

## Build & Deploy

- [ ] 4 Makefile targets: build, release, docker, test
- [ ] 4 GitHub/Gitea workflows: release, beta, daily, docker
- [ ] 8 platform builds (4 OS x 2 arch)
- [ ] Docker uses tini, Alpine base

## Security

- [ ] All security headers implemented
- [ ] Sensitive data never exposed unless necessary
- [ ] Rate limiting in production

---

**END OF SPECIFICATION**

**Remember: ALL sections marked NON-NEGOTIABLE must be implemented exactly as specified.**

**When in doubt: Re-read the spec. Ask questions. Never assume.**
