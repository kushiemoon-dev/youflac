package backend

import (
	"net/http"
	"sync"
	"time"
)

// ServiceStatus represents the health of an external music service.
type ServiceStatus struct {
	Status    string    `json:"status"`    // "up", "down", "unknown"
	CheckedAt time.Time `json:"checkedAt"`
}

// serviceStatusCache caches the result of HEAD checks per service.
type serviceStatusCache struct {
	mu      sync.RWMutex
	entries map[string]ServiceStatus
	ttl     time.Duration
}

var globalServiceCache = &serviceStatusCache{
	entries: make(map[string]ServiceStatus),
	ttl:     5 * time.Minute,
}

// serviceEndpoints maps service names to probe URLs.
var serviceEndpoints = map[string]string{
	"tidal":  "https://tidal.com",
	"qobuz":  "https://www.qobuz.com",
	"amazon": "https://music.amazon.com",
	"deezer": "https://www.deezer.com",
	"lucida": "https://lucida.to",
}

// CheckServiceStatus returns the status of all configured services.
// Results are cached for 5 minutes to avoid hammering external endpoints.
func CheckServiceStatus(proxyURL string) map[string]ServiceStatus {
	result := make(map[string]ServiceStatus)

	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, endpoint := range serviceEndpoints {
		// Check cache first
		globalServiceCache.mu.RLock()
		cached, ok := globalServiceCache.entries[name]
		globalServiceCache.mu.RUnlock()

		if ok && time.Since(cached.CheckedAt) < globalServiceCache.ttl {
			result[name] = cached
			continue
		}

		wg.Add(1)
		go func(svcName, url string) {
			defer wg.Done()

			status := probeService(url, proxyURL)

			globalServiceCache.mu.Lock()
			globalServiceCache.entries[svcName] = status
			globalServiceCache.mu.Unlock()

			mu.Lock()
			result[svcName] = status
			mu.Unlock()
		}(name, endpoint)
	}

	wg.Wait()
	return result
}

func probeService(endpoint, proxyURL string) ServiceStatus {
	client, err := NewHTTPClient(10*time.Second, proxyURL)
	if err != nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	resp, err := client.Head(endpoint)
	if err != nil {
		return ServiceStatus{Status: "down", CheckedAt: time.Now()}
	}
	defer resp.Body.Close()

	status := "up"
	if resp.StatusCode >= 500 {
		status = "down"
	}
	return ServiceStatus{Status: status, CheckedAt: time.Now()}
}
