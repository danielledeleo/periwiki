# Installation and configuration

## Requirements

- Go 1.21 or later
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

On first run, Periwiki creates the SQLite database automatically. By default, it listens on `0.0.0.0:8080`.

## Configuration

If a `config.yaml` file exists in the same directory as the binary, Periwiki reads and applies it. Otherwise, reasonable defaults are used.

| Option | Default | Description |
|--------|---------|-------------|
| `host` | `0.0.0.0:8080` | Address and port to listen on |
| `dbfile` | `periwiki.db` | Path to SQLite database |
| `min_password_length` | `8` | Minimum password length for registration |
| `cookie_expiry` | `604800` | Session lifetime in seconds (default: 7 days) |
| `log_format` | `pretty` | Log output format: `pretty`, `json`, or `text` |
| `log_level` | `info` | Log verbosity: `debug`, `info`, `warn`, or `error` |

Changes require a restart.
