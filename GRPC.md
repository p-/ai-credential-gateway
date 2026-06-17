# gRPC Live Request Stream

The gateway exposes a gRPC server that streams live request/response information to connected clients in real time.

## Configuration

Add `grpc_addr` to your `config.yaml` to enable the gRPC stream:

```yaml
listen_addr: "127.0.0.1:4180"
grpc_addr: "127.0.0.1:4181"
```

The gRPC server only starts if `grpc_addr` is explicitly set. If omitted, the gateway runs without the streaming interface.

## Service Definition

```protobuf
service RequestStreamService {
  rpc StreamRequests(StreamFilter) returns (stream RequestEvent);
}

message StreamFilter {
  string proxy_key = 1;   // optional: filter by proxy key (e.g. "openai")
  string path_prefix = 2; // optional: filter by request path prefix
}

message RequestEvent {
  google.protobuf.Timestamp timestamp = 1;
  string method = 2;
  string path = 3;
  int32 status_code = 4;
  bytes request_body = 5;
  bytes response_body = 6;
  string client_ip = 7;
  string proxy_key = 8;
}
```

Request and response bodies are capped at 64 KB per event.

## TUI Client

A built-in TUI client renders live request events with colored output.

### Usage

```bash
# Stream all requests
go run ./cmd/streamclient --addr 127.0.0.1:4181

# Filter by proxy key
go run ./cmd/streamclient --addr 127.0.0.1:4181 --proxy-key openai

# Filter by path prefix
go run ./cmd/streamclient --addr 127.0.0.1:4181 --path-prefix /v1/chat
```

### Flags

| Flag            | Default           | Description                     |
|-----------------|-------------------|---------------------------------|
| `--addr`        | `127.0.0.1:4181`  | gRPC server address             |
| `--proxy-key`   | (none)            | Only show events for this proxy |
| `--path-prefix` | (none)            | Only show events matching prefix|

### Controls

- `↑` / `↓` — Scroll through events
- `q` / `Ctrl+C` — Quit

## Development

### Prerequisites

```bash
# Install protobuf compiler
sudo apt-get install -y protobuf-compiler

# Install Go protoc plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Regenerate Proto

```bash
mkdir -p gen/acg/v1
protoc \
  --proto_path=proto \
  --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
  acg/v1/stream.proto
```

### Build

```bash
go build ./...
```

### Run Tests

```bash
go test ./...
```

### Quick Test (end-to-end)

Terminal 1 — start the gateway:

```bash
export OPENAI_CREDENTIAL=sk-your-key
go run ./cmd/gateway --config config.yaml
```

Terminal 2 — start the stream client:

```bash
go run ./cmd/streamclient --addr 127.0.0.1:4181
```

Terminal 3 — send a request through the gateway:

```bash
curl http://127.0.0.1:4180/openai/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'
```

The TUI client will display the request in real time with colored status codes (green 2xx, yellow 3xx, red 4xx/5xx), method, path, proxy key, client IP, and truncated request/response bodies.
