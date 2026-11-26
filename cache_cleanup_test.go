package main

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/lmdb"
	"fiatjaf.com/nostr/sdk"
	bolt_kv "fiatjaf.com/nostr/sdk/kvstore/bbolt"
)

func setupCleanupTestSystem(t *testing.T) {
	t.Helper()
	dir := t.TempDir()

	db := &lmdb.LMDBBackend{Path: filepath.Join(dir, "db")}
	if err := db.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	kv, err := bolt_kv.NewStore(filepath.Join(dir, "kv"))
	if err != nil {
		t.Fatal(err)
	}

	sys = sdk.NewSystem()
	sys.Store = db
	sys.KVStore = kv
}

func saveTestEvents(t *testing.T, n int, createdAt func(i int) nostr.Timestamp) []nostr.ID {
	t.Helper()
	sk := nostr.Generate()
	ids := make([]nostr.ID, n)
	for i := range n {
		evt := nostr.Event{
			Kind:      1,
			CreatedAt: createdAt(i),
			Content:   fmt.Sprintf("test %d", i),
			Tags:      nostr.Tags{},
		}
		if err := evt.Sign(sk); err != nil {
			t.Fatal(err)
		}
		if err := sys.Store.SaveEvent(evt); err != nil {
			t.Fatal(err)
		}
		ids[i] = evt.ID
	}
	return ids
}

func countStored() int {
	n := 0
	until := nostr.Timestamp(1<<32 - 1)
	for evt := range sys.Store.QueryEvents(nostr.Filter{Until: until}, 1_000_000) {
		_ = evt
		n++
	}
	return n
}

func TestScanCachedEventsPagesThroughEverything(t *testing.T) {
	setupCleanupTestSystem(t)

	// more than one page, many sharing the same created_at
	n := cacheScanPageSize*2 + 123
	saveTestEvents(t, n, func(i int) nostr.Timestamp { return nostr.Timestamp(1_700_000_000 + i/7) })

	events := scanCachedEvents(context.Background(), nostr.Now())
	if len(events) != n {
		t.Fatalf("scanned %d events, want %d", len(events), n)
	}
}

func TestCleanupCachedEventsCapsSize(t *testing.T) {
	setupCleanupTestSystem(t)

	now := nostr.Now()
	ids := saveTestEvents(t, 1000, func(i int) nostr.Timestamp { return now - nostr.Timestamp(i) })

	// the first 100 were looked at recently, the rest a while ago
	for i, id := range ids {
		key := id
		if i < 100 {
			sys.TrackEventAccessTime(key)
		} else {
			sys.KVStore.Set(append([]byte{'a'}, key[:8]...), encodeTestTimestamp(now-3600))
		}
	}

	cleanupCachedEvents(context.Background(), 13, 500)

	if got := countStored(); got != 450 {
		t.Fatalf("got %d events after cleanup, want 450", got)
	}
	for _, id := range ids[:100] {
		for range sys.Store.QueryEvents(nostr.Filter{IDs: []nostr.ID{id}}, 1) {
			goto found
		}
		t.Fatalf("recently used event %s was deleted", id)
	found:
	}
}

func TestCleanupCachedEventsRetention(t *testing.T) {
	setupCleanupTestSystem(t)

	now := nostr.Now()
	ids := saveTestEvents(t, 10, func(i int) nostr.Timestamp { return now })
	for i, id := range ids {
		if i < 4 {
			sys.KVStore.Set(append([]byte{'a'}, id[:8]...), encodeTestTimestamp(now-60*60*24*20))
		}
	}

	// untracked events count as used now, so only the 4 stale ones go
	cleanupCachedEvents(context.Background(), 13, 0)

	if got := countStored(); got != 6 {
		t.Fatalf("got %d events after cleanup, want 6", got)
	}
}

func encodeTestTimestamp(ts nostr.Timestamp) []byte {
	b := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		b[i] = byte(ts)
		ts >>= 8
	}
	return b
}
