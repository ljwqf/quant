---
name: OKX API proxy for China mainland
description: Tencent Cloud server cannot reach OKX API directly, requires SOCKS5 proxy on RackNerd server. Proxy is currently DOWN.
type: project
originSessionId: a2aa21b0-966c-4e8e-819a-10242a8c9bf0
---
China mainland servers (like Tencent Cloud 132.232.231.41) cannot access OKX API endpoints directly due to network restrictions.

**Solution**: Added `proxy_url` field in `internal/config/config.go` and wired it into both REST client (`internal/exchange/okx/rest_client.go`) and WebSocket client (`internal/exchange/okx/ws_client.go`).

**Why**: OKX API is blocked from China mainland IP ranges; proxy allows traffic to route through an overseas relay.

**How to apply**: When deploying to any China-based server, ensure `proxy_url` is set in config to an accessible proxy endpoint. Overseas servers (like RackNerd) do not need this.

**IMPORTANT (2026-04-22)**: The RackNerd SOCKS5 proxy at 23.95.165.223:1080 is DOWN. SSH to RackNerd on port 10087 is also refused. The proxy service needs to be restarted on the RackNerd server before the Tencent Cloud trader can connect to OKX.
