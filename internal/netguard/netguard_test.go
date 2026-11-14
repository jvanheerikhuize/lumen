package netguard

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestClassifyHost(t *testing.T) {
	publicHosts := []string{
		"torrentio.strem.fun",
		"8.8.8.8",
		"2606:4700::1111",
		"172.32.0.1",
	}
	for _, h := range publicHosts {
		if got := ClassifyHost(h); got != HostPublic {
			t.Errorf("ClassifyHost(%q) = %q, want %q", h, got, HostPublic)
		}
	}

	privateHosts := []string{
		"localhost",
		"127.0.0.1",
		"10.1.2.3",
		"172.16.0.1",
		"192.168.178.12",
		"100.64.0.1",
		"::1",
		"fd00::1",
		"nas",
		"nas.local",
		"router.lan",
		"[::1]",
		"::ffff:192.168.1.1",
	}
	for _, h := range privateHosts {
		if got := ClassifyHost(h); got != HostPrivate {
			t.Errorf("ClassifyHost(%q) = %q, want %q", h, got, HostPrivate)
		}
	}

	blockedHosts := []string{
		"169.254.169.254",
		"0.0.0.0",
		"::",
		"fe80::1",
		"224.0.0.1",
		"metadata.google.internal",
		"::ffff:169.254.169.254",
		"::ffff:a9fe:a9fe",
		"",
	}
	for _, h := range blockedHosts {
		if got := ClassifyHost(h); got != HostBlocked {
			t.Errorf("ClassifyHost(%q) = %q, want %q", h, got, HostBlocked)
		}
	}
}

func TestCheckServiceURL(t *testing.T) {
	tests := []struct {
		raw          string
		allowPrivate bool
		want         string
	}{
		{"https://torrentio.strem.fun/", false, "https://torrentio.strem.fun"},
		{"http://192.168.1.10:9696", true, "http://192.168.1.10:9696"},
		{"http://192.168.1.10:9696", false, ""},
		{"http://169.254.169.254/latest", true, ""},
		{"file:///etc/passwd", true, ""},
		{"https://user:pass@example.com", false, ""},
		{"https://example.com/?x=1", false, ""},
		{"not a url", true, ""},
	}
	for _, tc := range tests {
		got := CheckServiceURL(tc.raw, tc.allowPrivate)
		if got != tc.want {
			t.Errorf("CheckServiceURL(%q, %v) = %q, want %q", tc.raw, tc.allowPrivate, got, tc.want)
		}
	}
}

func TestREQ0011_DNSResolutionAndRedirectSSRFProtection(t *testing.T) {
	ctx := context.Background()

	// Scenario: REQ-0011 Outbound URL, DNS resolution, and redirect-hop SSRF protection with bounded response reads #1
	// Given a user-supplied URL whose public hostname resolves via DNS to 127.0.0.1 or 169.254.169.254,
	// or redirects (302) to http://192.168.1.1/admin
	// When the server performs a guarded fetch with allowPrivate: false
	// Then the request is rejected before connecting to the target address.

	rebindLoopback := func(ctx context.Context, host string) ([]string, error) {
		return []string{"127.0.0.1"}, nil
	}
	if IsAllowedURLWithLookup(ctx, "https://evil.example.com/manifest.json", false, rebindLoopback) {
		t.Fatal("expected DNS resolution to 127.0.0.1 to be rejected when allowPrivate=false")
	}
	if IsAllowedURLWithLookup(ctx, "https://evil.example.com/manifest.json", true, rebindLoopback) {
		t.Fatal("expected public hostname resolving to 127.0.0.1 to be rejected even when allowPrivate=true")
	}

	rebindMetadata := func(ctx context.Context, host string) ([]string, error) {
		return []string{"169.254.169.254"}, nil
	}
	if IsAllowedURLWithLookup(ctx, "https://evil.example.com/latest/meta-data", false, rebindMetadata) {
		t.Fatal("expected DNS resolution to 169.254.169.254 to be rejected")
	}

	// Verify GuardedFetch rejects before connecting when DNS resolves to 127.0.0.1 or 169.254.169.254
	if _, err := GuardedFetch(ctx, "http://rebind.example.com/test", FetchOptions{
		AllowPrivate: false,
		Lookup:       rebindLoopback,
	}); err == nil {
		t.Fatal("expected GuardedFetch to reject DNS rebinding to 127.0.0.1")
	}

	// Verify socket-level net.Dialer.Control rejects private/loopback IP when allowPrivate=false
	dialer := NewGuardedDialer(false, HostPublic)
	if err := dialer.Control("tcp4", "127.0.0.1:80", nil); err == nil {
		t.Fatal("expected Dialer.Control to reject 127.0.0.1:80")
	}
	if err := dialer.Control("tcp4", "192.168.1.1:80", nil); err == nil {
		t.Fatal("expected Dialer.Control to reject 192.168.1.1:80 when allowPrivate=false")
	}
	if err := dialer.Control("tcp4", "169.254.169.254:80", nil); err == nil {
		t.Fatal("expected Dialer.Control to reject 169.254.169.254:80")
	}
	if err := dialer.Control("tcp4", "93.184.216.34:443", nil); err != nil {
		t.Fatalf("expected Dialer.Control to allow public IP, got %v", err)
	}
}

func TestReadBoundedText(t *testing.T) {
	smallResp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(strings.NewReader("hello world")),
	}
	txt, err := ReadBoundedText(smallResp, 100)
	if err != nil || txt != "hello world" {
		t.Fatalf("unexpected result: txt=%q err=%v", txt, err)
	}

	largeResp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(strings.NewReader(strings.Repeat("x", 1025))),
	}
	if _, err := ReadBoundedText(largeResp, 1024); err != ErrResponseTooLarge {
		t.Fatalf("expected ErrResponseTooLarge, got %v", err)
	}
}
