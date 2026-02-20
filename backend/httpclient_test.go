package backend

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHTTPClient_Default(t *testing.T) {
	client, err := NewHTTPClient(30*time.Second, "")
	if err != nil {
		t.Fatalf("NewHTTPClient failed: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", client.Timeout)
	}
}

func TestNewHTTPClient_ZeroTimeout(t *testing.T) {
	client, err := NewHTTPClient(0, "")
	if err != nil {
		t.Fatalf("NewHTTPClient failed: %v", err)
	}
	if client.Timeout != 0 {
		t.Errorf("expected 0 timeout, got %v", client.Timeout)
	}
}

func TestNewHTTPClient_InvalidProxy(t *testing.T) {
	_, err := NewHTTPClient(10*time.Second, "://invalid")
	if err == nil {
		t.Error("expected error for invalid proxy URL")
	}
}

func TestNewHTTPClient_UnsupportedScheme(t *testing.T) {
	_, err := NewHTTPClient(10*time.Second, "ftp://localhost:21")
	if err == nil {
		t.Error("expected error for unsupported proxy scheme")
	}
}

func TestNewHTTPClient_HTTPProxy(t *testing.T) {
	// Create a test proxy server (just to verify the config path)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client, err := NewHTTPClient(10*time.Second, srv.URL)
	if err != nil {
		t.Fatalf("NewHTTPClient with http proxy failed: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestMustHTTPClient_Valid(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustHTTPClient panicked unexpectedly: %v", r)
		}
	}()
	client := MustHTTPClient(10*time.Second, "")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestMustHTTPClient_InvalidPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustHTTPClient to panic on invalid proxy")
		}
	}()
	MustHTTPClient(10*time.Second, "ftp://invalid")
}
