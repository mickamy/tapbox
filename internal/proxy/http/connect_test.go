package http_test

import (
	"testing"

	httpproxy "github.com/mickamy/tapbox/internal/proxy/http"
)

func TestIsConnectRequest(t *testing.T) {
	t.Parallel()

	connectPath := "/acme.greeter.v1.GreeterService/Greet"
	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		want        bool
	}{
		{"valid unary json", "POST", connectPath, "application/json", true},
		{"valid unary proto", "POST", connectPath, "application/proto", true},
		{"valid connect+json", "POST", connectPath, "application/connect+json", true},
		{"valid connect+proto", "POST", connectPath, "application/connect+proto", true},
		{"ct with charset", "POST", connectPath, "application/json; charset=utf-8", true},
		{"not POST", "GET", connectPath, "application/json", false},
		{"rest path", "POST", "/api/users", "application/json", false},
		{"no dot in service", "POST", "/Service/Method", "application/json", false},
		{"html content type", "POST", connectPath, "text/html", false},
		{"empty path", "POST", "", "application/json", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := httpproxy.IsConnectRequest(tt.method, tt.path, tt.contentType)
			if got != tt.want {
				t.Errorf("IsConnectRequest(%q, %q, %q) = %v, want %v",
					tt.method, tt.path, tt.contentType, got, tt.want)
			}
		})
	}
}

func TestIsConnectPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{"/acme.greeter.v1.GreeterService/Greet", true},
		{"/buf.connect.v1.ElizaService/Say", true},
		{"/api/users", false},
		{"/Service/Method", false},
		{"/a.b/", false},
		{"//Method", false},
		{"/a.b/c/d", false},
		{"", false},
		{"/", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			if got := httpproxy.IsConnectPath(tt.path); got != tt.want {
				t.Errorf("IsConnectPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestParseConnectPath(t *testing.T) {
	t.Parallel()

	svc, method := httpproxy.ParseConnectPath(
		"/acme.greeter.v1.GreeterService/Greet",
	)
	if svc != "acme.greeter.v1.GreeterService" {
		t.Errorf("service = %q, want %q", svc, "acme.greeter.v1.GreeterService")
	}
	if method != "Greet" {
		t.Errorf("method = %q, want %q", method, "Greet")
	}
}

func TestIsConnectStreaming(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ct   string
		want bool
	}{
		{"application/connect+json", true},
		{"application/connect+proto", true},
		{"application/connect+json; charset=utf-8", true},
		{"application/json", false},
		{"application/proto", false},
		{"text/html", false},
	}
	for _, tt := range tests {
		t.Run(tt.ct, func(t *testing.T) {
			t.Parallel()
			if got := httpproxy.IsConnectStreaming(tt.ct); got != tt.want {
				t.Errorf("IsConnectStreaming(%q) = %v, want %v", tt.ct, got, tt.want)
			}
		})
	}
}

func TestParseConnectError(t *testing.T) {
	t.Parallel()

	t.Run("valid error", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"code":"not_found","message":"user not found"}`)
		code, msg := httpproxy.ParseConnectError(body)
		if code != "not_found" {
			t.Errorf("code = %q, want %q", code, "not_found")
		}
		if msg != "user not found" {
			t.Errorf("message = %q, want %q", msg, "user not found")
		}
	})

	t.Run("not json", func(t *testing.T) {
		t.Parallel()
		code, msg := httpproxy.ParseConnectError([]byte("not json"))
		if code != "" || msg != "" {
			t.Errorf("expected empty code/message, got %q/%q", code, msg)
		}
	})

	t.Run("no error fields", func(t *testing.T) {
		t.Parallel()
		code, msg := httpproxy.ParseConnectError([]byte(`{"result":"ok"}`))
		if code != "" || msg != "" {
			t.Errorf("expected empty code/message, got %q/%q", code, msg)
		}
	})
}
