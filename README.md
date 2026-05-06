# AI Credential Gateway (acg)

**EXPERIMENTAL**

A configurable reverse proxy that provides access to AI APIs without exposing credentials to clients.

**NOTE**: This gateway is intended to be used locally or in controlled environments and should NEVER be exposed to uncontrolled or even public networks.

## How it works

The gateway sits between your application and AI provider APIs. Clients send requests to the gateway using plain paths (e.g. `/openai/chat/completions`), and the gateway injects the real API credentials before forwarding to the upstream endpoint.

Credentials are currently read from environment variables in the form `<KEY>_CREDENTIAL` (uppercased), where `<KEY>` matches the `key` field in the config.

## Gateway Authentication

By default, `require_auth` is `true` and the gateway will refuse to start unless the `GATEWAY_SECRET` environment variable is set. When authentication is enabled, the gateway extracts the credential from each incoming request using the same header format defined by `credential_header` for that proxy entry. For example, OpenAI-bound requests must send `Authorization: Bearer <gateway-token>` and Anthropic-bound requests must send `x-api-key: <gateway-token>`. The client's auth header is stripped before forwarding; the real upstream credential is injected by the gateway.

If `require_auth` is set to `false`, authentication is optional — but if `GATEWAY_SECRET` is still provided, client authentication will be enforced.

## Configuration

Create a `config.yaml` (or pass a custom path via `-config`):

```yaml
listen_addr: "127.0.0.1:4180"
require_auth: false

proxies:
  - key: openai
    path: openai
    credential_header: "Authorization: Bearer {credential}"
    endpoint: "https://api.openai.com/v1"

  - key: anthropic
    path: anthropic
    credential_header: "x-api-key: {credential}"
    endpoint: "https://api.anthropic.com/v1"
```

| Field            | Description                                                                 |
|------------------|-----------------------------------------------------------------------------|
| `key`            | Unique identifier; also determines the env var name (`<KEY>_CREDENTIAL`)    |
| `path`           | URL path prefix the gateway listens on                                      |
| `credential_header` | Header template — `{credential}` is replaced with the actual secret         |
| `endpoint`       | Upstream API base URL                                                       |

## Build & Run

```bash
go build -o acg ./cmd/gateway/

# Set credentials via environment variables
export OPENAI_CREDENTIAL="sk-..."
export ANTHROPIC_CREDENTIAL="sk-ant-..."

# Optionally require clients to authenticate to the gateway (e.g. generating a secret with `openssl rand -hex 40`)
export GATEWAY_SECRET=<secret>

./acg -config config.yaml
```

Clients can then replace configured endpoints like `https://api.openai.com/v1` and `https://api.anthropic.com/v1` to `http://127.0.0.1:4180/openai` and `http://127.0.0.1:4180/anthropic`respectively. The real tokens then can be removed from this clients. (if a GATEWAY_SECRET is set, that one has to be provided instead.) 


## Sample curl requests:


```bash
curl http://127.0.0.1:4180/openai/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'
```

```bash
curl http://127.0.0.1:4180/anthropic/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-sonnet-4-6","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}'
```

When `GATEWAY_SECRET` is set, clients must include the gateway token using the same header format as the target API:

```bash
# OpenAI — uses Authorization header
curl http://127.0.0.1:4180/openai/chat/completions \
  -H "Authorization: Bearer my-secret-gateway-token" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'
```

```bash
# Anthropic — uses x-api-key header
curl http://127.0.0.1:4180/anthropic/messages \
  -H "x-api-key: my-secret-gateway-token" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-sonnet-4-6","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}'
```

## Build and run with Docker

```
docker build -t ai-credential-gateway .
```

Use the Docker image in a way the AI agent that should be isolated has no access to the ENV variables passed to the ai-credential-gateway.