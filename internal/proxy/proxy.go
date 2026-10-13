package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
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
	basePath := cleanBasePath(target.Path)

	rp := &httputil.ReverseProxy{
		// Flush immediately so streamed responses (e.g. SSE) are
		// forwarded to the client without buffering.
		FlushInterval: -1,
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			// Strip the proxy path prefix and join with the target path.
			// Already validated by the wrapping handler below.
			upstreamPath, _ := upstreamPath(entry.Path, basePath, req.URL.Path)
			req.URL.Path = upstreamPath
			req.URL.RawPath = ""

			// Set the credential header.
			req.Header.Set(headerName, headerValue)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("[%s] proxy error: %v", entry.Key, err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TRACE/TRACK make the upstream echo the request headers, which
		// would include the injected credential.
		if strings.EqualFold(r.Method, http.MethodTrace) || strings.EqualFold(r.Method, "TRACK") {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Reject paths that would leave the configured upstream base path
		// before the credential is attached to the outbound request.
		if _, ok := upstreamPath(entry.Path, basePath, r.URL.Path); !ok {
			log.Printf("[%s] rejected request path outside upstream base path: %q", entry.Key, r.URL.Path)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		rp.ServeHTTP(w, r)
	}), nil
}

// cleanBasePath normalizes the configured endpoint path into the base path the
// upstream request must stay within. An empty endpoint path yields "/", which
// imposes no confinement beyond rejecting dot-segments.
func cleanBasePath(endpointPath string) string {
	if endpointPath == "" {
		return "/"
	}
	return path.Clean("/" + strings.TrimPrefix(endpointPath, "/"))
}

// upstreamPath strips the proxy route prefix from the client path, joins the
// remainder onto the upstream base path and returns the result, reporting
// whether it is safe to forward.
//
// req.URL.Path is percent-decoded, so dot-segments the client encoded (e.g.
// "%2e%2e", "..%2f", "..%5c") are visible here even though they were opaque to
// the ServeMux path canonicalization that dispatched the request. Such
// segments are rejected outright rather than resolved, and the joined path is
// additionally required to stay within the base path.
func upstreamPath(entryPath, basePath, clientPath string) (string, bool) {
	reqPath := clientPath
	if entryPath != "/" {
		reqPath = strings.TrimPrefix(clientPath, "/"+entryPath)
	}

	if hasDotDotSegment(reqPath) {
		return "", false
	}

	joined := singleJoiningSlash(basePath, reqPath)
	cleaned := path.Clean(joined)
	if strings.HasSuffix(joined, "/") && !strings.HasSuffix(cleaned, "/") {
		cleaned += "/"
	}

	if !withinBase(cleaned, basePath) {
		return "", false
	}
	return cleaned, true
}

// hasDotDotSegment reports whether the path contains a ".." segment. Backslashes
// are treated as separators because some upstreams and intermediaries accept
// them as such.
func hasDotDotSegment(p string) bool {
	for _, seg := range strings.Split(strings.ReplaceAll(p, "\\", "/"), "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// withinBase reports whether p is base itself or a descendant of it.
func withinBase(p, base string) bool {
	if base == "/" {
		return strings.HasPrefix(p, "/")
	}
	return p == base || p == base+"/" || strings.HasPrefix(p, base+"/")
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
