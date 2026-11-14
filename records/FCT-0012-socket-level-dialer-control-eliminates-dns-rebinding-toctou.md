---
id: FCT-0012
title: Socket-level dialer address verification eliminates the DNS-rebinding TOCTOU window between hostname lookup and TCP connection
status: active
evidence: "Verified in internal/netguard/guarded_fetch.go (NewGuardedDialer) and internal/netguard/netguard_test.go: net.Dialer.Control inspects the resolved syscall.RawConn network address immediately before connect(2), blocking loopback, link-local, and unauthorized RFC1918 addresses even if DNS records mutate between initial resolution and socket connection."
---

## Statement
Application-layer pre-flight DNS resolution (`dns.lookup` followed by `fetch`) leaves a time-of-check to time-of-use (TOCTOU) window in which an attacker-controlled DNS server with `TTL=0` can return a public IP during pre-flight validation and a loopback (`127.0.0.1`) or link-local (`169.254.169.254`) IP when the HTTP client opens its socket. Enforcing IP classification inside the transport dialer's socket-control hook (`net.Dialer.Control`) inspects the exact destination IP address passed to the operating system's `connect` syscall, closing the DNS-rebinding TOCTOU window completely across initial connections and manual redirect hops.
