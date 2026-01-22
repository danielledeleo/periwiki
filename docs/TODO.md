In progress
- [MEDIUM] Replace or fork michaeljs1990/sqlitestore
  - No releases, last commit May 2021, uses CGO-based sqlite driver
  - Consider implementing session store with modernc.org/sqlite instead

Soon(ish)
- Make user pages, index, and other special pages all types of Articles (and revisions)
- Docs-on-docs (ship the documentation for Periwiki as sample files)
- Sitemap (XML and HTML)
- Include better sample pages
- Auto populate new databases with an Admin user with id 1 as the owner of all the default pages
- Create Markdown templates for default pages (user profiles, admin pages, etc.)
- Re-render pages on launch (somehow determine if they are stale, e.g. if the renderer changes)
- User settings
- Password recovery, 2FA, Login providers
- Widgets
  - Images (thumbnails and large view)
  - Side cards

Down the line
- Moderation tools?
- Admin panel
- A theme system
- WebAssembly plugin system
  - Plugin Interface
  - There may be different classes of plugin, requiring different interfaces
  - Scopes/permissions/API/custom database tables
- I am not a robot
- Rate limiting
- Language locales!
- Locked/featured pages
- Search
- Postgres/MySQL support
- Better editor experience
- Richer diffs, talk pages

Dependency Maintenance (January 2025)
- [MEDIUM] Evaluate SQLBoiler alternatives (volatiletech/sqlboiler)
  - Maintainers recommend Bob (bob.stephenafamo.com) or sqlc as alternatives
  - Project is in low-maintenance mode
- [LOW] Monitor jmoiron/sqlx - stable but minimal maintenance since v1.4.0 (April 2024)
- [LOW] Monitor sergi/go-diff - functional but limited maintenance, issues unresolved for years
