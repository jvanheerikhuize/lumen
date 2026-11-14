---
id: FCT-0008
title: Server-side fetching of user-supplied URLs exposes the host network to SSRF and DNS rebinding if unchecked
status: active
evidence: "Verified by security unit tests in src/__tests__/security.test.ts covering IPv4/IPv6 loopback, RFC1918 private ranges, link-local metadata endpoints (169.254.169.254), IPv4-mapped IPv6 addresses, and HTTP 30x redirect hops."
---

## Statement
When a self-hosted home server fetches user-supplied addon manifests, indexer endpoints, or live stream URLs (`/api/live/proxy`, `/api/live/status`, custom stream addons), an attacker or malicious playlist can target loopback (`127.0.0.0/8`, `::1`), link-local cloud/router metadata (`169.254.0.0/16`, `fe80::/10`), or internal LAN services (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) directly, via DNS resolution of a public hostname to a private IP, or via an HTTP `30x` redirect hop.
