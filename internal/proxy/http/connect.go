package http

import (
	"encoding/json"
	"net/http"
	"strings"
)

// isConnectRequest returns true if the request looks like a Connect RPC call.
// Criteria: POST method + Connect-compatible Content-Type + path matching /pkg.Service/Method.
func isConnectRequest(method, path, contentType string) bool {
	return method == http.MethodPost && isConnectPath(path) && isConnectContentType(contentType)
}

// isConnectContentType checks whether the Content-Type is used by the Connect protocol.
func isConnectContentType(ct string) bool {
	// Strip parameters (e.g. charset).
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch ct {
	case "application/json",
		"application/proto",
		"application/connect+json",
		"application/connect+proto":
		return true
	}
	return false
}

// isConnectPath returns true when the path has exactly two segments and
// the first segment contains a dot (package-qualified service name).
// e.g. /acme.greeter.v1.GreeterService/Greet
func isConnectPath(path string) bool {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	return strings.Contains(parts[0], ".")
}

// parseConnectPath extracts the service and method names from a Connect path.
// The path is expected to be validated by isConnectPath first.
func parseConnectPath(path string) (service, method string) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// isConnectStreaming returns true if the Content-Type indicates a Connect streaming RPC.
// Only the connect+ prefix forms support streaming.
func isConnectStreaming(ct string) bool {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return strings.HasPrefix(ct, "application/connect+")
}

// connectError represents the JSON error envelope returned by Connect.
type connectError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// parseConnectError attempts to parse a Connect error from a response body.
func parseConnectError(body []byte) (code, message string) {
	var ce connectError
	if err := json.Unmarshal(body, &ce); err != nil {
		return "", ""
	}
	return ce.Code, ce.Message
}
