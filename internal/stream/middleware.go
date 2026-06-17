package stream

import (
	"bytes"
	"io"
	"net/http"
	"time"

	acgv1 "github.com/p-/ai-credential-gateway/gen/acg/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const maxBodyCapture = 64 * 1024 // 64 KB

// Middleware returns an HTTP middleware that captures request/response data
// and publishes it to the Hub.
func Middleware(hub *Hub, proxyKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Capture request body (bounded).
			var reqBody []byte
			if r.Body != nil {
				reqBody, _ = readBounded(r.Body, maxBodyCapture)
				r.Body = io.NopCloser(bytes.NewReader(reqBody))
			}

			// Wrap response writer to capture status and body.
			cw := &captureWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(cw, r)

			ev := &acgv1.RequestEvent{
				Timestamp:    timestamppb.New(start),
				Method:       r.Method,
				Path:         r.URL.Path,
				StatusCode:   int32(cw.statusCode),
				RequestBody:  reqBody,
				ResponseBody: cw.body.Bytes(),
				ClientIp:     r.RemoteAddr,
				ProxyKey:     proxyKey,
			}
			hub.Publish(ev)
		})
	}
}

func readBounded(r io.Reader, max int) ([]byte, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, io.LimitReader(r, int64(max)))
	return buf.Bytes(), err
}

type captureWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (cw *captureWriter) WriteHeader(code int) {
	cw.statusCode = code
	cw.ResponseWriter.WriteHeader(code)
}

func (cw *captureWriter) Write(b []byte) (int, error) {
	if cw.body.Len() < maxBodyCapture {
		remaining := maxBodyCapture - cw.body.Len()
		if len(b) > remaining {
			cw.body.Write(b[:remaining])
		} else {
			cw.body.Write(b)
		}
	}
	return cw.ResponseWriter.Write(b)
}

// Flush supports streaming (SSE) through the capture layer.
func (cw *captureWriter) Flush() {
	if f, ok := cw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
