package app

import (
	"fmt"
	"net/http"
	"time"
)

const (
	// A comment line this often keeps Cloudflare and nginx from closing a
	// quiet stream; both cut a response that sends nothing for about 100 s.
	streamHeartbeat = 20 * time.Second
	// Streams end on their own after a while and the browser reconnects, so
	// nginx reloads and releases never wait on old connections.
	streamLifetime = 4 * time.Minute
)

// events is the live-update stream the explorer and the sales desk listen to.
func (a *App) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	// Changes reach only browsers sharing the visitor's own copy. Subscribe
	// before the first byte goes out, so a change made the moment the browser
	// sees the stream open is not missed.
	var updates chan []byte
	if v := visitorFrom(r.Context()); v != nil && v.copyID != "" {
		ch, unsubscribe := a.broker.subscribe(v.copyID)
		defer unsubscribe()
		updates = ch
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 3000\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(streamHeartbeat)
	defer heartbeat.Stop()
	lifetime := time.NewTimer(streamLifetime)
	defer lifetime.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-a.streams.Done():
			return
		case <-lifetime.C:
			return
		case body := <-updates:
			fmt.Fprintf(w, "event: change\ndata: %s\n\n", body)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": still here\n\n")
			flusher.Flush()
		}
	}
}
