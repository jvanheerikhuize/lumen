package netguard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultMaxResponseBytes = 2 * 1024 * 1024 // 2 MB
	MaxRedirectHops         = 3
)

var (
	ErrAddressNotAllowed = errors.New("Address not allowed")
	ErrTooManyRedirects  = errors.New("Too many redirects")
	ErrResponseTooLarge  = errors.New("Response too large")
)

// LookupHostFunc is the DNS lookup function used by IsAllowedURL (overridable in unit tests).
type LookupHostFunc func(ctx context.Context, host string) ([]string, error)

var defaultLookupHost LookupHostFunc = func(ctx context.Context, host string) ([]string, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.IP.String())
	}
	return out, nil
}

// AllowPrivateAddons reports whether private/LAN custom stream addons are enabled via env.
func AllowPrivateAddons() bool {
	return os.Getenv("LUMEN_ALLOW_PRIVATE_ADDONS") == "1" || os.Getenv("SCREENTIME_ALLOW_PRIVATE_ADDONS") == "1"
}

func isLiteralIP(host string) bool {
	h := strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	if _, ok := parseIPv4(h); ok {
		return true
	}
	if strings.Contains(h, ":") {
		_, err := netip.ParseAddr(h)
		return err == nil
	}
	return false
}

// IsAllowedURL checks the URL scheme, credentials, hostname classification, and all resolved DNS records.
func IsAllowedURL(ctx context.Context, raw string, allowPrivate bool) bool {
	return IsAllowedURLWithLookup(ctx, raw, allowPrivate, defaultLookupHost)
}

// IsAllowedURLWithLookup is like IsAllowedURL but uses a custom DNS lookup function.
func IsAllowedURLWithLookup(ctx context.Context, raw string, allowPrivate bool, lookup LookupHostFunc) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	if u.User != nil {
		return false
	}
	hostname := u.Hostname()
	cls := ClassifyHost(hostname)
	if !IsAllowedClass(cls, allowPrivate) {
		return false
	}
	if isLiteralIP(hostname) {
		return true
	}

	if lookup == nil {
		lookup = defaultLookupHost
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addrs, err := lookup(lookupCtx, hostname)
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, addr := range addrs {
		addrClass := ClassifyHost(addr)
		if !IsAllowedClass(addrClass, allowPrivate) {
			return false
		}
		// Never allow a public hostname to resolve to loopback (prevents DNS rebinding to 127.0.0.1/::1).
		if cls == HostPublic && IsLoopbackIP(addr) {
			return false
		}
	}
	return true
}

// NewGuardedDialer returns a net.Dialer whose Control callback validates the resolved peer IP
// at the socket level immediately before connect(), eliminating DNS-rebinding TOCTOU windows.
func NewGuardedDialer(allowPrivate bool, originalHostClass HostClass) *net.Dialer {
	return &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				host = address
			}
			cls := ClassifyHost(host)
			if !IsAllowedClass(cls, allowPrivate) {
				return fmt.Errorf("%w: %s", ErrAddressNotAllowed, host)
			}
			if originalHostClass == HostPublic && IsLoopbackIP(host) {
				return fmt.Errorf("%w: DNS rebinding to loopback %s", ErrAddressNotAllowed, host)
			}
			return nil
		},
	}
}

// FetchOptions configures GuardedFetch.
type FetchOptions struct {
	AllowPrivate bool
	Method       string
	Headers      map[string]string
	Body         io.Reader
	Timeout      time.Duration
	Lookup       LookupHostFunc
}

// GuardedFetch executes an HTTP request verifying the initial URL, socket-level IP via net.Dialer.Control,
// and up to MaxRedirectHops (3) manual HTTP 30x redirect hops.
func GuardedFetch(ctx context.Context, rawURL string, opts FetchOptions) (*http.Response, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		// Wrap response body on success so cancel is called when body is closed; on error cancel immediately.
		resp, err := guardedFetchInternal(ctx, rawURL, opts)
		if err != nil {
			cancel()
			return nil, err
		}
		resp.Body = &cancelOnCloseReadCloser{ReadCloser: resp.Body, cancel: cancel}
		return resp, nil
	}
	return guardedFetchInternal(ctx, rawURL, opts)
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnCloseReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

func guardedFetchInternal(ctx context.Context, rawURL string, opts FetchOptions) (*http.Response, error) {
	current := strings.TrimSpace(rawURL)
	method := opts.Method
	if method == "" {
		method = http.MethodGet
	}

	for hop := 0; hop <= MaxRedirectHops; hop++ {
		if !IsAllowedURLWithLookup(ctx, current, opts.AllowPrivate, opts.Lookup) {
			return nil, ErrAddressNotAllowed
		}
		parsedURL, err := url.Parse(current)
		if err != nil {
			return nil, ErrAddressNotAllowed
		}
		hostCls := ClassifyHost(parsedURL.Hostname())
		dialer := NewGuardedDialer(opts.AllowPrivate, hostCls)
		transport := &http.Transport{
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   8 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
		}
		client := &http.Client{
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		var reqBody io.Reader
		if hop == 0 {
			reqBody = opts.Body
		}
		req, err := http.NewRequestWithContext(ctx, method, current, reqBody)
		if err != nil {
			return nil, err
		}
		for k, v := range opts.Headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			if loc != "" {
				_ = resp.Body.Close()
				nextURL, err := parsedURL.Parse(loc)
				if err != nil {
					return nil, ErrAddressNotAllowed
				}
				current = nextURL.String()
				if resp.StatusCode == http.StatusSeeOther || (method == http.MethodPost && (resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusFound)) {
					method = http.MethodGet
				}
				continue
			}
		}
		return resp, nil
	}

	return nil, ErrTooManyRedirects
}

// ReadBoundedText reads a response body up to maxBytes (default 2 MB), returning ErrResponseTooLarge if exceeded.
func ReadBoundedText(resp *http.Response, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxResponseBytes
	}
	defer func() { _ = resp.Body.Close() }()

	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > maxBytes {
			return "", ErrResponseTooLarge
		}
	}

	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if int64(len(data)) > maxBytes {
		return "", ErrResponseTooLarge
	}
	return string(data), nil
}
