package backend

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"golang.org/x/net/proxy"
)

// NewHTTPClient returns an *http.Client configured with the given timeout
// and optionally routed through a proxy.
//
// proxyURL examples:
//   - "" (empty) — no proxy
//   - "http://host:8080"
//   - "socks5://host:1080"
//
// The PROXY_URL environment variable overrides the proxyURL argument.
func NewHTTPClient(timeout time.Duration, proxyURL string) (*http.Client, error) {
	if env := os.Getenv("PROXY_URL"); env != "" {
		proxyURL = env
	}

	transport := &http.Transport{}

	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyURL, err)
		}

		switch parsed.Scheme {
		case "http", "https":
			transport.Proxy = http.ProxyURL(parsed)
		case "socks5":
			dialer, err := proxy.FromURL(parsed, proxy.Direct)
			if err != nil {
				return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		default:
			return nil, fmt.Errorf("unsupported proxy scheme %q (use http, https, or socks5)", parsed.Scheme)
		}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}

// MustHTTPClient is like NewHTTPClient but panics on configuration errors.
// Only use during startup when a misconfigured proxy should be fatal.
func MustHTTPClient(timeout time.Duration, proxyURL string) *http.Client {
	c, err := NewHTTPClient(timeout, proxyURL)
	if err != nil {
		panic(fmt.Sprintf("httpclient: %v", err))
	}
	return c
}
