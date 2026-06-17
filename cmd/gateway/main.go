package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"

	"google.golang.org/grpc"

	"github.com/p-/ai-credential-gateway/internal/auth"
	"github.com/p-/ai-credential-gateway/internal/config"
	"github.com/p-/ai-credential-gateway/internal/proxy"
	"github.com/p-/ai-credential-gateway/internal/stream"

	acgv1 "github.com/p-/ai-credential-gateway/gen/acg/v1"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	mux := http.NewServeMux()

	hub := stream.NewHub()

	// Start gRPC server for live request streaming if configured.
	if cfg.GRPCAddr != "" {
		grpcServer := grpc.NewServer()
		acgv1.RegisterRequestStreamServiceServer(grpcServer, stream.NewServer(hub))
		grpcLis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			log.Fatalf("failed to listen on gRPC addr %s: %v", cfg.GRPCAddr, err)
		}
		go func() {
			log.Printf("gRPC stream listening on %s", cfg.GRPCAddr)
			if err := grpcServer.Serve(grpcLis); err != nil {
				log.Fatalf("gRPC server error: %v", err)
			}
		}()
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	gatewayCredential := os.Getenv("GATEWAY_SECRET")
	if cfg.IsRequireAuth() && gatewayCredential == "" {
		log.Fatalf("require_auth is enabled but GATEWAY_SECRET environment variable is not set")
	}
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

		var h http.Handler = stream.Middleware(hub, entry.Key)(handler)
		if cfg.IsRequireAuth() || gatewayCredential != "" {
			h = auth.NewGatewayAuth(entry.HeaderReplace, gatewayCredential)(h)
		}

		var pattern string
		if entry.Path == "/" {
			pattern = "/"
		} else {
			pattern = "/" + entry.Path + "/"
		}
		mux.Handle(pattern, h)
		log.Printf("registered proxy: %s -> %s", pattern, entry.Endpoint)
	}

	log.Printf("listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
