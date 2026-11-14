---
id: FCT-0010
title: Socket-level connection-time IP verification eliminates DNS-rebinding TOCTOU windows
status: active
evidence: "Verified by outbound network guard tests covering IPv4/IPv6 loopback, RFC1918 private ranges, CGNAT, link-local metadata endpoints (169.254.169.254), IPv4-mapped IPv6 addresses, and manual 30x redirect hops."
---

## Statement
Validating a hostname via a separate pre-request DNS lookup before opening an HTTP connection leaves a time-of-check-to-time-of-use (TOCTOU) window in which an attacker with a short-TTL DNS record can rebind the hostname to `127.0.0.1` or `169.254.169.254` on the second lookup. Inspecting the resolved destination IP address directly on the network socket dialer callback immediately prior to establishing the TCP connection—combined with manual redirect validation (`maxRedirects = 3`)—eliminates the DNS-rebinding TOCTOU window completely.
