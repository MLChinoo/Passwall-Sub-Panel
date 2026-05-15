# Logic and Security Review

Last updated: 2026-05-15

This note tracks the code review findings that were prioritized for the current hardening pass. It intentionally excludes the `material-demo.html` refactor work.

## Findings

| Priority | Area | Problem | Fix strategy |
|---|---|---|---|
| Critical | JWT auth | Refresh tokens were accepted anywhere an access token was accepted because token type was only stored in `sub` and not checked by auth middleware. | Add access/refresh-specific parsing and make HTTP auth accept only `sub=access`. |
| Critical | Secrets at rest | 3X-UI credentials, SMTP password, SAML SP private key, and OIDC client secret were persisted as plaintext. | Add AES-GCM encryption with `enc:v1:` prefix, generated `encryption_key`, env override, and plaintext read compatibility. |
| High | YAML-backed config | Rule/template slugs were used directly in filesystem paths, allowing path traversal if an admin API body supplied `../...`. | Restrict slugs to `[A-Za-z0-9_-]+` and verify resolved paths remain under the configured directory. |
| High | Subscription blocking | Blocked-client auto-disable updated only the local user row and did not disable managed clients in 3X-UI. | Route auto-disable through `UserService.SetEnabledAndSync`. |
| High | Node deletion | Node delete removed managed clients before verifying unmanaged clients would block inbound deletion. | Add a preflight inbound delete guard and run it before deleting managed clients. |
| Medium | Imported client ownership | Claimed clients recorded their original UUID, but rendering and reconciliation used `user.uuid`, which could silently rotate imported credentials. | Align the user UUID to the claimed client UUID on claim, preserving imported client credentials. |
| Medium | Subscription path | The configured `sub_path` was only partially used: generated URLs still used `/sub/`, and the router prefix was cached only at startup. | Resolve `sub_path` when generating URLs and refresh the route cache periodically/on request. |
| Medium | Runtime settings | `sub_update_interval_hours` existed in the DTO but was not persisted by settings updates. | Save the field in `AdminSettingsHandler.Put`. |
| Medium | Rate limiting | Per-IP limiter had no bucket cleanup, and proxy IP trust was implicit. | Add bucket cleanup and configure trusted proxies from env/default loopback. |

## Notes

- The encryption layer is backwards-compatible for existing plaintext database rows. Reading plaintext still works; the next successful save writes encrypted values.
- Existing deployments without `encryption_key` in `config.yaml` fall back to deriving the encryption key from `jwt_secret`. New generated configs include a separate `encryption_key`.
- Any production rotation of `encryption_key` needs a migration step: decrypt with the old key, save with the new key.
- Reverse-proxy deployments should set `PSP_TRUSTED_PROXIES` to the proxy or container gateway CIDRs. The default trusts only loopback proxies; `none` disables forwarded client IP headers.
