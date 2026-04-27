FROM golang:1.26.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /acg ./cmd/gateway

FROM alpine:3.23.4
RUN apk add --no-cache ca-certificates
COPY --from=build /acg /acg
ENTRYPOINT ["/acg"]
