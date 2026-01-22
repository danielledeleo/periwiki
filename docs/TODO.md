In progress
- Automated tests
- WikiLinks (with deadlink detection/styling)

Soon(ish)
- Make user pages, index, and other special pages all types of Articles (and revisions)
- Docs-on-docs (ship the documentation for Periwiki as sample files)
- Include better sample pages
- More robust testing
- Auto populate new databases with an Admin user with id 1 as the owner of all the default pages
- Create Markdown templates for default pages (user profiles, admin pages, etc.)
- Postgres/MySQL support
- Re-render pages on launch (somehow determine if they are stale, e.g. if the renderer changes)
- More content types and widgets for Articles

Down the line
- A theme system
- Plugin system that uses WebAssembly (that adhere to some sort of plugin Interface) to be loaded at runtime.
- There may be different classes of plugin, e.g. a simple lister widget, versus template file watcher/reloader, thus different interfaces
- I am not a robot
- rate limiting
- Language locales!


Maybe
- file watcher for template files (maybe as a plugin?)

Dependency Maintenance (January 2025)
- [HIGH] Replace github.com/pkg/errors and github.com/friendsofgo/errors with standard library errors (Go 1.13+)
  - Both packages are deprecated/unmaintained
  - Use fmt.Errorf with %w for wrapping, standard errors.Is/As for checking
  - For stack traces, consider emperror/errors or tozd/go-errors
- [MEDIUM] Evaluate SQLBoiler alternatives (volatiletech/sqlboiler)
  - Maintainers recommend Bob (bob.stephenafamo.com) or sqlc as alternatives
  - Project is in low-maintenance mode
- [MEDIUM] Replace or fork michaeljs1990/sqlitestore
  - No releases, last commit May 2021, uses CGO-based sqlite driver
  - Consider implementing session store with modernc.org/sqlite instead
- [LOW] Monitor jmoiron/sqlx - stable but minimal maintenance since v1.4.0 (April 2024)
- [LOW] Monitor sergi/go-diff - functional but limited maintenance, issues unresolved for years
- [OK] gorilla/* packages - revived July 2023 with new maintainers
- [OK] spf13/viper, yuin/goldmark, PuerkitoBio/goquery, microcosm-cc/bluemonday, modernc.org/sqlite - all actively maintained
