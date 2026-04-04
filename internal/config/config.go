package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ProxyEntry struct {
	Key           string `yaml:"key"`
	Path          string `yaml:"path"`
	HeaderReplace string `yaml:"credential_header"`
	Endpoint      string `yaml:"endpoint"`
}

type Config struct {
	ListenAddr string       `yaml:"listen_addr"`
	Proxies    []ProxyEntry `yaml:"proxies"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":4180"
	}

	for i, p := range cfg.Proxies {
		if p.Key == "" {
			return nil, fmt.Errorf("proxy entry %d: key is required", i)
		}
		if p.Path == "" {
			return nil, fmt.Errorf("proxy entry %d (%s): path is required", i, p.Key)
		}
		if p.Endpoint == "" {
			return nil, fmt.Errorf("proxy entry %d (%s): endpoint is required", i, p.Key)
		}
		if p.HeaderReplace == "" {
			return nil, fmt.Errorf("proxy entry %d (%s): credential_header is required", i, p.Key)
		}
	}

	return &cfg, nil
}

// ResolveCredential reads the credential for a proxy entry from the environment.
// The environment variable is expected in the form <KEY>_CREDENTIAL (uppercased).
func ResolveCredential(key string) (string, error) {
	envKey := strings.ToUpper(key) + "_CREDENTIAL"
	val := os.Getenv(envKey)
	if val == "" {
		return "", fmt.Errorf("environment variable %s is not set", envKey)
	}
	return val, nil
}
