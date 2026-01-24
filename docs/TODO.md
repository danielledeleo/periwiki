In progress
- Docs-on-docs (ship the documentation for Periwiki as sample files)

Soon(ish)
- CLAUDE.md (how to behave in the code base, strategy, hygiene)
- Sitemap (XML and HTML, Special:Sitemap and Special:Sitemap.xml, available at /sitemap.xml as well)
- Include better sample pages
- Links on history pages to diff of live page
- Move .go files out of root directory
- Tagging system (YAML frontmatter exposed through editor as interface?)
- Auto populate new databases with an Admin user with id 1 as the owner of all the default pages
- Create Markdown templates for default pages (user profiles, admin pages, etc.)
- Re-render pages on launch (somehow determine if they are stale, e.g. if the renderer changes)
- Make user pages, index, and other special pages all types of Articles (and revisions)
- User profiles pages to be ~user URLs, which are linked to with wikilinks e.g. [[~dani]]
- User settings
- Password recovery, 2FA, Login providers
- Widgets
  - Images (thumbnails and large view)
  - Side cards
- Backup/data export mechanism
- 404 page with "Did you mean /wiki/notfound?" link
- Two column References section layout like wikipedia
- Add extension for custom superscripts/subscripts, [citation needed]-style, not bound to a footnote
- Rich customizable home page (featured articles, other custom widgets)
- When editing a page that does not yet exist, replace underscores with spaces in title
- File:image.jpg static file handing (and design overall media strategy...)
  - Asset metadata is tracked by the wiki system, but blobs are not stored in the database
  - References are kept to an externally managed media system (local filesystem, S3, static files on an nginx server, etc.)

Configuration and Runtime
- Store cookie secret in database instead of .cookiesecret.yaml (update docs/security.md)
- Compile template files into the binary (embed)
- Improve "server failed to start" error message with specific reason (port in use, permission denied, etc.)
- CLI flags to override config file path, database file, host, log level, etc.

Down the line
- Moderation tools?
- Admin panel
- A theme system (custom templates)
- Theme configurability (custom logo, custom colours)
- WebAssembly plugin system
  - Plugin Interface
  - There may be different classes of plugin, requiring different interfaces
  - Scopes/permissions/API/custom database tables
- I am not a robot
- Rate limiting
- Language locales!
  - UI locale strings
  - maybe copy wikipedia's subdomain approach
- Locked/featured pages
- Search
- Postgres/MySQL support
- Better editor experience
- Richer diffs, talk pages
- [LOW] extending.md (documenting how to add special pages and WikiLink resolvers)

Dependency Maintenance (January 2025)
- [MEDIUM] Evaluate SQLBoiler alternatives (volatiletech/sqlboiler)
  - Maintainers recommend Bob (https://bob.stephenafamo.com) or sqlc as alternatives
  - Consider Bun, others found here (https://awesome-go.com/#orm)
  - Project is in low-maintenance mode
- [MEDIUM] Replace or fork michaeljs1990/sqlitestore
  - No releases, last commit May 2021, uses CGO-based sqlite driver
  - Consider implementing session store with modernc.org/sqlite instead
- [LOW] Monitor jmoiron/sqlx - stable but minimal maintenance since v1.4.0 (April 2024)
- [LOW] Monitor sergi/go-diff - functional but limited maintenance, issues unresolved for years
