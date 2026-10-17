package app

import (
	"encoding/json"
	"sync"
)

// event is a change worth showing live: a home's status, a new reservation,
// a viewing booked. It only reaches browsers sharing the same copy.
type event struct {
	Type   string `json:"type"`
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
	Text   string `json:"text,omitempty"`
}

type broker struct {
	mu   sync.Mutex
	subs map[string]map[chan []byte]struct{}
}

func newBroker() *broker {
	return &broker{subs: map[string]map[chan []byte]struct{}{}}
}

func (b *broker) subscribe(copyID string) (chan []byte, func()) {
	ch := make(chan []byte, 16)
	b.mu.Lock()
	if b.subs[copyID] == nil {
		b.subs[copyID] = map[chan []byte]struct{}{}
	}
	b.subs[copyID][ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs[copyID], ch)
		if len(b.subs[copyID]) == 0 {
			delete(b.subs, copyID)
		}
		b.mu.Unlock()
	}
}

func (b *broker) publish(copyID string, events ...event) {
	if copyID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, e := range events {
		body, err := json.Marshal(e)
		if err != nil {
			continue
		}
		for ch := range b.subs[copyID] {
			select {
			case ch <- body:
			default:
				// A browser that has fallen behind misses an update; the
				// next page load is correct anyway.
			}
		}
	}
}
