In progress
- Docs-on-docs (ship the documentation for Periwiki as sample files)

Soon(ish)
- [HIGH] Anonymous editing toggle (env config for now)
- Frontmatter schema design (see docs/plans/frontmatter-design.md)
  - Consider ORM replacement first (see Dependency Maintenance section)
- Content re-render job (due to template changes or edits, on launch, slow trickle, manually triggered, cron)
- When editing from an old revision (restoring, essentially), allow it to publish instead of 409.
- CLAUDE.md (how to behave in the code base, strategy, hygiene)
- Page alias and redirect system (see docs/plans/alias-redirect-design.md)
- Backlinks feature (will help trigger redlink re-renders)
- Sitemap registry: include special pages via opt-in interface (see docs/plans/sitemap-registry.md)
- Improve editing request flow
  - unmodified content submissions should still re-render
  - submission errors should appear above editor window, not a new page (unmodified, conflicts)
- Include better sample pages
- Move .go files out of root directory
- User-defined templates system (see docs/plans/user-templates-design.md)
  - Prerequisites: content re-render queue, article type field
  - Subsumes: widgets, tagging/frontmatter, rich homepage, special page customization
  - See: wikipedia {Navbar} + {Template that calls navbar}
  - **Privileged templates defined in the theme, dynamic templates defined as Articles**
- Auto populate new databases with an Admin user with id 1 as the owner of all the default pages
- Create Markdown templates for default pages (user profiles, admin pages, etc.)
- Action rate limiting (saves per x time, anti-crawl, etc.)
- Make user pages, index, and other special pages all types of Articles (and revisions)
- User profiles pages to be ~user URLs, which are linked to with wikilinks e.g. [[~dani]]
- User settings
- Password recovery, 2FA, Login providers
- Backup/data export mechanism
- 404 page with "Did you mean /wiki/notfound?" link
- Two column References section layout like wikipedia
- Add extension for custom superscripts/subscripts, [citation needed]-style, not bound to a footnote
- When editing a page that does not yet exist, replace underscores with spaces in title
- File:image.jpg static file handing (and design overall media strategy...)
  - Asset metadata is tracked by the wiki system, but blobs are not stored in the database
  - References are kept to an externally managed media system (local filesystem, S3, static files on an nginx server, etc.)
- Article content injection fuzzing

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

Depends on user-defined templates (see docs/plans/user-templates-design.md)
- Widgets
  - Images (thumbnails and large view)
  - Side cards
- Rich customizable home page (featured articles, other custom widgets)
- Tagging system (frontmatter-based, with tag pages)

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
