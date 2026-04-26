package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/p-/ai-credential-gateway/internal/config"
)

func TestNew_ProxiesRequestAndInjectsCredential(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-secret" {
			t.Errorf("backend received Authorization = %q, want %q", got, "Bearer sk-secret")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer backend.Close()

	entry := config.ProxyEntry{
		Key:           "openai",
		Path:          "openai",
		HeaderReplace: "Authorization: Bearer {credential}",
		Endpoint:      backend.URL,
	}

	handler, err := New(entry, "sk-secret")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestNew_StripsPathPrefixAndJoinsTargetPath(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	entry := config.ProxyEntry{
		Key:           "api",
		Path:          "api",
		HeaderReplace: "x-api-key: {credential}",
		Endpoint:      backend.URL + "/v2",
	}

	handler, err := New(entry, "key123")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/chat/completions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotPath != "/v2/chat/completions" {
		t.Errorf("backend path = %q, want %q", gotPath, "/v2/chat/completions")
	}
}

func TestNew_SetsHostHeader(t *testing.T) {
	var gotHost string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	entry := config.ProxyEntry{
		Key:           "svc",
		Path:          "svc",
		HeaderReplace: "Authorization: {credential}",
		Endpoint:      backend.URL,
	}

	handler, err := New(entry, "tok")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/svc/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Host should be rewritten to the backend's host.
	if gotHost == "" {
		t.Error("backend received empty Host header")
	}
}

func TestNew_ForwardsRequestBody(t *testing.T) {
	var gotBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	entry := config.ProxyEntry{
		Key:           "llm",
		Path:          "llm",
		HeaderReplace: "Authorization: Bearer {credential}",
		Endpoint:      backend.URL,
	}

	handler, err := New(entry, "cred")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	body := `{"prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/llm/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotBody != body {
		t.Errorf("backend body = %q, want %q", gotBody, body)
	}
}

func TestNew_BackendError_ReturnsBadGateway(t *testing.T) {
	entry := config.ProxyEntry{
		Key:           "bad",
		Path:          "bad",
		HeaderReplace: "Authorization: Bearer {credential}",
		Endpoint:      "http://127.0.0.1:1", // unroutable port
	}

	handler, err := New(entry, "tok")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/bad/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestNew_InvalidEndpoint_ReturnsError(t *testing.T) {
	entry := config.ProxyEntry{
		Key:           "bad",
		Path:          "bad",
		HeaderReplace: "Authorization: Bearer {credential}",
		Endpoint:      "://invalid",
	}

	_, err := New(entry, "tok")
	if err == nil {
		t.Fatal("New() expected error for invalid endpoint, got nil")
	}
}

func TestParseHeaderReplace(t *testing.T) {
	tests := []struct {
		tmpl       string
		credential string
		wantName   string
		wantValue  string
	}{
		{
			tmpl:       "Authorization: Bearer {credential}",
			credential: "sk-abc",
			wantName:   "Authorization",
			wantValue:  "Bearer sk-abc",
		},
		{
			tmpl:       "x-api-key: {credential}",
			credential: "key123",
			wantName:   "x-api-key",
			wantValue:  "key123",
		},
		{
			tmpl:       "X-Custom: prefix-{credential}-suffix",
			credential: "tok",
			wantName:   "X-Custom",
			wantValue:  "prefix-tok-suffix",
		},
		{
			tmpl:       "HeaderOnly",
			credential: "val",
			wantName:   "HeaderOnly",
			wantValue:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.tmpl, func(t *testing.T) {
			name, value := parseHeaderReplace(tt.tmpl, tt.credential)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if value != tt.wantValue {
				t.Errorf("value = %q, want %q", value, tt.wantValue)
			}
		})
	}
}

func TestSingleJoiningSlash(t *testing.T) {
	tests := []struct {
		a, b string
		want string
	}{
		{"/a/", "/b", "/a/b"},
		{"/a", "/b", "/a/b"},
		{"/a/", "b", "/a/b"},
		{"/a", "b", "/a/b"},
		{"", "/b", "/b"},
		{"/a", "", "/a/"},
		{"", "", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.a+"+"+tt.b, func(t *testing.T) {
			got := singleJoiningSlash(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("singleJoiningSlash(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
