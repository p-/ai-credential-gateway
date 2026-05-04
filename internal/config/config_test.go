package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_RootPath(t *testing.T) {
	cfg, err := Load(writeConfig(t, `
listen_addr: ":8080"
proxies:
  - key: myapi
    path: /
    credential_header: "Authorization: Bearer {credential}"
    endpoint: "https://api.example.com"
`))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Proxies[0].Path != "/" {
		t.Errorf("path = %q, want %q", cfg.Proxies[0].Path, "/")
	}
}

func TestLoad_EmptyPath_Error(t *testing.T) {
	_, err := Load(writeConfig(t, `
proxies:
  - key: myapi
    path: ""
    credential_header: "Authorization: Bearer {credential}"
    endpoint: "https://api.example.com"
`))
	if err == nil {
		t.Fatal("Load() expected error for empty path, got nil")
	}
}

func TestLoad_SlashInPath_Error(t *testing.T) {
	_, err := Load(writeConfig(t, `
proxies:
  - key: myapi
    path: "foo/bar"
    credential_header: "Authorization: Bearer {credential}"
    endpoint: "https://api.example.com"
`))
	if err == nil {
		t.Fatal("Load() expected error for path with slashes, got nil")
	}
}

func TestLoad_NormalPath(t *testing.T) {
	cfg, err := Load(writeConfig(t, `
proxies:
  - key: openai
    path: openai
    credential_header: "Authorization: Bearer {credential}"
    endpoint: "https://api.openai.com/v1"
`))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Proxies[0].Path != "openai" {
		t.Errorf("path = %q, want %q", cfg.Proxies[0].Path, "openai")
	}
}
