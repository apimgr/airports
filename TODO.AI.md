# airports alignment backlog

These are verified implementation gaps found while rereading the current AI.md. Remove each item in the same change that resolves it.

## Configuration

- [ ] Reject unknown keys in `secrets.yml` with an explicit safe error while keeping the AI.md startup rule: log and ignore invalid configuration without preventing startup.

## Database

- [ ] Implement the AI.md PART 10 SQLite replica for remote libSQL/Turso mode: config schema, per-table cache metadata, background sync, offline read-only fallback, secret-dependent route behavior, and maintenance while the remote database is unavailable.

## Build and integration tests
- [ ] Complete `tests/docker.sh` as the AI.md PART 28 Alpine binary basic tier; run the separate shipped-image health and dependency check after `make docker` and in CI. The linked-token write and bad-token CLI assertions remain impossible for this read-only project under the current AI.md contract and are logged in the API template inbox.

## Full-spec follow-through

- [ ] Continue the current AI.md PART-by-PART audit; implement and verify every remaining project-specific code, server-page, documentation, test, build, lint, and CI requirement before clearing this backlog.
