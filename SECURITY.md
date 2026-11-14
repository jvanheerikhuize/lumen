# Security

## Intended use

**Lumen** is built to run on a trusted home network. It has **no login**, so:

- Don't expose it to the internet as is (no port forwarding to it). If you need remote access, put it behind a VPN such as WireGuard or Tailscale, or behind a reverse proxy that adds authentication.
- Anyone who can reach it can use it, including starting torrent downloads on the machine it runs on.

## What the server protects against

Several features make the server fetch addresses that come from the browser: stream addons, Prowlarr/Jackett, direct streams, subtitles, and live channels. To keep them from being used to reach other machines on your network (`internal/netguard`):

- **Stream addons and direct streams** (including those inspected or converted with `ffprobe`/`ffmpeg`) must be on the public internet. Every hostname is checked against all addresses it resolves to, and the transport dialer's `net.Dialer.Control` hook verifies the exact peer IP address immediately before the socket `connect(2)` syscall (preventing DNS-rebinding TOCTOU attacks). Addons on your own private LAN are allowed only when `LUMEN_ALLOW_PRIVATE_ADDONS=1` is set.
- **Prowlarr and Jackett** may be on your local RFC1918 network, but loopback (`127.0.0.0/8`, `::1`) and link-local / cloud-metadata addresses (`169.254.0.0/16`, `fe80::/10`, `100.64.0.0/10`) are always refused. Only their fixed search and status API paths are called, and responses are never passed back raw.
- **Redirects** are followed manually one hop at a time (up to 3 hops), and every hop is re-validated before connecting.
- **Bounded upstream reads** cap manifest, playlist, and subtitle responses at `2 MB` to prevent memory exhaustion.
- **Subtitles** are only fetched over `https:` from `*.strem.io` (`strem.io` and subdomains).
- **API keys** for Prowlarr and Jackett are sent to the server in request headers (`X-Prowlarr-Api-Key`, `X-Jackett-Api-Key`), so they don't end up in URLs or request logs. They are stored in the browser's local storage.

## Known limits

- `ffmpeg` follows HTTP redirects itself once launched on a verified public direct-stream URL, so a public direct-stream URL that redirects mid-stream into your local network isn't intercepted by `ffmpeg`.
- The Prowlarr/Jackett "Test connection" endpoint (`/api/integrations/test`) reveals whether an address on your network answers.

These are acceptable on a trusted home network, and are further reasons not to expose the appliance directly to the public internet.

## Reporting a vulnerability

Please report security issues privately through GitHub: **Security → Report a vulnerability** on this repository. Don't open a public issue. You'll get a reply as soon as possible.
