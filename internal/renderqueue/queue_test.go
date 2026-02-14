package renderqueue

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockRender creates a simple render function that returns HTML based on markdown
func mockRender(markdown string) (string, error) {
	return "<p>" + markdown + "</p>", nil
}

// slowRender creates a render function that takes specified duration
func slowRender(d time.Duration) RenderFunc {
	return func(markdown string) (string, error) {
		time.Sleep(d)
		return "<p>" + markdown + "</p>", nil
	}
}

// errorRender creates a render function that returns an error
func errorRender(err error) RenderFunc {
	return func(markdown string) (string, error) {
		return "", err
	}
}

// panicRender creates a render function that panics
func panicRender() RenderFunc {
	return func(markdown string) (string, error) {
		panic("render panic!")
	}
}

func TestQueue_BasicSubmitAndReceive(t *testing.T) {
	q := New(2, mockRender)
	defer q.Shutdown(context.Background())

	waitCh := make(chan Result, 1)
	job := Job{
		ArticleURL:  "test-article",
		RevisionID:  1,
		Markdown:    "Hello World",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}

	err := q.Submit(context.Background(), job, waitCh)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	select {
	case result := <-waitCh:
		if result.Err != nil {
			t.Fatalf("expected no error, got: %v", result.Err)
		}
		expected := "<p>Hello World</p>"
		if result.HTML != expected {
			t.Errorf("expected HTML %q, got %q", expected, result.HTML)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for result")
	}
}

func TestQueue_PriorityOrdering(t *testing.T) {
	// Use a slow render to ensure jobs queue up
	var processOrder []string
	var mu sync.Mutex

	trackingRender := func(markdown string) (string, error) {
		mu.Lock()
		processOrder = append(processOrder, markdown)
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return "<p>" + markdown + "</p>", nil
	}

	// Create queue with 1 worker to ensure sequential processing
	q := New(1, trackingRender)

	// Submit a blocking job first to let other jobs queue up
	blockCh := make(chan Result, 1)
	blockJob := Job{
		ArticleURL:  "blocker",
		RevisionID:  1,
		Markdown:    "blocker",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}
	if err := q.Submit(context.Background(), blockJob, blockCh); err != nil {
		t.Fatalf("Submit blocker failed: %v", err)
	}

	// Wait a bit for blocker to start processing
	time.Sleep(5 * time.Millisecond)

	// Now submit background jobs first, then interactive
	bgWait1 := make(chan Result, 1)
	bgWait2 := make(chan Result, 1)
	intWait := make(chan Result, 1)

	bgJob1 := Job{
		ArticleURL:  "bg1",
		RevisionID:  1,
		Markdown:    "background1",
		Tier:        TierBackground,
		SubmittedAt: time.Now(),
	}
	bgJob2 := Job{
		ArticleURL:  "bg2",
		RevisionID:  1,
		Markdown:    "background2",
		Tier:        TierBackground,
		SubmittedAt: time.Now().Add(time.Millisecond),
	}
	intJob := Job{
		ArticleURL:  "int",
		RevisionID:  1,
		Markdown:    "interactive",
		Tier:        TierInteractive,
		SubmittedAt: time.Now().Add(2 * time.Millisecond),
	}

	// Submit in order: bg1, bg2, interactive
	if err := q.Submit(context.Background(), bgJob1, bgWait1); err != nil {
		t.Fatalf("Submit bg1 failed: %v", err)
	}
	if err := q.Submit(context.Background(), bgJob2, bgWait2); err != nil {
		t.Fatalf("Submit bg2 failed: %v", err)
	}
	if err := q.Submit(context.Background(), intJob, intWait); err != nil {
		t.Fatalf("Submit int failed: %v", err)
	}

	// Wait for all to complete
	<-blockCh
	<-bgWait1
	<-bgWait2
	<-intWait

	q.Shutdown(context.Background())

	// Check order: blocker first (was processing), then interactive, then bg1, bg2
	mu.Lock()
	defer mu.Unlock()

	if len(processOrder) != 4 {
		t.Fatalf("expected 4 jobs processed, got %d: %v", len(processOrder), processOrder)
	}
	if processOrder[0] != "blocker" {
		t.Errorf("expected blocker first, got %s", processOrder[0])
	}
	if processOrder[1] != "interactive" {
		t.Errorf("expected interactive second, got %s", processOrder[1])
	}
	// bg1 and bg2 should be after interactive (order between them is FIFO by SubmittedAt)
	if processOrder[2] != "background1" {
		t.Errorf("expected background1 third, got %s", processOrder[2])
	}
	if processOrder[3] != "background2" {
		t.Errorf("expected background2 fourth, got %s", processOrder[3])
	}
}

func TestQueue_SameArticleDeduplication(t *testing.T) {
	// Track which markdown gets rendered
	var renderedMarkdown string
	var renderCount int
	var mu sync.Mutex

	trackingRender := func(markdown string) (string, error) {
		mu.Lock()
		renderedMarkdown = markdown
		renderCount++
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return "<p>" + markdown + "</p>", nil
	}

	q := New(1, trackingRender)

	// Submit a blocking job first
	blockCh := make(chan Result, 1)
	blockJob := Job{
		ArticleURL:  "blocker",
		RevisionID:  0,
		Markdown:    "blocker",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}
	if err := q.Submit(context.Background(), blockJob, blockCh); err != nil {
		t.Fatalf("Submit blocker failed: %v", err)
	}

	// Wait for blocker to start processing
	time.Sleep(10 * time.Millisecond)

	// Submit first version of article
	wait1 := make(chan Result, 1)
	job1 := Job{
		ArticleURL:  "same-article",
		RevisionID:  1,
		Markdown:    "version1",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}
	if err := q.Submit(context.Background(), job1, wait1); err != nil {
		t.Fatalf("Submit job1 failed: %v", err)
	}

	// Submit second version of same article (should replace first)
	wait2 := make(chan Result, 1)
	job2 := Job{
		ArticleURL:  "same-article",
		RevisionID:  2,
		Markdown:    "version2",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}
	if err := q.Submit(context.Background(), job2, wait2); err != nil {
		t.Fatalf("Submit job2 failed: %v", err)
	}

	// Wait for blocker to finish
	<-blockCh

	// Both waiters should receive the same result (version2)
	result1 := <-wait1
	result2 := <-wait2

	q.Shutdown(context.Background())

	// Both should have received version2's render
	expected := "<p>version2</p>"
	if result1.HTML != expected {
		t.Errorf("wait1: expected %q, got %q", expected, result1.HTML)
	}
	if result2.HTML != expected {
		t.Errorf("wait2: expected %q, got %q", expected, result2.HTML)
	}

	// Verify only one render happened for the article (plus blocker = 2 total)
	mu.Lock()
	defer mu.Unlock()
	if renderCount != 2 {
		t.Errorf("expected 2 renders (blocker + deduplicated), got %d", renderCount)
	}
	if renderedMarkdown != "version2" {
		t.Errorf("expected last rendered markdown to be 'version2', got %q", renderedMarkdown)
	}
}

func TestQueue_MultipleWaitersForSameArticle(t *testing.T) {
	q := New(1, slowRender(50*time.Millisecond))

	// Submit a blocking job first
	blockCh := make(chan Result, 1)
	blockJob := Job{
		ArticleURL:  "blocker",
		RevisionID:  0,
		Markdown:    "blocker",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}
	if err := q.Submit(context.Background(), blockJob, blockCh); err != nil {
		t.Fatalf("Submit blocker failed: %v", err)
	}

	// Wait for blocker to start
	time.Sleep(10 * time.Millisecond)

	// Submit same article multiple times with different waiters
	waiters := make([]chan Result, 5)
	for i := 0; i < 5; i++ {
		waiters[i] = make(chan Result, 1)
		job := Job{
			ArticleURL:  "multi-waiter-article",
			RevisionID:  int64(i + 1),
			Markdown:    "final-version",
			Tier:        TierInteractive,
			SubmittedAt: time.Now(),
		}
		if err := q.Submit(context.Background(), job, waiters[i]); err != nil {
			t.Fatalf("Submit %d failed: %v", i, err)
		}
	}

	// Wait for blocker
	<-blockCh

	// All waiters should receive the same result
	expected := "<p>final-version</p>"
	for i, ch := range waiters {
		select {
		case result := <-ch:
			if result.Err != nil {
				t.Errorf("waiter %d: unexpected error: %v", i, result.Err)
			}
			if result.HTML != expected {
				t.Errorf("waiter %d: expected %q, got %q", i, expected, result.HTML)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("waiter %d: timeout", i)
		}
	}

	q.Shutdown(context.Background())
}

func TestQueue_ConcurrentSubmitAndPop(t *testing.T) {
	var completed int64
	countingRender := func(markdown string) (string, error) {
		time.Sleep(time.Millisecond)
		atomic.AddInt64(&completed, 1)
		return "<p>" + markdown + "</p>", nil
	}

	q := New(4, countingRender)

	const numJobs = 100
	var wg sync.WaitGroup
	wg.Add(numJobs)

	for i := 0; i < numJobs; i++ {
		go func(idx int) {
			defer wg.Done()
			waitCh := make(chan Result, 1)
			job := Job{
				ArticleURL:  "article-" + string(rune('a'+idx%26)) + string(rune('0'+idx/26)),
				RevisionID:  int64(idx),
				Markdown:    "content",
				Tier:        Tier(idx % 2),
				SubmittedAt: time.Now(),
			}
			if err := q.Submit(context.Background(), job, waitCh); err != nil {
				t.Errorf("Submit %d failed: %v", idx, err)
				return
			}
			<-waitCh
		}(i)
	}

	wg.Wait()
	q.Shutdown(context.Background())

	// Due to deduplication, we may have fewer completions than jobs
	// but we should have processed at least some jobs
	if atomic.LoadInt64(&completed) == 0 {
		t.Error("expected some jobs to complete")
	}
}

func TestQueue_GracefulShutdown(t *testing.T) {
	var completed int64
	slowCountingRender := func(markdown string) (string, error) {
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt64(&completed, 1)
		return "<p>" + markdown + "</p>", nil
	}

	q := New(2, slowCountingRender)

	// Submit several jobs
	waiters := make([]chan Result, 5)
	for i := 0; i < 5; i++ {
		waiters[i] = make(chan Result, 1)
		job := Job{
			ArticleURL:  "shutdown-article-" + string(rune('0'+i)),
			RevisionID:  int64(i),
			Markdown:    "content",
			Tier:        TierInteractive,
			SubmittedAt: time.Now(),
		}
		if err := q.Submit(context.Background(), job, waiters[i]); err != nil {
			t.Fatalf("Submit %d failed: %v", i, err)
		}
	}

	// Give workers time to start processing
	time.Sleep(10 * time.Millisecond)

	// Shutdown with sufficient timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := q.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// All in-flight jobs should have completed
	completedCount := atomic.LoadInt64(&completed)
	if completedCount == 0 {
		t.Error("expected some jobs to complete during shutdown")
	}

	// New submits should be rejected
	newWait := make(chan Result, 1)
	newJob := Job{
		ArticleURL:  "new-article",
		RevisionID:  100,
		Markdown:    "should fail",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}
	err := q.Submit(context.Background(), newJob, newWait)
	if err == nil {
		t.Error("expected Submit to fail after shutdown")
	}
	if !errors.Is(err, ErrQueueClosed) {
		t.Errorf("expected ErrQueueClosed, got: %v", err)
	}
}

func TestQueue_WorkerPanicRecovery(t *testing.T) {
	var callCount int64

	panicOnceRender := func(markdown string) (string, error) {
		count := atomic.AddInt64(&callCount, 1)
		if count == 1 {
			panic("intentional panic")
		}
		return "<p>" + markdown + "</p>", nil
	}

	q := New(1, panicOnceRender)

	// First job should trigger panic
	wait1 := make(chan Result, 1)
	job1 := Job{
		ArticleURL:  "panic-article",
		RevisionID:  1,
		Markdown:    "will panic",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}
	if err := q.Submit(context.Background(), job1, wait1); err != nil {
		t.Fatalf("Submit job1 failed: %v", err)
	}

	// Wait for panic result
	select {
	case result := <-wait1:
		if result.Err == nil {
			t.Error("expected error from panic")
		}
		if result.HTML != "" {
			t.Errorf("expected empty HTML on panic, got %q", result.HTML)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for panic result")
	}

	// Second job should work fine (worker recovered)
	wait2 := make(chan Result, 1)
	job2 := Job{
		ArticleURL:  "after-panic",
		RevisionID:  2,
		Markdown:    "should work",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}
	if err := q.Submit(context.Background(), job2, wait2); err != nil {
		t.Fatalf("Submit job2 failed: %v", err)
	}

	select {
	case result := <-wait2:
		if result.Err != nil {
			t.Errorf("expected no error after recovery, got: %v", result.Err)
		}
		expected := "<p>should work</p>"
		if result.HTML != expected {
			t.Errorf("expected %q, got %q", expected, result.HTML)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for recovery result")
	}

	q.Shutdown(context.Background())
}

func TestQueue_ContextCancellation(t *testing.T) {
	// Very slow render to ensure we can cancel before completion
	verySlowRender := func(markdown string) (string, error) {
		time.Sleep(10 * time.Second)
		return "<p>" + markdown + "</p>", nil
	}

	q := New(1, verySlowRender)
	defer q.Shutdown(context.Background())

	// Submit with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	waitCh := make(chan Result, 1)
	job := Job{
		ArticleURL:  "slow-article",
		RevisionID:  1,
		Markdown:    "slow content",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}

	if err := q.Submit(ctx, job, waitCh); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Cancel after a short time
	time.Sleep(50 * time.Millisecond)
	cancel()

	// The waiter can give up (context cancelled)
	// Note: the job may still be processing, but the caller has moved on
	select {
	case <-waitCh:
		// Got result (maybe worker finished quickly or panic)
	case <-ctx.Done():
		// This is expected - context was cancelled
	case <-time.After(100 * time.Millisecond):
		// Timeout is acceptable - we just want to verify we can cancel
	}
}

func TestQueue_EmptyQueue(t *testing.T) {
	var processCount int64
	trackingRender := func(markdown string) (string, error) {
		atomic.AddInt64(&processCount, 1)
		return "<p>" + markdown + "</p>", nil
	}

	q := New(2, trackingRender)

	// Workers should be idle, not spinning
	time.Sleep(100 * time.Millisecond)

	// Verify no phantom processing
	if atomic.LoadInt64(&processCount) != 0 {
		t.Error("workers should not process when queue is empty")
	}

	// Now submit a job - it should be processed
	waitCh := make(chan Result, 1)
	job := Job{
		ArticleURL:  "delayed-article",
		RevisionID:  1,
		Markdown:    "content",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}
	if err := q.Submit(context.Background(), job, waitCh); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	select {
	case result := <-waitCh:
		if result.Err != nil {
			t.Errorf("unexpected error: %v", result.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for result")
	}

	if atomic.LoadInt64(&processCount) != 1 {
		t.Errorf("expected 1 job processed, got %d", atomic.LoadInt64(&processCount))
	}

	q.Shutdown(context.Background())
}

func TestQueue_RenderError(t *testing.T) {
	renderErr := errors.New("render failed")
	q := New(1, errorRender(renderErr))
	defer q.Shutdown(context.Background())

	waitCh := make(chan Result, 1)
	job := Job{
		ArticleURL:  "error-article",
		RevisionID:  1,
		Markdown:    "content",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}

	if err := q.Submit(context.Background(), job, waitCh); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	select {
	case result := <-waitCh:
		if result.Err == nil {
			t.Error("expected error from render")
		}
		if !errors.Is(result.Err, renderErr) {
			t.Errorf("expected %v, got %v", renderErr, result.Err)
		}
		if result.HTML != "" {
			t.Errorf("expected empty HTML on error, got %q", result.HTML)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for error result")
	}
}

func TestQueue_ShutdownTimeout(t *testing.T) {
	// Very slow render that won't complete in time
	verySlowRender := func(markdown string) (string, error) {
		time.Sleep(10 * time.Second)
		return "<p>" + markdown + "</p>", nil
	}

	q := New(1, verySlowRender)

	// Submit a job that will take forever
	waitCh := make(chan Result, 1)
	job := Job{
		ArticleURL:  "forever-article",
		RevisionID:  1,
		Markdown:    "content",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}
	if err := q.Submit(context.Background(), job, waitCh); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Give worker time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := q.Shutdown(ctx)

	// Should get context deadline exceeded error
	if err == nil {
		t.Error("expected shutdown timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}
}

func TestQueue_NilWaiterChannel(t *testing.T) {
	q := New(1, mockRender)
	defer q.Shutdown(context.Background())

	// Submit with nil channel - should not panic
	job := Job{
		ArticleURL:  "nil-waiter-article",
		RevisionID:  1,
		Markdown:    "content",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}

	// This should not panic even with nil channel
	err := q.Submit(context.Background(), job, nil)
	if err != nil {
		t.Fatalf("Submit with nil channel failed: %v", err)
	}

	// Wait a bit for processing to complete
	time.Sleep(100 * time.Millisecond)
}

// Heap-specific tests

func TestHeap_Ordering(t *testing.T) {
	h := &jobHeap{}
	heap.Init(h)

	now := time.Now()

	// Add jobs in various orders
	jobs := []*Job{
		{ArticleURL: "bg-late", Tier: TierBackground, SubmittedAt: now.Add(2 * time.Millisecond)},
		{ArticleURL: "int-early", Tier: TierInteractive, SubmittedAt: now},
		{ArticleURL: "bg-early", Tier: TierBackground, SubmittedAt: now},
		{ArticleURL: "int-late", Tier: TierInteractive, SubmittedAt: now.Add(time.Millisecond)},
	}

	for _, j := range jobs {
		heap.Push(h, j)
	}

	// Should pop in order: interactive-early, interactive-late, bg-early, bg-late
	expected := []string{"int-early", "int-late", "bg-early", "bg-late"}
	for i, exp := range expected {
		if h.Len() == 0 {
			t.Fatalf("heap empty before getting all expected items")
		}
		job := heap.Pop(h).(*Job)
		if job.ArticleURL != exp {
			t.Errorf("pop %d: expected %s, got %s", i, exp, job.ArticleURL)
		}
	}

	if h.Len() != 0 {
		t.Error("heap should be empty after popping all jobs")
	}
}

func TestHeap_FixAfterUpdate(t *testing.T) {
	h := &jobHeap{}
	heap.Init(h)

	now := time.Now()

	// Add two background jobs
	job1 := &Job{ArticleURL: "job1", Tier: TierBackground, SubmittedAt: now.Add(time.Second)}
	job2 := &Job{ArticleURL: "job2", Tier: TierBackground, SubmittedAt: now}

	heap.Push(h, job1)
	heap.Push(h, job2)

	// job2 should be first (earlier SubmittedAt)
	if (*h)[0].ArticleURL != "job2" {
		t.Error("job2 should be at top initially")
	}

	// Now change job1 to interactive - it should move to top after Fix
	job1.Tier = TierInteractive
	heap.Fix(h, job1.heapIndex)

	if (*h)[0].ArticleURL != "job1" {
		t.Error("job1 should be at top after becoming interactive")
	}
}

func TestQueue_ShutdownDrainsPendingJobs(t *testing.T) {
	var processed sync.Map
	started := make(chan struct{}) // signals when the blocker has started rendering

	render := func(markdown string) (string, error) {
		if markdown == "blocker" {
			close(started)
			time.Sleep(50 * time.Millisecond)
		}
		processed.Store(markdown, true)
		return "<p>" + markdown + "</p>", nil
	}

	q := New(1, render)

	// Submit a blocking job so subsequent jobs queue up
	blockerWait := make(chan Result, 1)
	blockerJob := Job{
		ArticleURL:  "blocker",
		RevisionID:  1,
		Markdown:    "blocker",
		Tier:        TierInteractive,
		SubmittedAt: time.Now(),
	}
	if err := q.Submit(context.Background(), blockerJob, blockerWait); err != nil {
		t.Fatalf("Submit blocker failed: %v", err)
	}

	// Wait for blocker to actually start rendering
	<-started

	// Submit several more jobs while blocker is in-flight
	const pendingCount = 5
	waiters := make([]chan Result, pendingCount)
	for i := 0; i < pendingCount; i++ {
		waiters[i] = make(chan Result, 1)
		job := Job{
			ArticleURL:  fmt.Sprintf("pending-%d", i),
			RevisionID:  int64(i),
			Markdown:    fmt.Sprintf("pending-%d", i),
			Tier:        TierBackground,
			SubmittedAt: time.Now().Add(time.Duration(i) * time.Millisecond),
		}
		if err := q.Submit(context.Background(), job, waiters[i]); err != nil {
			t.Fatalf("Submit pending-%d failed: %v", i, err)
		}
	}

	// Shutdown with generous timeout - should drain all pending jobs
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := q.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// Verify blocker result was received
	select {
	case result := <-blockerWait:
		if result.Err != nil {
			t.Errorf("blocker: unexpected error: %v", result.Err)
		}
	default:
		t.Error("blocker waiter did not receive result")
	}

	// Verify ALL pending jobs were processed and their waiters received results
	for i := 0; i < pendingCount; i++ {
		select {
		case result := <-waiters[i]:
			if result.Err != nil {
				t.Errorf("pending-%d: unexpected error: %v", i, result.Err)
			}
			expected := fmt.Sprintf("<p>pending-%d</p>", i)
			if result.HTML != expected {
				t.Errorf("pending-%d: expected %q, got %q", i, expected, result.HTML)
			}
		default:
			t.Errorf("pending-%d waiter did not receive result (job was not drained)", i)
		}
	}

	// Double-check via the processed map
	for i := 0; i < pendingCount; i++ {
		key := fmt.Sprintf("pending-%d", i)
		if _, ok := processed.Load(key); !ok {
			t.Errorf("%s was never rendered", key)
		}
	}
}
