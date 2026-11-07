# Privilege Escalation & Service Management Rules (PART 23, 24)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Prompt for privilege escalation if the user cannot actually escalate (not in sudoers/wheel/admin) — show an informative error instead
- Skip the "already root/admin" check — always check EUID/admin token first, before any escalation prompt
- Run permanently as root/Administrator without explicit approval recorded in IDEA.md
- Reuse a well-known/reserved UID or GID (e.g. 999, 998, 65534, 101-110, 170-179) even if it looks free
- Assign different UID and GID values to the service account — they MUST match
- Give the service account a real shell or login capability (must be `/sbin/nologin`, `/usr/bin/false`, or VSA-equivalent)
- Uninstall (`--service --uninstall`) without an explicit `[y/N]` confirmation prompt before deleting data/user
- Have `--service --install` perform user/group creation or directory setup itself — that belongs to normal server startup, not the install flag
- Ship a service manager integration (systemd/OpenRC/SysVinit/runit/rc.d/launchd/Windows) that's missing from the platform detection matrix

## CRITICAL - ALWAYS DO

- Detect escalation method in OS-specific order (Linux: root → sudo → su → pkexec → doas; macOS: root → sudo → osascript; BSD: root → doas → sudo → su; Windows: admin token → UAC → runas)
- Detect the platform's init system and install the matching service file (systemd, OpenRC, SysVinit, runit, rc.d, launchd, or Windows Service)
- Start privileged (root/admin) only long enough to bind ports <1024, then drop to the dedicated service user for the rest of the run (Unix); Windows uses Virtual Service Account instead of privilege drop
- Create the dedicated system user/group as `{internal_name}` with matching UID=GID, chosen by scanning the safe range downward and skipping reserved IDs (Linux/BSD: 200-899; macOS: 200-399)
- Create required directories before creating the service user, then chown to that user
- On `--service --disable`, stop + disable only — keep the service file, config, data, and user intact
- On `--service --uninstall`, stop, disable, remove the service file, delete config/data/cache/log/backup dirs and the PID file, delete the system user/group, then print a manual `rm {binary_path}` message — after confirmation
- Document any permanent-root exception in both the service file and IDEA.md, explaining why privilege drop is not possible

## Key Rules Summary

| Area | Rule |
|------|------|
| Escalation order (Linux) | root → sudo → su → pkexec → doas |
| Escalation order (macOS) | root → sudo → osascript (GUI) |
| Escalation order (BSD) | root → doas → sudo → su |
| Escalation order (Windows) | admin token → UAC → runas |
| Service user | `{internal_name}`, UID=GID, system account, shell `/sbin/nologin` |
| Linux safe UID/GID range | 200-899 (scan from 899 down, skip reserved list) |
| macOS safe UID/GID range | 200-399 (scan from 399 down, same reserved list) |
| Windows account | Virtual Service Account `NT SERVICE\{internal_name}` — no privilege drop needed |
| systemd unit path | `/etc/systemd/system/{internal_name}.service` |
| OpenRC script path | `/etc/init.d/{internal_name}` (via `rc-update`/`rc-service`) |
| SysVinit script path | `/etc/init.d/{internal_name}` (via `update-rc.d`/`chkconfig`) |
| runit service dir | `/etc/sv/{internal_name}/` (`run`, `log/run`) |
| FreeBSD rc.d path | `/usr/local/etc/rc.d/{internal_name}` |
| launchd plist path | `/Library/LaunchDaemons/{plist_name}.plist` (`launchctl load/unload`) |
| Windows service impl | `golang.org/x/sys/windows/svc` + `svc.Run` |
| `--service --install` | detect init system → write service file → enable → start; user creation happens at binary startup, not here |
| `--service --uninstall` | requires `[y/N]` confirmation; deletes data/user; binary itself is left behind |
| `--service --disable` | stop + disable only; config/data/user remain for re-enable |

For complete details, see AI.md PART 23, PART 24.
