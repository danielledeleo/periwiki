In progress
- Automated tests
- WikiLinks (with deadlink detection/styling)

Soon(ish)
- Make user pages, index, and other special pages all types of Articles (and revisions)
- Include better sample pages
- Auto populate new databases with an Admin user with id 1 as the owner of all the default pages
- Create Markdown templates for default pages (user profiles, admin pages, etc.)
- Postgres/MySQL support
- Re-render pages on launch (somehow determine if they are stale, e.g. if the renderer changes)

Down the line
- Plugin system that uses WebAssembly (that adhere to some sort of plugin Interface) to be loaded at runtime.
- There may be different classes of plugin, e.g. a simple lister widget, versus template file watcher/reloader, thus different interfaces
- I am not a robot
- rate limiting
- Language locales!

Maybe
- file watcher for template files (maybe as a plugin?)
