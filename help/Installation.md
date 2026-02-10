---
display_title: Installation and configuration
---

## Requirements

- Go 1.24 or later
- Make

## Building

```bash
git clone https://github.com/danielledeleo/periwiki
cd periwiki
make
```

This compiles the `periwiki` binary in the current directory.

## Running

```bash
./periwiki
```

On first run, Periwiki creates the SQLite database, a default `config.yaml`, and a `Main_Page` article with a getting-started guide. By default, it listens on `0.0.0.0:8080`.

The root URL (`/`) redirects to `/wiki/Main_Page`. The seeded main page is a regular wiki article — you can edit or replace it at any time. It uses `layout: mainpage` frontmatter to display "Main page" instead of "Article" in the tab bar.

Periwiki handles SIGINT and SIGTERM gracefully — it stops accepting new HTTP requests, waits for in-flight requests and render jobs to finish (up to 30 seconds), then exits.

## Configuration

Periwiki has two layers of configuration: a **file-based** config for bootstrap settings and a **database-backed** runtime config for everything else.

### File configuration (config.yaml)

If a `config.yaml` file exists in the working directory, Periwiki reads and applies it. If it does not exist, one is created with defaults on first run.

These settings require a restart to take effect.

| Option | Default | Description |
|--------|---------|-------------|
| `dbfile` | `periwiki.db` | Path to SQLite database |
| `host` | `0.0.0.0:8080` | Address and port to listen on |
| `base_url` | `http://localhost:8080` | Public base URL (used in XML sitemaps) |
| `log_format` | `pretty` | Log output format: `pretty`, `json`, or `text` |
| `log_level` | `info` | Log verbosity: `debug`, `info`, `warn`, or `error` |

### Runtime configuration (database)

Other settings are stored in the SQLite `Setting` table and initialized with defaults on first run. There is no management UI for these yet — a settings page is planned.

| Key | Default | Description |
|-----|---------|-------------|
| `min_password_length` | `8` | Minimum characters required for registration |
| `cookie_expiry` | `604800` | Session lifetime in seconds (7 days) |
| `allow_anonymous_edits_global` | `true` | Whether unauthenticated users can edit articles |
| `render_workers` | `0` | Render queue worker count (`0` = one per CPU core) |
| `cookie_secret` | *(auto-generated)* | 64-byte random key for session signing |
| `render_template_hash` | *(auto-generated)* | Hash of render templates for stale content detection |

The `cookie_secret` and `render_template_hash` settings are managed automatically.

## Logging

All logs are written to stderr using Go's `slog` package. HTTP requests are automatically logged with method, path, status, response size, duration, and remote address.

| Format | Description |
|--------|-------------|
| `pretty` | Colorized, human-readable output (default) |
| `json` | JSON lines — suitable for log aggregation tools |
| `text` | Plain `key=value` pairs |

## Database

Periwiki uses SQLite. The database schema is applied and migrated automatically at startup. Some migrations involve recreating tables (SQLite lacks `ALTER TABLE DROP COLUMN`), which may briefly increase startup time on large databases.
