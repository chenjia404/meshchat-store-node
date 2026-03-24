package netutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectPublicIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("203.0.113.10\n"))
	}))
	defer server.Close()

	ip, err := DetectPublicIP(context.Background(), server.Client(), []string{server.URL})
	if err != nil {
		t.Fatalf("DetectPublicIP() error = %v", err)
	}
	if ip != "203.0.113.10" {
		t.Fatalf("DetectPublicIP() = %q, want %q", ip, "203.0.113.10")
	}
}

func TestDetectPublicIPFallback(t *testing.T) {
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer badServer.Close()

	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("198.51.100.20"))
	}))
	defer goodServer.Close()

	ip, err := DetectPublicIP(context.Background(), goodServer.Client(), []string{badServer.URL, goodServer.URL})
	if err != nil {
		t.Fatalf("DetectPublicIP() error = %v", err)
	}
	if ip != "198.51.100.20" {
		t.Fatalf("DetectPublicIP() = %q, want %q", ip, "198.51.100.20")
	}
}

func TestDetectPublicIPInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-an-ip"))
	}))
	defer server.Close()

	if _, err := DetectPublicIP(context.Background(), server.Client(), []string{server.URL}); err == nil {
		t.Fatal("DetectPublicIP() error = nil, want error")
	}
}
