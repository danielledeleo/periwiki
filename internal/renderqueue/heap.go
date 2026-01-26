package renderqueue

import "container/heap"

// jobHeap implements heap.Interface for priority queue ordering.
// Jobs are ordered by:
// 1. Tier (ascending): Interactive (0) before Background (1)
// 2. SubmittedAt (ascending): FIFO within same tier
type jobHeap []*Job

var _ heap.Interface = (*jobHeap)(nil)

func (h jobHeap) Len() int { return len(h) }

func (h jobHeap) Less(i, j int) bool {
	// First compare by tier (lower tier = higher priority)
	if h[i].Tier != h[j].Tier {
		return h[i].Tier < h[j].Tier
	}
	// Within same tier, earlier SubmittedAt = higher priority (FIFO)
	return h[i].SubmittedAt.Before(h[j].SubmittedAt)
}

func (h jobHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].heapIndex = i
	h[j].heapIndex = j
}

func (h *jobHeap) Push(x any) {
	n := len(*h)
	job := x.(*Job)
	job.heapIndex = n
	*h = append(*h, job)
}

func (h *jobHeap) Pop() any {
	old := *h
	n := len(old)
	job := old[n-1]
	old[n-1] = nil    // avoid memory leak
	job.heapIndex = -1 // for safety
	*h = old[0 : n-1]
	return job
}

// Fix re-establishes the heap ordering after the element at index i has changed.
// This is a convenience wrapper around heap.Fix.
func (h *jobHeap) Fix(i int) {
	heap.Fix(h, i)
}
