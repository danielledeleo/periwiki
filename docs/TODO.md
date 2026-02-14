Soon(ish)
- [MED] finish db refactor (see plans)
- [MED] Backlinks feature (will help trigger redlink re-renders)
  - any dep that causes a rerender
- [MED] Sitemap registry: include special pages via opt-in interface (see docs/plans/sitemap-registry.md)
- [MED] Talk:pages, User:pages
- Decide on case sensitivity in URLs
- Page alias and redirect system (design TBD)
- Article visibility settings
  - draft
  - private
  - internal-only
- e2e database migration tests (old test databases as part of testing)
- Improve editing request flow
  - unmodified content submissions should still re-render
  - submission errors should appear above editor window, not a new page (unmodified, conflicts)
- Include better sample pages
- Move .go files out of root directory (note: `content.go` added for overlay FS embed)
- User-defined templates system (see docs/plans/user-templates-design.md)
  - Prerequisites: content re-render queue, article type field
  - Subsumes: widgets, tagging/frontmatter, rich homepage, some special page customization
  - See: wikipedia {Navbar} + {Template that calls navbar}
  - **Privileged templates defined in the theme, dynamic templates defined as Articles**
  - In order for runtime privileged templates to work, we'll likely need a theming system
  - They should have distinct names
- Create Markdown templates for default pages (admin pages, etc.)
- User settings?
- Management UI for runtime config (anonymous editing toggle, password policy, render workers, etc.)
- Password recovery, 2FA, Login providers
- Backup/data export mechanism
- Tagging system (frontmatter-based, with tag pages)
- 404 page with "Did you mean /wiki/notfound?" link (?)
- Two column References section layout like wikipedia
- Add extension for custom superscripts/subscripts, [citation needed]-style, not bound to a footnote
- File:image.jpg static file handing (and design overall media strategy...)
  - Asset metadata is tracked by the wiki system, but blobs are not stored in the database
  - References are kept to an externally managed media system (local filesystem, S3, static files on an nginx server, etc.)
- Allow the binary to launch in the event of database errors/connectivity loss (at least display an error page)

Architecture
- Export `getOrCreateSetting` from `wiki` package, remove duplicate in `internal/server/setup.go`
- `EmbeddedArticles.Get` returns shared pointer — return shallow copy to prevent mutation bugs
- [BUG] Render queue `Shutdown` doesn't drain pending jobs — goroutines waiting on `waitCh` leak
- Parse TOC template once at init instead of on every `Render()` call
- Consider promoting orphan h4 headings (under h2, no h3) into the h2's children instead of dropping
- Add explicit transaction wrapping for table-recreation migrations in `migrations.go`
- Embedded articles not included in `GetAllArticles` — document whether intentional or add them
- Add integration test for Periwiki namespace handler (planned in Task 8 but not implemented)
- Add comment on wikilink regex noting Go's RE2 prevents true catastrophic backtracking

Configuration and Runtime
- First run/setup mode
  - Auto populate new databases with an Admin user with id 1 as the owner of all the default pages
- Improve "server failed to start" error message with specific reason (port in use, permission denied, etc.)
- CLI flags to override config file path, database file, host, log level, etc.

Down the line
- SSG mode (full static builds)
- Custom goldmark table ParagraphTransformer that tracks [[/]] bracket depth, allowing unescaped pipes in wikilinks inside tables (currently requires \| escape)
- Moderation tools?
- Admin panel
- A theme system (custom templates)
- Theme configurability (custom logo, custom colours)
- Action rate limiting (saves per x time, anti-crawl, etc.)
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

Dependency Maintenance (January 2026)
- [MEDIUM] Replace or fork michaeljs1990/sqlitestore
  - No releases, last commit May 2021, uses CGO-based sqlite driver
  - Consider implementing session store with modernc.org/sqlite instead
- [LOW] Monitor jmoiron/sqlx - stable but minimal maintenance since v1.4.0 (April 2024)
- [LOW] Monitor sergi/go-diff - functional but limited maintenance, issues unresolved for years
