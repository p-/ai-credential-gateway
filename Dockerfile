FROM golang:1.26.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /acg ./cmd/gateway

FROM alpine:3.23.4
RUN apk add --no-cache ca-certificates \
    && adduser -D -H acguser
COPY --from=build /acg /acg
USER acguser
EXPOSE 4180
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:4141/healthz || exit 1
ENTRYPOINT ["/acg"]
