package stream

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiddleware_ForwardsBodyLargerThanCaptureLimit(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe(nil)
	defer hub.Unsubscribe(sub)

	body := bytes.Repeat([]byte("a"), maxBodyCapture*2+123)

	var received []byte
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		received, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading body: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/openai/chat/completions", bytes.NewReader(body))
	Middleware(hub, "openai")(next).ServeHTTP(httptest.NewRecorder(), req)

	if !bytes.Equal(received, body) {
		t.Fatalf("forwarded body length = %d, want %d", len(received), len(body))
	}

	select {
	case ev := <-sub.Events():
		if len(ev.RequestBody) != maxBodyCapture {
			t.Errorf("captured body length = %d, want %d", len(ev.RequestBody), maxBodyCapture)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}
