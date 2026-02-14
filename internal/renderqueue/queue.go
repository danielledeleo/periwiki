// Package renderqueue provides a priority queue for markdown-to-HTML rendering jobs.
// It supports priority ordering (interactive before background), same-article
// deduplication, and graceful shutdown with in-flight job completion.
package renderqueue

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrQueueClosed is returned when Submit is called on a closed queue.
var ErrQueueClosed = errors.New("render queue is closed")

// Tier represents the priority tier of a render job.
type Tier int

const (
	// TierInteractive is for user-facing requests (edits, views) - highest priority.
	TierInteractive Tier = iota
	// TierBackground is for bulk/scheduled re-renders - lower priority.
	TierBackground
)

// Job represents a markdown rendering job.
type Job struct {
	ArticleURL  string    // Unique identifier for deduplication
	RevisionID  int64     // Revision being rendered (updated on dedup)
	Markdown    string    // Content to render
	Tier        Tier      // Priority tier
	SubmittedAt time.Time // For FIFO ordering within tier
	heapIndex   int       // Internal index for heap operations
}

// Result contains the outcome of a render job.
type Result struct {
	HTML string // Rendered HTML (empty on error)
	Err  error  // Render error, if any
}

// RenderFunc is the function signature for markdown-to-HTML rendering.
type RenderFunc func(markdown string) (string, error)

// Queue manages a pool of workers that process render jobs in priority order.
type Queue struct {
	render      RenderFunc
	mu          sync.Mutex
	heap        *jobHeap
	articleJobs map[string]*Job          // dedup by article URL
	waiters     map[string][]chan Result // notification channels by article URL
	jobReady    chan struct{}            // buffered(1), signals workers
	closed      bool
	closeCh     chan struct{}
	wg          sync.WaitGroup
	workerCount int
}

// New creates a new render queue with the specified number of workers.
// The render function is called to convert markdown to HTML.
func New(workerCount int, render RenderFunc) *Queue {
	if workerCount < 1 {
		workerCount = 1
	}

	q := &Queue{
		render:      render,
		heap:        &jobHeap{},
		articleJobs: make(map[string]*Job),
		waiters:     make(map[string][]chan Result),
		jobReady:    make(chan struct{}, 1),
		closeCh:     make(chan struct{}),
		workerCount: workerCount,
	}

	heap.Init(q.heap)

	// Start worker goroutines
	q.wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go q.worker()
	}

	return q
}

// Submit adds a job to the queue. If a job for the same article is already queued,
// the existing job is updated with the new revision and markdown, but keeps its
// queue position. The waiter channel (if non-nil) will receive the result when
// the job completes.
//
// Returns ErrQueueClosed if the queue has been shut down.
func (q *Queue) Submit(ctx context.Context, job Job, waitCh chan Result) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	// Check for existing job for this article (deduplication)
	if existing, ok := q.articleJobs[job.ArticleURL]; ok {
		// Update existing job in place (keeps queue position via SubmittedAt)
		existing.RevisionID = job.RevisionID
		existing.Markdown = job.Markdown
		// Note: we keep the original SubmittedAt to preserve FIFO position

		// If new job has higher priority (lower tier), update and fix heap
		if job.Tier < existing.Tier {
			existing.Tier = job.Tier
			q.heap.Fix(existing.heapIndex)
		}
	} else {
		// New job - add to heap and tracking map
		jobCopy := job // copy to avoid issues with caller modifying
		q.articleJobs[job.ArticleURL] = &jobCopy
		heap.Push(q.heap, &jobCopy)
	}

	// Add waiter if provided
	if waitCh != nil {
		q.waiters[job.ArticleURL] = append(q.waiters[job.ArticleURL], waitCh)
	}

	// Signal that a job is ready (non-blocking since channel is buffered)
	select {
	case q.jobReady <- struct{}{}:
	default:
	}

	return nil
}

// Shutdown gracefully shuts down the queue. It stops accepting new jobs,
// drains any pending jobs from the queue, waits for in-flight jobs to complete
// (up to context deadline), then returns. Returns context error if the deadline
// is exceeded.
func (q *Queue) Shutdown(ctx context.Context) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	q.closed = true
	close(q.closeCh)
	q.mu.Unlock()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// worker is the main worker loop that processes jobs from the queue.
func (q *Queue) worker() {
	defer q.wg.Done()

	for {
		// Wait for work or shutdown
		select {
		case <-q.closeCh:
			// Drain remaining jobs before exiting
			for q.processOneJob() {
			}
			return
		case <-q.jobReady:
			// Try to get and process a job
			q.processOneJob()
		}
	}
}

// processOneJob attempts to pop and process one job from the queue.
// Returns true if a job was processed, false if the queue was empty.
func (q *Queue) processOneJob() bool {
	// Pop job under lock
	q.mu.Lock()
	if q.heap.Len() == 0 {
		q.mu.Unlock()
		return false
	}

	job := heap.Pop(q.heap).(*Job)
	articleURL := job.ArticleURL
	markdown := job.Markdown
	delete(q.articleJobs, articleURL)

	// Get waiters for this job (will notify outside lock)
	jobWaiters := q.waiters[articleURL]
	delete(q.waiters, articleURL)

	// Check if more jobs are pending, signal next worker
	if q.heap.Len() > 0 {
		select {
		case q.jobReady <- struct{}{}:
		default:
		}
	}

	q.mu.Unlock()

	// Process job (outside lock)
	result := q.executeRender(markdown)

	// Notify all waiters (outside lock, non-blocking)
	for _, ch := range jobWaiters {
		if ch != nil {
			select {
			case ch <- result:
			default:
				// Waiter abandoned (buffer full or closed), skip
			}
		}
	}

	return true
}

// executeRender calls the render function with panic recovery.
func (q *Queue) executeRender(markdown string) Result {
	var result Result

	// Recover from panics in render function
	func() {
		defer func() {
			if r := recover(); r != nil {
				result = Result{
					HTML: "",
					Err:  fmt.Errorf("render panic: %v", r),
				}
			}
		}()

		html, err := q.render(markdown)
		result = Result{
			HTML: html,
			Err:  err,
		}
	}()

	return result
}
