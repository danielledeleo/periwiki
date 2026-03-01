# User:, User_talk:, and Special:Contributions

## Overview

Add User: and User_talk: namespaces plus a Special:Contributions page.
Usernames throughout the site (history, diff) become links with red-link
support. Follows the existing namespace patterns (Talk:, Special:, Periwiki:).

## User: Namespace

### Routing

Add `user` case to `NamespaceHandler`. Dispatches to a new `handleUserPage()`
handler that renders a two-section layout:

1. **Custom content** (top) — editable wiki article stored as `User:ScreenName`
   in the Article table. Only rendered if the article exists in the DB.
2. **Stats footer** (bottom) — always rendered. Computed at request time: member
   since date, total edit count. New template `templates/user_page.html`.

If the user account doesn't exist at all, return 404.

### Editing

`handleEdit()` gains a guard: if the article URL starts with `User:` (but not
`User_talk:`), only the matching user or an admin may save. Others get 403.

### Existence for red links

`User:ScreenName` existence is based on the **article** existing in the DB, not
the user account. `[[User:Alice]]` renders as a red link until Alice creates her
page. Navigating to `/wiki/User:Alice` still works (shows stats), but no custom
content appears.

No change to the ExistenceChecker — it already queries the Article table.

### Tabs

```
[User page] [Talk] [Contribs]          [Edit] [History]
```

- "Edit" only shown to the page owner and admins.
- "Contribs" links to `Special:Contributions/ScreenName`.

## User_talk: Namespace

### Routing

Add `user_talk` case to `NamespaceHandler`. Dispatches to `dispatchArticle()`
with the full `User_talk:ScreenName` URL — same pattern as Talk: pages.

### Editing

Any logged-in user can edit (same as regular Talk: pages).

### Creation guard

Creating `User_talk:ScreenName` requires user `ScreenName` to exist (parallel
to Talk: requiring the subject article to exist).

### Tabs

Same as User: page tabs, with "Talk" active instead of "User page".

## Special:Contributions

New special page at `Special:Contributions/ScreenName`. Lists all revisions by
that user, ordered by date descending, with links to the articles and diffs.

Registered in `RegisterSpecialPages()`. Needs a new repository/service method:
get revisions by user screen name.

## Username links on history and diff pages

### Pattern

```
<username> (talk | contribs)
```

Where:
- `<username>` links to `/wiki/User:ScreenName`, with `pw-deadlink` class if the
  User: article doesn't exist.
- `talk` links to `/wiki/User_talk:ScreenName`.
- `contribs` links to `/wiki/Special:Contributions/ScreenName`.

### HasUserPage via LEFT JOIN (Option B)

Add a `HasUserPage bool` field to `wiki.User`. Populated by a LEFT JOIN against
the Article table when fetching revision history and diff data. The JOIN checks
for an article with URL = `'User:' || User.screenname`. This avoids extra
queries.

## Navbar

Change `/profile/{{ pathEscape .User.ScreenName }}` to
`/wiki/User:{{ pathEscape .User.ScreenName }}`.

## Schema changes

None. User: and User_talk: pages are regular articles in the existing Article
table. HasUserPage is computed at query time. Special:Contributions is a handler.
