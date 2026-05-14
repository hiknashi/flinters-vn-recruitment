package aggregate

import (
	"hash/maphash"
	"sync"

	"adperf/internal/parse"
)

// shard is one independently locked slice of the campaign keyspace.
type shard struct {
	mu   sync.Mutex
	byID map[string]*Stats
}

// ShardedAggregator holds campaign totals partitioned across many shards. Each
// campaign ID hashes to exactly one shard, so the combined memory is
// O(distinct campaign IDs) no matter how many workers feed it concurrently —
// there is no per-worker copy of the full keyspace.
type ShardedAggregator struct {
	seed   maphash.Seed
	shards []*shard
	mask   uint64
}

// NewShardedAggregator creates an aggregator whose shard count is shardCount
// rounded up to a power of two (minimum 1). More shards means less lock
// contention between workers.
func NewShardedAggregator(shardCount int) *ShardedAggregator {
	n := 1
	for n < shardCount {
		n <<= 1
	}
	shards := make([]*shard, n)
	for i := range shards {
		shards[i] = &shard{byID: make(map[string]*Stats)}
	}
	return &ShardedAggregator{
		seed:   maphash.MakeSeed(),
		shards: shards,
		mask:   uint64(n - 1),
	}
}

func (sa *ShardedAggregator) shardFor(id string) *shard {
	return sa.shards[maphash.String(sa.seed, id)&sa.mask]
}

// merge folds the totals for one campaign into its owning shard. When the shard
// has no entry yet it adopts src directly, avoiding a copy.
func (sa *ShardedAggregator) merge(id string, src *Stats) {
	sh := sa.shardFor(id)
	sh.mu.Lock()
	if dst := sh.byID[id]; dst != nil {
		dst.merge(src)
	} else {
		sh.byID[id] = src
	}
	sh.mu.Unlock()
}

// collect returns every Stats matching keep (a nil keep returns all of them).
// It is intended to be called after all writers have finished.
func (sa *ShardedAggregator) collect(keep func(*Stats) bool) []*Stats {
	var out []*Stats
	for _, sh := range sa.shards {
		sh.mu.Lock()
		for _, s := range sh.byID {
			if keep == nil || keep(s) {
				out = append(out, s)
			}
		}
		sh.mu.Unlock()
	}
	return out
}

// defaultFlushThreshold caps how many distinct campaigns a Combiner holds
// locally before flushing into the shared aggregator.
const defaultFlushThreshold = 1 << 16

// Combiner is a per-worker pre-aggregator. A worker folds rows into a small
// local map and periodically flushes it into the shared ShardedAggregator.
// This keeps lock traffic low — one lock acquisition per flushed key rather
// than per row — while bounding each worker's memory to the flush threshold.
type Combiner struct {
	agg       *ShardedAggregator
	local     map[string]*Stats
	threshold int
}

// NewCombiner returns a Combiner that flushes into sa.
func (sa *ShardedAggregator) NewCombiner() *Combiner {
	return &Combiner{
		agg:       sa,
		local:     make(map[string]*Stats),
		threshold: defaultFlushThreshold,
	}
}

// Add folds one row into the worker-local map, flushing if the map has grown
// to the threshold. The map lookup uses string(r.CampaignID) directly, which
// the compiler performs without allocating; a string is only allocated when a
// genuinely new campaign is inserted.
func (c *Combiner) Add(r parse.Row) {
	s := c.local[string(r.CampaignID)]
	if s == nil {
		id := string(r.CampaignID)
		s = &Stats{CampaignID: id}
		c.local[id] = s
	}
	s.Add(r.Impressions, r.Clicks, r.Conversions, r.Spend)
	if len(c.local) >= c.threshold {
		c.Flush()
	}
}

// Flush merges all worker-local totals into the shared aggregator and resets
// the local map. It must be called once after the worker's final Add.
func (c *Combiner) Flush() {
	for id, s := range c.local {
		c.agg.merge(id, s)
	}
	c.local = make(map[string]*Stats)
}
