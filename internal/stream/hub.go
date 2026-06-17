package stream

import (
	"strings"
	"sync"

	acgv1 "github.com/p-/ai-credential-gateway/gen/acg/v1"
)

// Hub manages fan-out of RequestEvents to subscribers.
type Hub struct {
	mu   sync.RWMutex
	subs map[*subscriber]struct{}
}

type subscriber struct {
	ch     chan *acgv1.RequestEvent
	filter *acgv1.StreamFilter
}

func NewHub() *Hub {
	return &Hub{subs: make(map[*subscriber]struct{})}
}

// Subscribe registers a new subscriber with the given filter and returns a
// channel that will receive matching events. Call Unsubscribe to clean up.
func (h *Hub) Subscribe(filter *acgv1.StreamFilter) *subscriber {
	s := &subscriber{
		ch:     make(chan *acgv1.RequestEvent, 64),
		filter: filter,
	}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

// Unsubscribe removes a subscriber and closes its channel.
func (h *Hub) Unsubscribe(s *subscriber) {
	h.mu.Lock()
	if _, ok := h.subs[s]; ok {
		delete(h.subs, s)
		close(s.ch)
	}
	h.mu.Unlock()
}

// Publish sends an event to all subscribers whose filter matches.
// Non-blocking: slow subscribers will miss events.
func (h *Hub) Publish(ev *acgv1.RequestEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		if !matches(s.filter, ev) {
			continue
		}
		select {
		case s.ch <- ev:
		default:
			// drop if subscriber is slow
		}
	}
}

func matches(f *acgv1.StreamFilter, ev *acgv1.RequestEvent) bool {
	if f == nil {
		return true
	}
	if f.ProxyKey != "" && f.ProxyKey != ev.ProxyKey {
		return false
	}
	if f.PathPrefix != "" && !strings.HasPrefix(ev.Path, f.PathPrefix) {
		return false
	}
	return true
}

// Events returns the channel for receiving events.
func (s *subscriber) Events() <-chan *acgv1.RequestEvent {
	return s.ch
}
