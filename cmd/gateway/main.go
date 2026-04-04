package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/p-/ai-credential-gateway/internal/auth"
	"github.com/p-/ai-credential-gateway/internal/config"
	"github.com/p-/ai-credential-gateway/internal/proxy"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	mux := http.NewServeMux()

	gatewayCredential := os.Getenv("GATEWAY_SECRET")
	if gatewayCredential != "" {
		log.Println("GATEWAY_SECRET is set — client authentication enabled")
	}

	for _, entry := range cfg.Proxies {
		credential, err := config.ResolveCredential(entry.Key)
		if err != nil {
			log.Fatalf("proxy %q: %v", entry.Key, err)
		}

		handler, err := proxy.New(entry, credential)
		if err != nil {
			log.Fatalf("proxy %q: failed to create handler: %v", entry.Key, err)
		}

		var h http.Handler = handler
		if gatewayCredential != "" {
			h = auth.NewGatewayAuth(entry.HeaderReplace, gatewayCredential)(handler)
		}

		pattern := "/" + entry.Path + "/"
		mux.Handle(pattern, h)
		log.Printf("registered proxy: /%s/ -> %s", entry.Path, entry.Endpoint)
	}

	log.Printf("listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
