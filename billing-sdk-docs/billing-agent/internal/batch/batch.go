// Package batch buffers free-quota "report" events and flushes them to core's
// idempotent batch endpoint on a size/interval trigger (and on shutdown), so the
// durable billing ledger is written without one core call per consume. The gate
// counter is NOT batched — authorize stays synchronous (see pdp). Money-funded
// usage is never routed here; only in-quota reports are.
package batch

import (
	"context"
	"sync"
	"time"

	"github.com/simplify/billing-agent/internal/upstream"
)

type Batcher struct {
	up       *upstream.Client
	size     int
	interval time.Duration
	onFlush  func(orgs []string) // invalidate caches for flushed orgs (eventual read accuracy)

	mu   sync.Mutex
	buf  []upstream.BatchEvent
	stop chan struct{}
	done chan struct{}
}

func New(up *upstream.Client, size int, interval time.Duration, onFlush func([]string)) *Batcher {
	b := &Batcher{up: up, size: size, interval: interval, onFlush: onFlush, stop: make(chan struct{}), done: make(chan struct{})}
	go b.loop()
	return b
}

// Enabled reports whether batching is on (size>0 && interval>0).
func (b *Batcher) Enabled() bool { return b.size > 0 && b.interval > 0 }

// Add buffers a usage event; flushes immediately if the buffer is full.
func (b *Batcher) Add(ev upstream.BatchEvent) {
	b.mu.Lock()
	b.buf = append(b.buf, ev)
	full := len(b.buf) >= b.size
	b.mu.Unlock()
	if full {
		b.Flush(context.Background())
	}
}

func (b *Batcher) loop() {
	defer close(b.done)
	t := time.NewTicker(b.interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			b.Flush(context.Background())
		case <-b.stop:
			b.Flush(context.Background()) // final flush on shutdown — no lost consumption
			return
		}
	}
}

// Flush sends the buffered events to core. On error the events are re-queued so a
// transient core outage doesn't drop usage (at-least-once; idempotency keys dedup).
func (b *Batcher) Flush(ctx context.Context) {
	b.mu.Lock()
	if len(b.buf) == 0 {
		b.mu.Unlock()
		return
	}
	events := b.buf
	b.buf = nil
	b.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	status, err := b.up.ConsumeBatch(ctx, events)
	if err != nil || status >= 500 {
		b.mu.Lock()
		b.buf = append(events, b.buf...) // re-queue, preserve order
		b.mu.Unlock()
		return
	}
	if b.onFlush != nil {
		seen := map[string]bool{}
		orgs := make([]string, 0, len(events))
		for _, e := range events {
			if !seen[e.OrgID] {
				seen[e.OrgID] = true
				orgs = append(orgs, e.OrgID)
			}
		}
		b.onFlush(orgs)
	}
}

// Close stops the loop and performs a final flush.
func (b *Batcher) Close() {
	close(b.stop)
	<-b.done
}
