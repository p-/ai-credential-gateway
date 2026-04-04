package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/p-/ai-credential-gateway/internal/config"
)

// New creates an http.Handler that reverse-proxies requests for the given
// config entry, injecting the credential into the configured header.
func New(entry config.ProxyEntry, credential string) (http.Handler, error) {
	target, err := url.Parse(entry.Endpoint)
	if err != nil {
		return nil, err
	}

	headerName, headerValue := parseHeaderReplace(entry.HeaderReplace, credential)

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			// Strip the proxy path prefix and join with the target path.
			prefix := "/" + entry.Path
			reqPath := strings.TrimPrefix(req.URL.Path, prefix)
			req.URL.Path = singleJoiningSlash(target.Path, reqPath)
			req.URL.RawPath = ""

			// Set the credential header.
			req.Header.Set(headerName, headerValue)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("[%s] proxy error: %v", entry.Key, err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		},
	}

	return rp, nil
}

// parseHeaderReplace splits "HeaderName: value {credential}" into the header
// name and the resolved value with the credential substituted in.
func parseHeaderReplace(tmpl, credential string) (string, string) {
	parts := strings.SplitN(tmpl, ":", 2)
	name := strings.TrimSpace(parts[0])
	value := ""
	if len(parts) == 2 {
		value = strings.TrimSpace(parts[1])
	}
	value = strings.ReplaceAll(value, "{credential}", credential)
	return name, value
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
