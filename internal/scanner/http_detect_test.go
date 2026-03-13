package scanner

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectHTTP(t *testing.T) {
	// Start a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Extract host and port from server URL
	// server.URL is like "http://127.0.0.1:12345"
	host := server.Listener.Addr().(*net.TCPAddr).IP.String()
	port := server.Listener.Addr().(*net.TCPAddr).Port

	// Test detection
	result := DetectHTTP(host, port)
	if !result {
		t.Errorf("DetectHTTP should return true for HTTP server, got false")
	}
}

func TestDetectHTTP_NotHTTP(t *testing.T) {
	// Test against a non-existent port (should return false)
	result := DetectHTTP("127.0.0.1", 9999)
	if result {
		t.Errorf("DetectHTTP should return false for non-existent port, got true")
	}
}

func TestDetectHTTP_404Response(t *testing.T) {
	// Test that HTTP detection works even with 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	host := server.Listener.Addr().(*net.TCPAddr).IP.String()
	port := server.Listener.Addr().(*net.TCPAddr).Port

	result := DetectHTTP(host, port)
	if !result {
		t.Errorf("DetectHTTP should return true for HTTP server returning 404, got false")
	}
}
