---
id: REQ-0011
title: Outbound URL, DNS resolution, and redirect-hop SSRF protection with bounded response reads
status: active
because: [FCT-0008, FCT-0010]
---

## Statement
Every server-side fetch to a user-supplied or playlist-supplied URL MUST prevent Server-Side Request Forgery (SSRF), DNS rebinding, and memory exhaustion:
1. **Scheme & credential rejection**: Allow only `http:` and `https:` URLs without embedded `username` or `password` credentials.
2. **Host & IP classification**: Classify IPv4, IPv6, and IPv4-mapped IPv6 (`::ffff:a.b.c.d` and hex form) addresses into `loopback` (`127.0.0.0/8`, `::1`, `0.0.0.0/8`, `localhost`), `link-local` (`169.254.0.0/16`, `fe80::/10`, `100.64.0.0/10`), `private` (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fc00::/7`, `.local`, `.internal`, `.lan`, `.home`, `.arpa`), and `public`.
3. **Always block loopback and link-local**: Never permit `loopback` or `link-local` addresses under any configuration; permit `private` RFC1918 addresses only when explicitly enabled for self-hosted LAN addons/indexers.
4. **DNS resolution, socket-level IP verification, & manual redirect validation**: Resolve all DNS records for non-literal hostnames and verify that *every* resolved address (including the final socket destination address prior to connecting) is permitted; follow at most `3` HTTP `30x` redirect hops manually (`redirect: 'manual'`), re-validating each hop's target URL and DNS resolution before connecting.
5. **Bounded upstream text reads**: Abort any upstream manifest, subtitle, or playlist read whose `Content-Length` or streamed byte count exceeds the configured limit (`2 MB` default).

## Acceptance
- Given a user-supplied URL whose public hostname resolves via DNS to `127.0.0.1` or `169.254.169.254`, or redirects (`302`) to `http://192.168.1.1/admin`
- When the server performs a guarded fetch with `allowPrivate: false`
- Then the request is rejected before connecting to the target address
