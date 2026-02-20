package backend

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeService_Up(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	status := probeService(srv.URL, "")
	if status.Status != "up" {
		t.Errorf("expected 'up', got %q", status.Status)
	}
	if status.CheckedAt.IsZero() {
		t.Error("CheckedAt should be set")
	}
}

func TestProbeService_Down(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	status := probeService(srv.URL, "")
	if status.Status != "down" {
		t.Errorf("expected 'down', got %q", status.Status)
	}
}

func TestProbeService_Unreachable(t *testing.T) {
	// Start a server then immediately close it to get a "connection refused" fast
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close() // Close immediately so the port is unreachable

	status := probeService(addr, "")
	if status.Status != "down" {
		t.Errorf("expected 'down' for unreachable host, got %q", status.Status)
	}
}

func TestCheckServiceStatus_CacheHit(t *testing.T) {
	// Seed cache with a known entry
	testName := "test_service_" + t.Name()
	serviceEndpoints[testName] = "http://should-not-be-called.invalid"

	globalServiceCache.mu.Lock()
	globalServiceCache.entries[testName] = ServiceStatus{
		Status:    "up",
		CheckedAt: time.Now(),
	}
	globalServiceCache.mu.Unlock()

	result := CheckServiceStatus("")

	delete(serviceEndpoints, testName)
	globalServiceCache.mu.Lock()
	delete(globalServiceCache.entries, testName)
	globalServiceCache.mu.Unlock()

	status, ok := result[testName]
	if !ok {
		t.Fatal("expected test service in result")
	}
	if status.Status != "up" {
		t.Errorf("expected cached 'up', got %q", status.Status)
	}
}
