package netguard

import (
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// HostClass categorizes a host or IP address for outbound SSRF policy enforcement.
type HostClass string

const (
	HostPublic  HostClass = "public"
	HostPrivate HostClass = "private"
	HostBlocked HostClass = "blocked"
)

var (
	privateTLDsRe = regexp.MustCompile(`\.(localhost|local|lan|internal|home\.arpa|home|corp|arpa)$`)
	ipv6LinkLocal = regexp.MustCompile(`^fe[89ab]`)
	ipv6ULA       = regexp.MustCompile(`^f[cd]`)
)

func parseIPv4(host string) ([4]int, bool) {
	parts := strings.Split(host, ".")
	if len(parts) != 4 {
		return [4]int{}, false
	}
	var out [4]int
	for i, p := range parts {
		if len(p) == 0 || len(p) > 3 {
			return [4]int{}, false
		}
		for j := 0; j < len(p); j++ {
			if p[j] < '0' || p[j] > '9' {
				return [4]int{}, false
			}
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return [4]int{}, false
		}
		out[i] = n
	}
	return out, true
}

func classifyIPv4(octets [4]int) HostClass {
	a, b := octets[0], octets[1]
	if a == 0 || (a == 169 && b == 254) || a >= 224 {
		return HostBlocked
	}
	if a == 127 || a == 10 || (a == 172 && b >= 16 && b <= 31) || (a == 192 && b == 168) || (a == 100 && b >= 64 && b <= 127) {
		return HostPrivate
	}
	return HostPublic
}

// IsLoopbackIP reports whether an IP or hostname is in the loopback range (127.0.0.0/8, ::1, localhost).
func IsLoopbackIP(hostname string) bool {
	h := strings.ToLower(strings.TrimSpace(hostname))
	h = strings.TrimPrefix(h, "[")
	h = strings.TrimSuffix(h, "]")
	h = strings.TrimSuffix(h, ".")
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || h == "::1" {
		return true
	}
	if v4, ok := parseIPv4(h); ok {
		return v4[0] == 127
	}
	if addr, err := netip.ParseAddr(h); err == nil {
		return addr.Unmap().IsLoopback()
	}
	return false
}

// ClassifyHost classifies a hostname or literal IPv4/IPv6 address into public, private, or blocked.
func ClassifyHost(hostname string) HostClass {
	h := strings.ToLower(strings.TrimSpace(hostname))
	h = strings.TrimPrefix(h, "[")
	h = strings.TrimSuffix(h, "]")
	h = strings.TrimSuffix(h, ".")
	if h == "" {
		return HostBlocked
	}

	if v4, ok := parseIPv4(h); ok {
		return classifyIPv4(v4)
	}

	if strings.Contains(h, ":") {
		if h == "::" {
			return HostBlocked
		}
		if h == "::1" {
			return HostPrivate
		}
		if strings.HasPrefix(h, "::ffff:") {
			rest := strings.TrimPrefix(h, "::ffff:")
			if v4, ok := parseIPv4(rest); ok {
				return classifyIPv4(v4)
			}
			// Hex-form IPv4-mapped IPv6 address (e.g. ::ffff:a9fe:a9fe or ::ffff:7f00:1)
			if addr, err := netip.ParseAddr(h); err == nil && addr.Is4In6() {
				u := addr.Unmap().As4()
				cls := classifyIPv4([4]int{int(u[0]), int(u[1]), int(u[2]), int(u[3])})
				if cls != HostPublic {
					return HostBlocked
				}
			}
			return HostBlocked
		}
		if ipv6LinkLocal.MatchString(h) || strings.HasPrefix(h, "ff") {
			return HostBlocked
		}
		if ipv6ULA.MatchString(h) {
			return HostPrivate
		}
		if addr, err := netip.ParseAddr(h); err == nil {
			if addr.IsUnspecified() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() {
				return HostBlocked
			}
			if addr.IsLoopback() || addr.IsPrivate() {
				return HostPrivate
			}
		}
		return HostPublic
	}

	if h == "metadata.google.internal" || h == "metadata" {
		return HostBlocked
	}
	if h == "localhost" || !strings.Contains(h, ".") || privateTLDsRe.MatchString(h) {
		return HostPrivate
	}
	return HostPublic
}

// IsAllowedClass reports whether a HostClass is permitted given allowPrivate.
func IsAllowedClass(cls HostClass, allowPrivate bool) bool {
	return cls == HostPublic || (cls == HostPrivate && allowPrivate)
}

// CheckServiceURL normalizes the base address of a user-supplied service:
// http(s) only, no embedded credentials, query or fragment, no trailing slash.
// Returns "" when the URL is not allowed.
func CheckServiceURL(raw string, allowPrivate bool) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Contains(trimmed, "?") || strings.Contains(trimmed, "#") {
		return ""
	}
	hostname := u.Hostname()
	if !IsAllowedClass(ClassifyHost(hostname), allowPrivate) {
		return ""
	}
	return strings.TrimRight(u.String(), "/")
}
