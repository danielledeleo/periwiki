# Render Queue Design

**Date:** 2026-01-26
**Status:** Approved for implementation

## Overview

A priority render queue that processes markdown-to-HTML rendering jobs asynchronously. All revision renders flow through this queue, ensuring one unified render pathway for edits, bulk re-renders, and future triggers like template changes.

## Goals

- Unified render pathway for all revision renders
- Priority ordering: interactive requests before background jobs
- Same-article deduplication: newer revisions replace queued older ones
- Graceful shutdown with in-flight job completion
- Foundation for future triggers: template changes, startup re-render, scheduled jobs

## Non-Goals (this iteration)

- Persistent queue (in-memory only for now)
- Bulk re-render triggers (manual, template change, startup)
- User-defined templates (Wiki:Templates)

## Architecture

```
Handlers
    │
    ▼
ArticleService (single entry point)
    ├── PostArticle()     → queue → persist HTML
    ├── Preview()         → RenderingService directly (no queue)
    ├── RerenderArticle() → queue → persist HTML (future)
    └── RerenderAll()     → queue → batch persist (future)
    │
    ▼
RenderQueue (injected dependency)
    │
    ▼
RenderingService
```

## Package: `internal/renderqueue`

### Core Types

```go
type Tier int
const (
    TierInteractive Tier = iota  // User waiting (edits, views)
    TierBackground               // Bulk/scheduled re-renders
)

type Job struct {
    ArticleURL  string
    RevisionID  int64
    Markdown    string
    Tier        Tier
    SubmittedAt time.Time
    heapIndex   int  // internal for heap operations
}

type Result struct {
    HTML string
    Err  error
}

type RenderFunc func(markdown string) (string, error)

type Queue struct {
    render      RenderFunc
    mu          sync.Mutex
    heap        *jobHeap
    articleJobs map[string]*Job          // dedup by article URL
    waiters     map[string][]chan Result // notification channels
    jobReady    chan struct{}            // buffered(1), signals workers
    closed      bool
    closeCh     chan struct{}
    wg          sync.WaitGroup
    workerCount int
}
```

### Priority Ordering

Jobs are ordered by:
1. **Tier** (ascending): Interactive (0) before Background (1)
2. **SubmittedAt** (ascending): FIFO within tier

### Same-Article Deduplication

When a job is submitted for an article that already has a queued job:
- Update the existing job's RevisionID and Markdown in place
- Keep the original SubmittedAt (preserves FIFO position)
- Add the new waiter to the waiters list for that article URL

### Waiter Notification

- Waiters are keyed by article URL, not revision ID
- When a job completes, all waiters for that URL receive the result
- Non-blocking send (select with default) so abandoned waiters don't block
- Channels are buffered(1), owned by the caller

### Worker Pool

- Configurable worker count: detect via `runtime.NumCPU()`, config file override
- Workers block on `jobReady` channel when queue is empty
- Support context cancellation for graceful shutdown

### Graceful Shutdown

```go
func (q *Queue) Shutdown(ctx context.Context) error
```

- Set closed flag, close signal channel
- Workers finish current job, then exit
- Wait for all workers with timeout from context

## Database Changes

### New Column: `render_status`

Add to Revision table:

```sql
render_status TEXT NOT NULL DEFAULT 'rendered'
```

Values:
- `queued` - Revision created, render pending
- `rendered` - HTML is current
- `stale` - Needs re-render (template changed, etc.)
- `failed` - Render failed (for debugging)

### Migration Strategy

For existing databases, add ALTER TABLE in schema execution:

```sql
ALTER TABLE Revision ADD COLUMN render_status TEXT NOT NULL DEFAULT 'rendered';
```

Schema.sql updated for new databases.

## Configuration

Add to config.yaml:

```yaml
render_workers: 0  # 0 = auto-detect (NumCPU)
```

## Flow: PostArticle

```
1. Validate hash, sanitize title/comment
2. Insert revision to DB with render_status='queued', html=''
3. Submit job to queue (TierInteractive), get wait channel
4. Wait for result (with request context timeout)
5. Update revision: html=result.HTML, render_status='rendered'
6. If render failed: render_status='failed', log error, return error
```

## Flow: Preview

```
1. Call RenderingService.Render() directly (no queue)
2. Return HTML
```

No persistence, no queueing - previews are ephemeral.

## Concurrency Design

### Synchronization

- `sync.Mutex` protects all queue state (writes dominate, RWMutex not beneficial)
- Buffered channel `jobReady` for worker signaling (supports context cancellation)
- Waiter notification happens **outside** the lock to prevent deadlock

### Critical Sections

1. **Submit**: Check dedup, update/insert job, add waiter, signal worker
2. **Pop**: Wait for job, remove from heap and articleJobs map
3. **Notify**: Get waiters under lock, send results outside lock

### Error Handling

- Worker panics: recover, notify waiters with error, continue
- Render errors: notify waiters with error result
- Queue closed: Submit returns ErrQueueClosed

## Testing Strategy

### Unit Tests (`internal/renderqueue/queue_test.go`)

- Basic submit and receive
- Priority ordering (interactive before background)
- Same-article deduplication
- Multiple waiters for same article
- Concurrent submit/pop operations
- Graceful shutdown
- Worker panic recovery

### Integration Tests

- PostArticle creates revision, queues render, updates HTML
- Preview bypasses queue
- Shutdown drains in-flight jobs

## Files to Create/Modify

### New Files
- `internal/renderqueue/queue.go` - Queue implementation
- `internal/renderqueue/queue_test.go` - Unit tests
- `internal/renderqueue/heap.go` - Priority queue (heap.Interface)

### Modified Files
- `internal/storage/schema.sql` - Add render_status column
- `internal/storage/sqlite.go` - Migration for existing DBs
- `config.go` - Add render_workers config
- `config.yaml` - Add render_workers setting
- `setup.go` - Create queue, inject into ArticleService
- `server.go` - Graceful shutdown handling
- `wiki/service/article.go` - Use queue for PostArticle, add Preview method
- `wiki/repository/article.go` - Update methods for render_status

## Implementation Order

1. Queue core (with tests) - heap, submit, pop, workers
2. Database schema changes
3. Configuration
4. ArticleService integration
5. Graceful shutdown in server.go
6. Integration tests
