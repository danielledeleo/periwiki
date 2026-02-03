# Old Revision Restore Flow

## Problem

When editing from an old revision (restoring), the system returns 409 Conflict because `previous_id` is set to the old revision's ID, causing the new revision ID to collide with an existing one.

## Solution

Pass the *current* revision ID as `previous_id` when editing an old revision, while loading content from the old revision. This preserves conflict detection (racing against the latest) while allowing restores.

Add informational banners to make the restore flow clear to users.

## Data Passed to Templates

**Article view (old revision):**
- `IsOldRevision` (bool) - true when viewing a non-current revision
- `CurrentRevisionID` (int) - the latest revision's ID
- `CurrentRevisionCreated` (time.Time) - when the current version was last edited

**Article edit (restoring):**
- `IsRestoring` (bool) - true when editing from an old revision
- `SourceRevisionID` (int) - the revision being restored from
- `CurrentRevisionID` (int) - for the `previous_id` form field

## Handler Changes

### handleView

When viewing a specific revision, also fetch the current revision:

```go
article, err := a.Articles.GetArticleByRevisionID(articleURL, revisionID)
// ...

current, _ := a.Articles.GetArticle(articleURL)
isOldRevision := current != nil && current.ID != article.ID

// Pass to template
map[string]interface{}{
    "IsOldRevision":          isOldRevision,
    "CurrentRevisionID":      current.ID,
    "CurrentRevisionCreated": current.Created,
}
```

### handleEdit

When editing a specific revision, fetch current revision for `previous_id`:

```go
sourceArticle, err := a.Articles.GetArticleByRevisionID(articleURL, revisionID)
current, _ := a.Articles.GetArticle(articleURL)

other["IsRestoring"] = true
other["SourceRevisionID"] = sourceArticle.ID
other["CurrentRevisionID"] = current.ID
```

## Template Changes

### article.html

Add banner when viewing old revision:

```html
{{if $.IsOldRevision}}
<div class="pw-callout pw-info">
    You are viewing an old revision of this article from {{.Created.Format "January 2, 2006 at 3:04 pm"}}.
    <a href="{{articleURL .URL}}">View current version</a>
</div>
{{end}}
```

### article_edit.html

Add restore warning and use correct `previous_id`:

```html
{{if $.Other.IsRestoring}}
<div class="pw-callout pw-info">
    You are restoring an old revision of this article (revision {{$.Other.SourceRevisionID}}).
    Saving will create a new revision with this content.
</div>
{{end}}

<!-- Use current revision ID for conflict detection when restoring -->
{{if $.Other.CurrentRevisionID}}
<input type="hidden" name="previous_id" value="{{$.Other.CurrentRevisionID}}" />
{{else}}
<input type="hidden" name="previous_id" value="{{.ID}}" />
{{end}}
```

## Test Coverage

- Old revision view shows banner with correct timestamp and link
- Edit from old revision uses current revision's ID as `previous_id`
- Concurrent edit detection still works when restoring
- Normal edit flow unchanged (no regression)

## Files to Modify

1. `internal/server/handlers.go` - `handleView` and `handleEdit`
2. `templates/article.html` - old revision banner
3. `templates/article_edit.html` - restore warning + `previous_id` fix
