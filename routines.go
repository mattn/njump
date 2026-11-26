package main

import (
	"context"
	"time"

	"fiatjaf.com/nostr"
)

var npubsArchive map[nostr.PubKey]struct{}

func updateArchives(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour * 3):
			log.Debug().Msg("refreshing the npubs archive")

			pubkeySet := make(map[nostr.PubKey]struct{})
			for _, pubkey := range s.trustedPubKeys {
				ctx, cancel := context.WithTimeout(ctx, time.Second*4)
				follows := sys.FetchFollowList(ctx, pubkey)
				for _, follow := range follows.Items {
					pubkeySet[follow.Pubkey] = struct{}{}
				}
				cancel()
			}
			npubsArchive = pubkeySet
		}
	}
}
