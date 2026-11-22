# Security Policy

## Supported versions

Security fixes are provided for the latest published Airports release. Update
to the latest release before reporting an issue.

## Report a vulnerability

GitHub private vulnerability reporting is the primary channel:
[Report a vulnerability](https://github.com/apimgr/airports/security/advisories/new).
Do not file vulnerabilities as public bug reports. On GitLab, Gitea, Forgejo,
or other mirrors, report privately through the GitHub origin repository above.
The security email at [security@apimgr.us](mailto:security@apimgr.us) is a
secondary contact only.

For a vulnerability in a running instance, its `/server/security` page links
to the report form. The canonical machine-readable disclosure policy is
`/.well-known/security.txt`; the form is also available at
`/server/contact?security_id=...` using a current security ID from the page.

## Response and disclosure

Maintainers aim to acknowledge a report within 72 hours and target a fix within
14 days for verified critical or high severity issues. The default coordinated
disclosure period is 90 days; see `/server/security/policy` for the current
policy and safe-harbor terms. Maintainers will coordinate status and disclosure
timing with the reporter.
