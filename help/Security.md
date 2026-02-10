---
display_title: Security
---

## Passwords

Passwords are hashed with [bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt) at minimum cost before storage. The raw password is never saved.

| Setting | Default | Description |
|---------|---------|-------------|
| `min_password_length` | `8` | Minimum characters required |

Usernames must match `^[\p{L}0-9-_]+$` (letters, numbers, hyphens, underscores).

## Sessions

Sessions use [gorilla/sessions](https://github.com/gorilla/sessions) with an SQLite-backed store.

| Setting | Default | Description |
|---------|---------|-------------|
| `cookie_expiry` | `604800` | Session lifetime in seconds (7 days) |

The session secret is a 64-byte random key auto-generated on first run and stored in the SQLite `Setting` table (key `cookie_secret`). Keep your database secure — anyone with the secret can forge session cookies.

## HTML sanitization

User-submitted Markdown passes through a rendering pipeline before display:

```
User Markdown
      │
      ▼
Goldmark (CommonMark parser + WikiLink/footnote extensions)
      │
      ▼
TOC injection (goquery) — builds table of contents from headings
      │
      ▼
Bluemonday UGC policy — strips unsafe HTML
      │
      ▼
Custom allowlist — class attributes, footnote data, table alignment
      │
      ▼
Safe HTML
```

Sanitization happens at render time. The sanitized HTML is stored with the revision, so articles are not re-sanitized on every page view.
