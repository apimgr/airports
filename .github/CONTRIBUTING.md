# Contributing to Airports

## Set up and verify changes

Use Docker for Go commands; the host does not need a Go toolchain.

```sh
git clone https://github.com/apimgr/airports.git
cd airports
make dev
make test
make build
```

The documentation source is in `docs/`. Install its declared dependencies with
`python -m pip install -r docs/requirements.txt`, then build it with
`mkdocs build`.

Update or add tests and documentation with behavior changes. Do not introduce
stubs, placeholder behavior, credentials, or tool attribution.

## Branches and pull requests

Create a focused branch from `main`, make the change, and open a pull request
against `main`. Describe the behavior and include the commands and results used
to verify it. Keep commits scoped and include all required documentation and
configuration updates.

## Security reports

Do not open a public issue for a vulnerability. Use
[GitHub private vulnerability reporting](https://github.com/apimgr/airports/security/advisories/new).
For a vulnerability in a running instance, see its `/server/security` page or
the canonical `/.well-known/security.txt` endpoint.
