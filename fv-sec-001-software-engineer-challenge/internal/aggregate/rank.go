package aggregate

import "container/heap"

// worseFunc reports whether a ranks below b — that is, whether a is the better
// candidate for eviction when keeping only the top results.
type worseFunc func(a, b *Stats) bool

// minHeap keeps the k best Stats seen so far; by the worse ordering its root is
// the worst of those k, so it is the first to be replaced.
type minHeap struct {
	items []*Stats
	worse worseFunc
}

func (h minHeap) Len() int           { return len(h.items) }
func (h minHeap) Less(i, j int) bool { return h.worse(h.items[i], h.items[j]) }
func (h minHeap) Swap(i, j int)      { h.items[i], h.items[j] = h.items[j], h.items[i] }

func (h *minHeap) Push(x any) { h.items = append(h.items, x.(*Stats)) }

func (h *minHeap) Pop() any {
	old := h.items
	n := len(old)
	x := old[n-1]
	h.items = old[:n-1]
	return x
}

// topK returns up to k Stats from all, ordered best-first by worse. It runs in
// O(len(all) · log k) time and O(k) extra space, so the ranking step does not
// scale with the number of distinct campaigns beyond a single linear pass.
func topK(all []*Stats, k int, worse worseFunc) []*Stats {
	if k <= 0 {
		return nil
	}
	h := &minHeap{worse: worse}
	for _, s := range all {
		if h.Len() < k {
			heap.Push(h, s)
		} else if worse(h.items[0], s) {
			// s beats the current worst of the top k.
			h.items[0] = s
			heap.Fix(h, 0)
		}
	}
	out := make([]*Stats, h.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(h).(*Stats)
	}
	return out
}

// ctrWorse orders by CTR descending, breaking ties by campaign_id ascending so
// the output is deterministic.
func ctrWorse(a, b *Stats) bool {
	ca, cb := a.CTR(), b.CTR()
	if ca != cb {
		return ca < cb
	}
	return a.CampaignID > b.CampaignID
}

// cpaWorse orders by CPA ascending, breaking ties by campaign_id ascending. It
// assumes both campaigns have conversions > 0 (the caller filters the rest).
func cpaWorse(a, b *Stats) bool {
	ca, _ := a.CPA()
	cb, _ := b.CPA()
	if ca != cb {
		return ca > cb
	}
	return a.CampaignID > b.CampaignID
}

// TopByCTR returns the k campaigns with the highest CTR.
func (sa *ShardedAggregator) TopByCTR(k int) []*Stats {
	return topK(sa.collect(nil), k, ctrWorse)
}

// TopByCPA returns the k campaigns with the lowest CPA, excluding campaigns
// with zero conversions for which CPA is undefined.
func (sa *ShardedAggregator) TopByCPA(k int) []*Stats {
	return topK(sa.collect(hasConversions), k, cpaWorse)
}

func hasConversions(s *Stats) bool { return s.Conversions > 0 }
