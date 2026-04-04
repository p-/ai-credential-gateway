package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// NewGatewayAuth returns middleware that validates the client's credential
// against gatewayCredential. The credential is extracted from the request
// using the same header format defined by credentialHeader (e.g.
// "Authorization: Bearer {credential}" or "x-api-key: {credential}").
//
// The comparison is performed in constant time.
func NewGatewayAuth(credentialHeader, gatewayCredential string) func(http.Handler) http.Handler {
	headerName, valueTemplate := parseTemplate(credentialHeader)
	prefix, suffix := splitTemplate(valueTemplate)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headerVal := r.Header.Get(headerName)
			if headerVal == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			token := extractToken(headerVal, prefix, suffix)
			if token == "" || !constantTimeEqual(token, gatewayCredential) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Remove the gateway credential header so it is not forwarded upstream.
			r.Header.Del(headerName)

			next.ServeHTTP(w, r)
		})
	}
}

// parseTemplate splits "HeaderName: value template" into the header name and
// the value portion.
func parseTemplate(tmpl string) (string, string) {
	parts := strings.SplitN(tmpl, ":", 2)
	name := strings.TrimSpace(parts[0])
	value := ""
	if len(parts) == 2 {
		value = strings.TrimSpace(parts[1])
	}
	return name, value
}

// splitTemplate splits a value template like "Bearer {credential}" into the
// prefix ("Bearer ") and suffix ("") around the {credential} placeholder.
func splitTemplate(valueTmpl string) (prefix, suffix string) {
	idx := strings.Index(valueTmpl, "{credential}")
	if idx < 0 {
		return valueTmpl, ""
	}
	return valueTmpl[:idx], valueTmpl[idx+len("{credential}"):]
}

// extractToken removes the prefix and suffix from a header value to recover
// the raw credential token.
func extractToken(headerVal, prefix, suffix string) string {
	if !strings.HasPrefix(headerVal, prefix) {
		return ""
	}
	headerVal = strings.TrimPrefix(headerVal, prefix)

	if suffix != "" {
		if !strings.HasSuffix(headerVal, suffix) {
			return ""
		}
		headerVal = strings.TrimSuffix(headerVal, suffix)
	}
	return headerVal
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
