package main

import (
	"context"
	"math"
	"slices"
	"time"

	"fiatjaf.com/nostr"
)

const (
	cacheCleanupFirstRun = time.Minute
	cacheCleanupInterval = time.Hour
	cacheScanPageSize    = 5000
)

type cachedEventUse struct {
	id       nostr.ID
	lastUsed nostr.Timestamp
}

// deleteOldCachedEvents keeps the event store bounded: events nobody looked at
// for cacheRetentionDays go away, and when there are still more than
// maxEvents the least recently used ones are dropped down to 90% of that,
// so a crawler walking through thousands of profiles can't fill the disk.
func deleteOldCachedEvents(ctx context.Context, cacheRetentionDays int, maxEvents int) {
	wait := cacheCleanupFirstRun
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
			wait = cacheCleanupInterval
			cleanupCachedEvents(ctx, cacheRetentionDays, maxEvents)
		}
	}
}

func cleanupCachedEvents(ctx context.Context, cacheRetentionDays int, maxEvents int) {
	start := time.Now()
	now := nostr.Now()
	threshold := now - nostr.Timestamp(60*60*24*cacheRetentionDays)

	events := scanCachedEvents(ctx, now)
	if ctx.Err() != nil {
		return
	}
	total := len(events)

	// least recently used first
	slices.SortFunc(events, func(a, b cachedEventUse) int {
		return int(a.lastUsed) - int(b.lastUsed)
	})

	expired := 0
	for expired < len(events) && events[expired].lastUsed < threshold {
		expired++
	}

	toDelete := expired
	if maxEvents > 0 && total-expired > maxEvents {
		toDelete = total - maxEvents*9/10
	}

	for _, e := range events[:toDelete] {
		if ctx.Err() != nil {
			return
		}
		deleteEvent(e.id)
	}

	log.Info().
		Int("total", total).
		Int("expired", expired).
		Int("deleted", toDelete).
		Int("max_events", maxEvents).
		Dur("took", time.Since(start)).
		Msg("cached events cleanup")
}

// scanCachedEvents reads every stored event in pages (asking the store for
// everything at once makes it allocate a buffer the size of the limit) and
// returns when each was last used. Events that were never tracked (fetched by
// the sdk in the background) get tracked now, so they live for the retention
// period from the first time we see them instead of being judged by created_at.
func scanCachedEvents(ctx context.Context, now nostr.Timestamp) []cachedEventUse {
	seen := make(map[nostr.ID]struct{})
	events := make([]cachedEventUse, 0, 1024)

	until := nostr.Timestamp(math.MaxUint32)
	for ctx.Err() == nil {
		n := 0
		oldest := until
		for evt := range sys.Store.QueryEvents(nostr.Filter{Until: until, Limit: cacheScanPageSize}, cacheScanPageSize) {
			n++
			if evt.CreatedAt < oldest {
				oldest = evt.CreatedAt
			}
			if _, ok := seen[evt.ID]; ok {
				continue
			}
			seen[evt.ID] = struct{}{}

			lastUsed := sys.GetEventAccessTime(evt.ID)
			if lastUsed == 0 {
				sys.TrackEventAccessTime(evt.ID)
				lastUsed = now
			}
			events = append(events, cachedEventUse{id: evt.ID, lastUsed: lastUsed})
		}

		if n < cacheScanPageSize {
			break
		}
		if oldest == until {
			// a whole page with the same created_at, step over it
			if until == 0 {
				break
			}
			until--
		} else {
			until = oldest
		}
	}

	return events
}
