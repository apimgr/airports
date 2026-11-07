# CI/CD Workflows Rules (PART 27)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Pin third-party Actions/images to a tag (e.g. `@v4`) — always pin to a full commit SHA with a
  version comment (`uses: owner/action@<sha>  # vX.Y.Z`)
- Use Makefile targets or `make dev`/`make local`/`make build` inside a CI/CD workflow — CI/CD
  requires explicit `go build ...` commands with all flags visible
- Reference local host paths (`~/.local/share/go`, host cache dirs) inside CI/CD — use CI-native
  caching or `/tmp/`
- Depend on local Docker/Incus containers for CI builds — GitHub/Gitea/Forgejo Actions and GitLab CI
  use the `casjaysdev/go:latest` image directly as the job container
- Cross-cancel different release refs — a concurrency group may only auto-cancel an older run for
  the *exact same* branch or tag ref, never a different one
- Ship a project without `ci.yml` and `release.yml` (or their Gitea/GitLab/Jenkins equivalents)
- Create `build-toolchain.yml` or a `docker/Dockerfile.build` for Go projects — `casjaysdev/go:latest`
  is externally maintained and sufficient
- Use `default_branch` for the secret-scan commit range — use `github.event.before`/`.after` instead
  (`default_branch` can resolve to HEAD after a push and silently skip the scan)

## CRITICAL - ALWAYS DO

- Match workflow file to git provider: GitHub → `.github/workflows/`; Gitea → `.gitea/workflows/`;
  Forgejo → `.forgejo/workflows/` (or `.gitea/workflows/` for compat); GitLab → single
  `.gitlab-ci.yml`; Jenkins → `Jenkinsfile`
- Run every CI/CD job (build/lint/test/vuln-scan) inside `container: image: casjaysdev/go:latest`
  (or GitLab `image:`) — never install Go tooling inline
- Set `VERSION`, `COMMIT_ID`, `BUILD_DATE` explicitly in a "Set build info" step, then inject via
  `-ldflags "-s -w -X 'main.Version=...' -X 'main.CommitID=...' -X 'main.BuildDate=...' -X 'main.OfficialSite=...'"`
- Build the full 8-target matrix: linux/darwin/windows/freebsd × amd64/arm64 (windows adds `.exe`)
- Add workflow concurrency (`cancel-in-progress: true`) to every push-triggered workflow targeting
  `main`, `master`, `devel`, `dev`, or `beta`; tag-release workflows use a per-exact-tag group
- Validate every staged workflow file with `act --list -W {file}` before it's considered done
- Check the triggered CI run's status after every push; a red or still-running build is a bug to
  fix immediately, never left for later
- Read `OFFICIAL_SITE` from `site.txt` first, falling back to the CI secret/variable — never guess

## Key Rules Summary

| Area | Rule |
|------|------|
| GitHub workflow files | `ci.yml` (push/PR + weekly security cron), `release.yml` (tag `v*`/semver, required), `beta.yml` (push to `beta`, optional), `daily.yml` (3am UTC + push main/master, optional), `docker.yml` (any push + version tags, optional) |
| Gitea/Forgejo workflow files | Same file set as GitHub, in `.gitea/workflows/` or `.forgejo/workflows/`; use `gitea.*`/`forgejo.*` context and `GITEA_ENV`/`FORGEJO_ENV` in place of `github.*`/`GITHUB_ENV` |
| GitLab | Single `.gitlab-ci.yml`; stages `build → test → package → release → docker`; `rules:` (not `only/except`) for triggers |
| Jenkins | Single `Jenkinsfile`; `agent none` at pipeline level, per-stage `agent { label 'amd64'/'arm64' }`; branch/tag `when{}` mirrors GitHub triggers |
| Action pinning | Every third-party `uses:` pinned to full commit SHA + `# vX.Y.Z` comment, never a tag |
| `ci.yml` required jobs | `lint` (go vet, staticcheck), `test` (coverage ≥60% threshold enforced), `build` (needs lint+test), `vuln-scan` (govulncheck); security jobs (`secret-scan`, `workflow-policy`, `vuln-scan`, `image-scan`) also run weekly (`cron: '0 6 * * 1'`) |
| Release triggers | Tag push matching `v*` or `[0-9]*.[0-9]*.[0-9]*`; concurrency group per exact tag ref |
| Docker image tags | any push → `devel`, `{commit_id}`; push to `beta` → adds `beta`; version tag → `{version}`, `latest`, `YYMM` |
| Docker registry | GitHub → `ghcr.io`; Gitea/Forgejo → auto-detected from server URL; GitLab → `$CI_REGISTRY_IMAGE` |
| Docker platforms | `linux/amd64,linux/arm64` via `docker buildx`, `provenance: false` |
| Build matrix targets | linux/darwin/windows/freebsd × amd64/arm64 (8 total); windows adds `.exe` |
| CLI binary | Only built `if src/client/` exists, output named `{project}-cli-{goos}-{goarch}` |
| Coverage threshold | 60% minimum, enforced via `go tool cover -func` in CI, build fails below it |
| Daily release cleanup | Delete prior `daily` tag/release before creating the new one (`gh release delete daily` / Gitea API DELETE) |

For complete details, see AI.md PART 27.
