
Soon(ish)
- [MED] Whole-site cache busting on deploy — after a binary or template update, browsers can serve stale page HTML because ETags/Last-Modified only track article content, not the surrounding templates or CSS. Need a mechanism (e.g. mixing BuildCommit into ETags, or similar) so deploying a new binary invalidates all browser caches via normal HTTP conditional request handling.
- [MED] Sitemap registry: include special pages via opt-in interface (see docs/plans/sitemap-registry.md)
- Page alias and redirect system (design TBD)
- Page renaming tool e.g., move foo to Foobar_(Computing)
  - this is separate from but related to from the alias/redirect system
  - watch out for interactions with the backlink invalidation system
- Article visibility settings
  - draft
  - private
  - internal-only
- e2e database migration tests (old test databases as part of testing)
- [MED] Pagination for potentially large queries (page history, user contributions)
  - establish a general pattern
- Improve editing request flow
  - unmodified content submissions should still re-render
  - submission errors should appear above editor window, not a new page (unmodified, conflicts)
- Include better sample pages
- Move .go files out of root directory (note: `content.go` added for overlay FS embed)
- [MED] User-defined templates system (see docs/plans/user-templates-design.md)
  - Prerequisites: content re-render queue (done), namespace routing (done), frontmatter parsing (done)
  - Core widgets in `templates/_widgets/`, user templates as `Template:*` articles
  - Subsumes: widgets, rich homepage, some special page customization
- Create Markdown templates for default pages (admin pages, etc.)
- User settings?
- Password recovery, 2FA, Login providers
- Backup/data export mechanism
- Tagging system (frontmatter-based, with tag pages)
  - Research task: look at prior art for taxonomy systems
- 404 page with "Did you mean /wiki/notfound?" link (?)
- Two column References section layout like wikipedia
- Add extension for custom superscripts/subscripts, [citation needed]-style, not bound to a footnote
- File:image.jpg static file handing (and design overall media strategy...)
  - Asset metadata is tracked by the wiki system, but blobs are not stored in the database
  - References are kept to an externally managed media system (local filesystem, S3, static files on an nginx server, etc.)
- Allow the binary to launch in the event of database errors/connectivity loss (at least display an error page)
- [MED] Licensing strategy for user generated content
  - At what level should user-generated content be licensed?
  - Sitewide, per page, default, overrides?
  - How is it displayed?

Architecture

Configuration and Runtime
- First run/setup mode
- Improve "server failed to start" error message with specific reason (port in use, permission denied, etc.)
- CLI flags to override config file path, database file, host, log level, etc.

HTML Meta Tags
- `<meta name="description">` — explore auto-generation from article content vs. frontmatter opt-in
- Open Graph tags (og:title, og:type, og:url) for link previews in Slack/Discord/social media
- `<meta name="author">` from article creator
- `<link rel="canonical">` for articles
- `<meta name="robots">` noindex for old revisions

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
  - maybe copy wikipedia's subdomain approach?
- Locked/featured pages
- Search
- Postgres/MySQL support
- Better editor experience
- Richer diffs
- [LOW] extending.md (documenting how to add special pages and WikiLink resolvers)

Depends on user-defined templates (see docs/plans/user-templates-design.md)
- Widgets
  - Images (thumbnails and large view)
  - Side cards
- Rich customizable home page (featured articles, other custom widgets)

Dependency Maintenance (January 2026)
- [LOW] Monitor jmoiron/sqlx - stable but minimal maintenance since v1.4.0 (April 2024)
- [LOW] Monitor sergi/go-diff - functional but limited maintenance, issues unresolved for years
