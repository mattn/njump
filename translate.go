package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

var translateHTTPClient = &http.Client{Timeout: 30 * time.Second}

// translateConfigured reports whether a LibreTranslate instance has been set up.
// The translate button is only shown (and the endpoint only works) when it is.
func translateConfigured() bool {
	return s.TranslateAPIURL != ""
}

// translateProxy forwards a translation request to the configured LibreTranslate
// instance. Proxying server-side keeps the (optional) API key secret and avoids
// CORS, so the client only ever talks to njump's own origin.
func translateProxy(w http.ResponseWriter, r *http.Request) {
	if !translateConfigured() {
		http.Error(w, "translation not configured", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Q      string `json:"q"`
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Q == "" || req.Target == "" {
		http.Error(w, "missing q or target", http.StatusBadRequest)
		return
	}
	if req.Source == "" {
		req.Source = "auto"
	}

	payload := map[string]string{
		"q":      req.Q,
		"source": req.Source,
		"target": req.Target,
		"format": "text",
	}
	if s.TranslateAPIKey != "" {
		payload["api_key"] = s.TranslateAPIKey
	}
	body, _ := json.Marshal(payload)

	endpoint := strings.TrimRight(s.TranslateAPIURL, "/") + "/translate"
	resp, err := translateHTTPClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "translation service unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	if resp.StatusCode == http.StatusOK {
		w.Header().Set("Cache-Control", "max-age=86400")
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, io.LimitReader(resp.Body, 1<<20))
}
