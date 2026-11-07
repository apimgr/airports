# License & Attribution, Project Structure, OS-Specific Paths Rules (PART 2, 3, 4)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use GPL/AGPL/LGPL-licensed dependencies (copyleft — forces derivative works open-source)
- Ship without `LICENSE.md` in the repo root, or with dependency licenses missing from it
- Hardcode `project_name`/`project_org` — infer from git remote or directory path
- Use a config file name other than `server.yml` (never `.yaml`)
- Use `/data/**` or `/config/**` paths outside a Docker container — those are container-only

## CRITICAL - ALWAYS DO

- Keep `LICENSE.md` = MIT text + a "Third-Party Licenses" table for every MIT/Apache/BSD/ISC/MPL dep
- Keep the README license badge as `img.shields.io/github/license/{project_org}/{project_name}` (GitHub-detectable, not a static badge)
- Follow OS-specific path conventions exactly per platform (see table below)
- Keep `internal_name` frozen at its first-time value even if `project_name` is later renamed

## Key Rules Summary

| Area | Rule |
|------|------|
| License | MIT only; `Copyright (c) {year} {project_org}` |
| Variables | `{project_name}` mutable; `{internal_name}` frozen forever after first setup |
| Linux paths | root: `/etc\|/var/lib\|/var/cache\|/var/log/{internal_org}/{internal_name}/`; user: `~/.config\|~/.local/share\|~/.cache\|~/.local/log/...` |
| macOS paths | root: `/Library/Application Support\|Caches\|Logs/{internal_org}/{internal_name}/`; user: `~/Library/...` |
| BSD paths | root: `/usr/local/etc\|/var/db\|/var/cache\|/var/log/{internal_org}/{internal_name}/` |
| Windows paths | root: `%ProgramData%\{internal_org}\{internal_name}\`; user: `%AppData%`/`%LocalAppData%` |
| Docker paths | `/config/{project_name}/`, `/data/{project_name}/` — container-only, never native OS |
| Required dirs | `src/`, `docker/`, `docs/`, `tests/`, `scripts/`, `.github/workflows/` (or provider equivalent) |
| Required root files | `AI.md`, `IDEA.md`, `README.md`, `LICENSE.md`, `CLAUDE.md`, `.gitignore`, `release.txt` |
| Team AI configuration | `.claude/rules/`, `.claude/memory/`, `.claude/CLAUDE.md`, `.cursor/rules/`, and `.windsurf/rules/` are committed; personal overrides, credentials, cache, and lock files are gitignored |

For complete details, see AI.md PART 2, PART 3, PART 4.
