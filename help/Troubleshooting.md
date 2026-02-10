---
display_title: Troubleshooting
---

## Startup errors

**"failed to read config"**
The `config.yaml` file exists but is malformed. Validate the YAML syntax or delete the file to use defaults.

## Runtime errors

**"article save failed"**
An edit could not be saved. The log includes the reason — commonly a conflict when two users edit the same revision simultaneously.

## Database issues

If you encounter SQLite errors, ensure:
- The database file has correct permissions
- The disk has available space
- No other process has an exclusive lock on the database
