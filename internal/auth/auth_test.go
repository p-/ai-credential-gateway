package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
})

func TestNewGatewayAuth_ValidBearerToken(t *testing.T) {
	mw := NewGatewayAuth("Authorization: Bearer {credential}", "my-secret")
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer my-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestNewGatewayAuth_MissingHeader(t *testing.T) {
	mw := NewGatewayAuth("Authorization: Bearer {credential}", "my-secret")
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNewGatewayAuth_WrongCredential(t *testing.T) {
	mw := NewGatewayAuth("Authorization: Bearer {credential}", "my-secret")
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNewGatewayAuth_WrongPrefix(t *testing.T) {
	mw := NewGatewayAuth("Authorization: Bearer {credential}", "my-secret")
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic my-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNewGatewayAuth_XApiKey(t *testing.T) {
	mw := NewGatewayAuth("x-api-key: {credential}", "key123")
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("x-api-key", "key123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestNewGatewayAuth_DeletesHeaderBeforeForwarding(t *testing.T) {
	var gotHeader string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})

	mw := NewGatewayAuth("Authorization: Bearer {credential}", "tok")
	handler := mw(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotHeader != "" {
		t.Errorf("inner handler received Authorization = %q, want empty (should be stripped)", gotHeader)
	}
}

func TestNewGatewayAuth_EmptyCredentialValue(t *testing.T) {
	mw := NewGatewayAuth("Authorization: Bearer {credential}", "my-secret")
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestParseTemplate(t *testing.T) {
	tests := []struct {
		tmpl      string
		wantName  string
		wantValue string
	}{
		{"Authorization: Bearer {credential}", "Authorization", "Bearer {credential}"},
		{"x-api-key: {credential}", "x-api-key", "{credential}"},
		{"HeaderOnly", "HeaderOnly", ""},
	}
	for _, tt := range tests {
		t.Run(tt.tmpl, func(t *testing.T) {
			name, value := parseTemplate(tt.tmpl)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if value != tt.wantValue {
				t.Errorf("value = %q, want %q", value, tt.wantValue)
			}
		})
	}
}

func TestSplitTemplate(t *testing.T) {
	tests := []struct {
		tmpl       string
		wantPrefix string
		wantSuffix string
	}{
		{"Bearer {credential}", "Bearer ", ""},
		{"{credential}", "", ""},
		{"pre-{credential}-suf", "pre-", "-suf"},
		{"no placeholder", "no placeholder", ""},
	}
	for _, tt := range tests {
		t.Run(tt.tmpl, func(t *testing.T) {
			prefix, suffix := splitTemplate(tt.tmpl)
			if prefix != tt.wantPrefix {
				t.Errorf("prefix = %q, want %q", prefix, tt.wantPrefix)
			}
			if suffix != tt.wantSuffix {
				t.Errorf("suffix = %q, want %q", suffix, tt.wantSuffix)
			}
		})
	}
}

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name      string
		headerVal string
		prefix    string
		suffix    string
		want      string
	}{
		{"bearer", "Bearer sk-abc", "Bearer ", "", "sk-abc"},
		{"raw", "key123", "", "", "key123"},
		{"with suffix", "pre-tok-suf", "pre-", "-suf", "tok"},
		{"wrong prefix", "Basic tok", "Bearer ", "", ""},
		{"wrong suffix", "pre-tok-bad", "pre-", "-suf", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractToken(tt.headerVal, tt.prefix, tt.suffix)
			if got != tt.want {
				t.Errorf("extractToken(%q, %q, %q) = %q, want %q", tt.headerVal, tt.prefix, tt.suffix, got, tt.want)
			}
		})
	}
}
