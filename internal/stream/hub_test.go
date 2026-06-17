package stream

import (
	"testing"
	"time"

	acgv1 "github.com/p-/ai-credential-gateway/gen/acg/v1"
)

func TestHub_PublishSubscribe(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(nil)
	defer hub.Unsubscribe(sub)

	ev := &acgv1.RequestEvent{Method: "GET", Path: "/test", ProxyKey: "openai"}
	hub.Publish(ev)

	select {
	case got := <-sub.Events():
		if got.Method != "GET" || got.Path != "/test" {
			t.Fatalf("unexpected event: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestHub_FilterByProxyKey(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(&acgv1.StreamFilter{ProxyKey: "anthropic"})
	defer hub.Unsubscribe(sub)

	hub.Publish(&acgv1.RequestEvent{ProxyKey: "openai", Path: "/a"})
	hub.Publish(&acgv1.RequestEvent{ProxyKey: "anthropic", Path: "/b"})

	select {
	case got := <-sub.Events():
		if got.ProxyKey != "anthropic" {
			t.Fatalf("expected anthropic, got %s", got.ProxyKey)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestHub_FilterByPathPrefix(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(&acgv1.StreamFilter{PathPrefix: "/api"})
	defer hub.Unsubscribe(sub)

	hub.Publish(&acgv1.RequestEvent{ProxyKey: "x", Path: "/other"})
	hub.Publish(&acgv1.RequestEvent{ProxyKey: "x", Path: "/api/v1/chat"})

	select {
	case got := <-sub.Events():
		if got.Path != "/api/v1/chat" {
			t.Fatalf("expected /api/v1/chat, got %s", got.Path)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestHub_Unsubscribe(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(nil)
	hub.Unsubscribe(sub)

	// Channel should be closed.
	_, ok := <-sub.Events()
	if ok {
		t.Fatal("expected channel to be closed")
	}

	// Publishing after unsubscribe should not panic.
	hub.Publish(&acgv1.RequestEvent{})
}

func TestHub_SlowSubscriberDropsEvents(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(nil)
	defer hub.Unsubscribe(sub)

	// Fill the buffer (capacity 64).
	for i := 0; i < 100; i++ {
		hub.Publish(&acgv1.RequestEvent{Method: "GET"})
	}

	count := 0
	for {
		select {
		case <-sub.Events():
			count++
		default:
			goto done
		}
	}
done:
	if count != 64 {
		t.Fatalf("expected 64 events (buffer size), got %d", count)
	}
}
