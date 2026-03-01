# Anonymous Edit IP Tracking

## Goal

Persist anonymous editors' IP addresses on revisions and display them publicly in article history, enabling identification of vandalism sources. Admin tooling (blocking, bulk revert) is out of scope — this is visibility only.

## Current State

- `AnonymousEdit` table exists in schema but is completely unused (no code references, no migrations).
- Middleware captures IP from `req.RemoteAddr` onto `User.IPAddress` at runtime but never persists it.
- All anonymous edits display as "by Anonymous" with no way to distinguish editors.

## Design

### Schema

- **Add** `ip_address TEXT` (nullable) to `Revision`. NULL for authenticated edits; populated for anonymous edits.
- **Drop** the unused `AnonymousEdit` table.
- **Migration**: `ALTER TABLE Revision ADD COLUMN ip_address TEXT;` — no backfill needed (historical anonymous revisions will have NULL, which is accurate).

### IP Extraction

Currently `serveAsAnonymous()` uses `net.SplitHostPort(req.RemoteAddr)`, which returns the TCP peer address — wrong when behind a reverse proxy.

- **Add a boolean config flag** `trust-proxy` (default `false`).
- When `true`, prefer `X-Forwarded-For` (rightmost value) then `X-Real-IP`, falling back to `RemoteAddr`.
- When `false`, use `RemoteAddr` as today.
- Extract this into a helper function used by the session middleware.

### Persistence

In `InsertArticle` / `InsertArticleQueued` (article_repo.go), include `ip_address` in the Revision INSERT. Value comes from `article.Creator.IPAddress`. Store NULL for authenticated users.

### Model

Add `IPAddress string` to the revision struct used for history display. Update history queries to SELECT `ip_address`.

### Display

- Article history: show IP address instead of "Anonymous" for anonymous edits. Authenticated edits continue showing screenname.
- Diff view: same treatment in revision metadata.
- No hashing or obfuscation — IPs shown publicly (Wikipedia-style).

## Out of Scope

- IP blocking / rate limiting
- Bulk revert by IP
- IP obfuscation or hashing
- Backfill of historical anonymous edits
