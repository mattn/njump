package main

import (
	"net/http"
	"strings"
)

func agentBlock(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// let them read robots.txt, which tells them to go away for good
		if r.URL.Path == "/robots.txt" {
			next.ServeHTTP(w, r)
			return
		}
		ua := r.Header.Get("User-Agent")
		// only crawlers that robots.txt turns away anyway: they don't all
		// read it often, and some (meta-webindexer) crawl hard until they do.
		// words like "Opera" or "Amethyst" would also hit people and clients.
		for _, bua := range []string{
			"Amazonbot",
			"SemrushBot",
			"meta-externalagent",
			"meta-webindexer",
			"DataForSeoBot",
			"DotBot",
			"MJ12bot",
			"PetalBot",
		} {
			if strings.Contains(ua, bua) {
				// log.Debug().Str("ua", ua).Msg("user-agent blocked")
				http.Error(w, "", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
