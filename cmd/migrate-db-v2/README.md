# cmd/migrate-db-v2

One-shot side-by-side migration from the v2 schema to the v3 schema. Modelled
on the Cloudreve `drive` → `drive_v2` upgrade pattern: the v3 panel only
knows about the new database, so the old DB stays 100% intact as a permanent
backup until the operator decides to drop it.

> **Lifecycle**: this directory is meant to be deleted in the same commit
> that marks the migration as signed-off. Git history is the audit trail.

## What it does

- Reads the v2 source DB (`--src`) — never writes to it.
- Creates the v3 schema on the destination DB (`--dst`) via the main
  program's `mysql.EnsureSchema`.
- Copies / transforms each table:

| v2 table | v3 destination | Transform |
|---|---|---|
| `ui_settings` (one wide row) | `settings` (KV) | Splits ~40 fields into KV rows grouped by `type` (site / auth / sub / security / runtime / notice / notify). The migration also adds a `_migration` sentinel row at start so a crashed re-run is caught by the empty-dst guard. |
| `mail_settings.{expire_before_days, traffic_remain_percent}` | `settings.type='notify'` | Moves the two notify thresholds out of `mail_settings`. |
| `mail_settings` (other fields) | `mail_settings` (slimmed) | SMTP-connection subset only. |
| `saml_config` | `saml_settings` | Renamed; same fields. |
| `oidc_config` | `oidc_settings` | Renamed; same fields. |
| `xui_clients` | `user_xui_clients` | Renamed; `panel_name` dropped; `last_raw_*_bytes` seeded from the latest pre-v3 `client_traffic_snapshots` row per (panel, inbound, email) so the first post-migration traffic poll does not double-count history. |
| `nodes` | `nodes` | `panel_name` column dropped. |
| `users` / `groups_` / `xui_panels` / `traffic_snapshots` / `node_traffic_snapshots` / `audit_log` / `sub_logs` / `sync_tasks` / `mail_templates` / `mail_sent` | same name in v3 | Copied verbatim. |
| `client_traffic_snapshots` | (not copied) | Pre-v3 stored raw counters; v3 stores lifetime. Mixing the two would corrupt history graphs. The latest raw value per client is preserved on the new `user_xui_clients.last_raw_*` columns so live traffic accounting continues correctly. |
| `rule_sets` | (dropped) | Dead code — rule sets actually live in `config/rulesets/*.yaml`. |

## Usage

From repo root.

### Local dev (SQLite → SQLite)

```bash
go run ./cmd/migrate-db-v2/ --driver=sqlite \
  --src=data/panel.db --dst=data/panel_v3.db
```

### Production (MySQL → MySQL)

```bash
# 1. Create the empty destination DB out-of-band.
mysql -u root -p -e 'CREATE DATABASE panel_v3 CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;'

# 2. (Optional but recommended) Take a dump of the source DB.
mysqldump -u root -p panel > panel-v2-backup-$(date +%F).sql

# 3. Run the migration.
go run ./cmd/migrate-db-v2/ --driver=mysql \
  --src='user:pass@tcp(host:3306)/panel?charset=utf8mb4&parseTime=true' \
  --dst='user:pass@tcp(host:3306)/panel_v3?charset=utf8mb4&parseTime=true'

# 4. Dry-run mode (counts only, no writes) is also supported:
go run ./cmd/migrate-db-v2/ --driver=mysql --src=... --dst=... --dry-run
```

**On encrypted columns**: AES-GCM-encrypted values (mail password, SAML SP
key, OIDC client secret, 3X-UI panel API token / admin password) move as
opaque ciphertext — this cmd never decrypts or re-encrypts. The v3.0.0 panel
decrypts at boot using the `SecretKeyMaterial` already configured in your
`config.yaml`. Migration only succeeds end-to-end if that key matches what
the legacy panel was using; if not, the v3.0.0 panel will surface a clear "decrypt
secret" error on first boot and you fix `config.yaml` separately.

## Safety guarantees

- The source DB is opened in read-only paths only; the migration code never
  issues a write against `--src`.
- The destination DB must be empty (no rows in `settings`) — re-running over
  a half-migrated DB would double-insert. The program exits cleanly with a
  friendly error if it detects existing v3 data.
- Each table category is wrapped in its own transaction; a failure on one
  category does not corrupt earlier writes.

## After it succeeds

1. Edit `config.yaml`: change the `database` setting to point at the v3 DB
   (`panel_v3`).
2. Restart the panel.
3. Walk through admin → settings → SMTP / SAML / OIDC to verify config came
   through. Walk through admin → nodes / traffic to verify panel names
   render correctly (they're now resolved from the in-memory pool, not the
   DB).
4. Once you're satisfied (1-2 weeks of monitoring is reasonable for a
   self-hosted panel), `DROP DATABASE panel;` to free the disk.
5. Delete `cmd/migrate-db-v2/` in your next commit. Commit message should
   record the migration date — that's the durable record.
