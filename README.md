<div align="center">

<img src="assets/logo.svg" alt="Sub2API Logo" width="128" />

# Sub2API · DevKu

[![Go](https://img.shields.io/badge/Go-1.27.3-00ADD8.svg)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3.5-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D.svg)](https://redis.io/)

**AI API gateway with Desktop enterprise management, accounting and BI**

English | [简体中文](README_CN.md)

[Repository](https://github.com/DevKuAI/devku-sub2api) · [Releases](https://github.com/DevKuAI/devku-sub2api/releases) · [Upstream](https://github.com/Wei-Shaw/sub2api)

</div>

## Important Notice

Review these conditions before deploying or using Sub2API:

- **Upstream terms:** Using Sub2API may violate the terms of service of Anthropic or other upstream providers. Review each provider's agreement before use. You are responsible for the resulting risks.
- **Legal compliance:** Use Sub2API only when permitted by the laws and regulations of your country or region. Do not use it for unlawful activity.
- **Disclaimer:** This project is provided only for technical learning and research. The authors are not liable for account suspensions, service interruptions, data loss, or other direct or indirect damages resulting from its use.
- **No commercial authorization:** The developers have not authorized any individual or organization to operate commercial services in the project's name or based on the project. The operator is solely responsible for any resulting disputes, losses, or legal liability.

## Overview

Sub2API is a self-hosted AI API gateway that manages upstream OAuth and API-key accounts, routes requests, distributes quotas, and records usage and costs. This repository is the DevKu fork of [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api), with Desktop enterprise management, account-facing subscription views, enterprise accounting, and BI administration.

Use this README to choose a deployment method and configure the service. See the [documentation index](docs/README.md) for feature guides and API contracts. Deployment commands below use DevKu releases and images; installing from `main` may include changes newer than the latest release.

[Features](#features) · [Deployment](#deployment) · [Configuration](#configuration) · [Providers and compatibility](#providers-and-compatibility) · [Development](#development) · [Documentation](#documentation) · [License](#license)

## Features

| Area | Capabilities |
| --- | --- |
| Gateway | Messages, Responses, Chat Completions, Gemini and provider-specific protocols; routing, sticky sessions, load balancing and composite groups. Protocol support depends on the selected provider. |
| Accounts and API keys | OAuth/API-key upstream accounts, user key creation limits, model allowlists, account bindings and read-only subscription-account status and quota views. |
| Usage and limits | Token and cost accounting, user/account concurrency limits, request/token rate limits, per-key spending windows, output TPS and operations dashboards. Output TPS is distinct from system throughput; long-context billing labels show the applied pricing tier. |
| Payments | EasyPay, Alipay, WeChat Pay and Stripe for self-service top-ups; [payment configuration](docs/payments/PAYMENT.md). |
| Desktop enterprises | Organizations and members, enterprise-to-Web-account binding, managed Model Tokens, model configuration, usage statistics, session/conversation reporting and daily reports. Reporting is controlled per enterprise. |
| Desktop resources and updates | MCP/Skill package management and authenticated distribution, plus Desktop update publishing backed by S3-compatible storage. Private resource storage is separate from public update artifacts. |
| Enterprise accounting | Transfers and tax calculations, VCC allocations/transactions/refunds, contracts, invoices, AI Invoice, expenses and installments, reconciliation, CSV export, private attachments and audits. Currencies are tracked separately. |
| Enterprise BI | Independent WeChat identity, enterprise invitations and feature grants, Desktop usage and daily/weekly/monthly reports, and administrative metadata. WeChat-device and production-capacity acceptance remains tracked in the [BI documentation](docs/mobile-bi/README.md). |
| Administration and extensions | Web administration, [Admin API keys scoped by operation and resource](docs/admin-api-keys.md), operational monitoring, plugin host services and iframe integration with external systems. |

### Enterprise account access

- Desktop organizations bind an existing Web account through `gateway_user_id`. The bound account can view its enterprise at `/desktop/organization`.
- Administrators manage all enterprises' accounting at `/admin/accounting`. A bound Web account gets **Financial records** at `/organization/accounting`, even before any accounting records exist. For a bound Admin, this appears under **My account**.
- The enterprise view is read-only and contains only its own transfers, card spending, and explicitly shared contracts/invoices. Internal expenses, profit and sensitive card data are excluded. Desktop member sessions cannot access accounting.
- Financial accounting is independent of online payments, Gateway balance and API usage billing; recording a transfer does not top up a Gateway account.

## Deployment

| Method | Requirements | Initialization |
| --- | --- | --- |
| **Docker Compose (recommended)** | Docker 20.10+ and Compose v2+ | Includes PostgreSQL and Redis; automatic first-run setup. |
| Linux binary | Linux amd64/arm64, systemd, root for installation; PostgreSQL 15+ and Redis 7+ already running | Web setup wizard or CLI `--setup`. |
| Apple `container` | Apple silicon, macOS 26, `container` 1.1.0+ | Local three-service stack; [operations and limitations](deploy/APPLE_CONTAINER.md). |
| Source build | Go 1.27.3, Node.js 24, pnpm 9, PostgreSQL and Redis | See [Development](#development). |

### Docker Compose quick start

Run in a new deployment directory:

```bash
mkdir -p sub2api-deploy
cd sub2api-deploy
curl -fsSL https://raw.githubusercontent.com/DevKuAI/devku-sub2api/main/deploy/docker-deploy.sh -o docker-deploy.sh
bash docker-deploy.sh
docker compose up -d
docker compose ps
docker compose logs -f sub2api
```

The preparation script downloads the local-directory Compose variant as `docker-compose.yml`, writes `.env` with generated PostgreSQL/JWT/Desktop/TOTP secrets, and creates `data/`, `postgres_data/` and `redis_data/`. It displays generated credentials; keep both the output and `.env` private.

Open `http://YOUR_SERVER_IP:8080`. Container deployments initialize automatically. If `ADMIN_EMAIL` or `ADMIN_PASSWORD` is blank, first-run logs show the generated login credentials:

```bash
docker compose logs sub2api | grep "Generated admin"
```

For production, configure HTTPS and the reverse proxy before exposing the service. Complete the admin setup, add upstream accounts and groups, and create a user API key for the required group. Supported client configuration is available through **Use Key** on the API-key page.

<details>
<summary>Manual Compose deployment</summary>

```bash
git clone https://github.com/DevKuAI/devku-sub2api.git sub2api
cd sub2api/deploy
cp .env.example .env
chmod 600 .env
```

Edit `.env` before starting. Generate each of `POSTGRES_PASSWORD`, `JWT_SECRET` and `TOTP_ENCRYPTION_KEY` independently with `openssl rand -hex 32`. Keep JWT and encryption keys stable across restarts and restores. Optionally set `ADMIN_EMAIL`, `ADMIN_PASSWORD` and `SERVER_PORT` (default `8080`); avoid guessable administrator credentials.

```bash
mkdir -p data postgres_data redis_data
docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml ps
docker compose -f docker-compose.local.yml logs -f sub2api
```

`docker-compose.local.yml` uses local directories; `docker-compose.yml` in the repository uses named volumes. Use the same `-f` selection for later operations. The preparation script's generated `docker-compose.yml` already uses local directories.

</details>

### Linux binary

The installer downloads a DevKu release to `/opt/sub2api` and configures the system user and systemd unit:

```bash
curl -fsSL https://raw.githubusercontent.com/DevKuAI/devku-sub2api/main/deploy/install.sh -o install.sh
sudo bash install.sh
sudo systemctl start sub2api
sudo systemctl enable sub2api
sudo journalctl -u sub2api -f
```

On a fresh installation, visit `http://YOUR_SERVER_IP:8080` and complete database, Redis and administrator setup. Do not pre-create `config.yaml`: its presence skips the wizard. For CLI setup, run the installed binary with `--setup` under the service's user and data-directory context. Historical `default.admin_email` / `default.admin_password` YAML fields do not initialize the administrator.

### Updates and backups

For Compose, back up the database and persistent data before recreating containers:

```bash
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 sub2api
```

Use `-f docker-compose.local.yml` for a manual local-directory deployment. Pin an explicit DevKu image tag if deployment reproducibility is required. Binary installations can use **Check for Updates** in the admin dashboard, including the supported rollback flow; Compose upgrades should use image updates.

Back up PostgreSQL, application data/configuration, `.env` and encryption keys; preserve Redis state used for sessions and media-task bindings. Do not copy live database directories as a backup: use database-aware backups or stop the stack before archiving local directories. Private S3 objects require a separate backup/retention policy. See [deployment operations](deploy/README.md) for migration and lifecycle commands.

## Configuration

Use [deploy/.env.example](deploy/.env.example) for containers and [deploy/config.example.yaml](deploy/config.example.yaml) for binary deployments. Non-empty environment values override YAML; Compose templates may also supply defaults. After changing `.env`, recreate containers with `docker compose up -d`; restarting alone does not load new environment values.

| Setting | Purpose |
| --- | --- |
| `JWT_SECRET` | Fixed Web-session signing secret; generate with `openssl rand -hex 32`. |
| `TOTP_ENCRYPTION_KEY` | Fixed encryption key for 2FA and sensitive VCC fields; generate with `openssl rand -hex 32` and retain during restore. |
| `DESKTOP_ENABLED` | Default `false`. Enabling requires `DESKTOP_JWT_SECRET` (independent, `openssl rand -base64 32`) and an externally reachable HTTPS `DESKTOP_PUBLIC_GATEWAY_BASE_URL` ending in `/v1`. Controls Desktop APIs and organization accounting access. |
| `DESKTOP_UPDATE_STORAGE_RESOURCE_BUCKET` | Private MCP/Skill and accounting attachment bucket, using the existing `DESKTOP_UPDATE_STORAGE_*` S3 connection. Accounting objects use `accounting/{organization_id}/{attachment_id}/`; no separate accounting bucket is needed. |
| `BI_ENABLED` | Default `false`. Requires real `BI_APPID`/`BI_APP_SECRET`, independent base64 `BI_JWT_SECRET`/`BI_IDENTITY_SECRET`, and a published BI privacy notice with its own HTTPS site URL. See [BI configuration](docs/mobile-bi/configuration.md). |

Accounting remains usable for basic records without S3. Uploads and confirmations requiring evidence return `ACCOUNTING_STORAGE_UNAVAILABLE`. Card-sensitive fields require a fixed encryption key; read APIs return only the last four digits. Record and attachment sharing are independent and default to private. See [accounting configuration and API rules](docs/accounting-management/implementation.md).

### Reverse proxy and security

- Configure HTTPS, CORS origins, CSP and upstream URL validation for the deployment. When URL allowlisting is disabled, HTTP upstream URLs are allowed by default; explicitly set `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=false` for HTTPS-only upstream access. HTTP exposes keys and data to interception/MITM.
- `security.trust_forwarded_ip_for_api_key_acl` defaults to `true` for upgrade compatibility. In that mode, restrict origin access to the CDN/proxy and overwrite trusted headers at the edge. To enforce Gin's trust boundary, disable it and set `server.trusted_proxies` to the exact directly connected proxy CIDRs.
- `SECURITY_FORWARDED_CLIENT_IP_HEADERS=True-Client-IP,X-CDN-Client-IP` accepts up to 16 validated, canonicalized, de-duplicated header names. Admin security settings can update them without restart. With legacy takeover disabled, raw custom and built-in forwarding headers are ignored. See [edge security and migration](deploy/EDGE_SECURITY.md).
- If URL validation or response-header filtering is disabled, enforce upstream egress allowlists, block private/loopback/link-local targets, enforce TLS and strip sensitive response headers at the proxy. Review `billing.circuit_breaker` and `turnstile.required` for production.

Nginx drops underscore headers such as `session_id` by default, which breaks Codex sticky routing. Enable them in its `http` block:

```nginx
underscores_in_headers on;
```

<details>
<summary>Codex Fast/Flex, WebSocket and h2c settings</summary>

### Codex Fast/Flex policies

The OpenAI Fast/Flex policy under the admin gateway settings controls `service_tier` in request bodies. It does not change the Codex client model catalog or automatically expose Speed or `/fast` in the UI.

Supported actions:

- `pass`: preserve the client's `service_tier`; normalize `fast` to the upstream value `priority`.
- `filter`: remove `service_tier` and request normal priority.
- `block`: reject matching Fast/Flex requests.
- `force_priority`: set matching requests to `priority`, billed at Priority/Fast rates. To preserve existing rule semantics, `all` only matches an explicitly supplied tier. To upgrade OpenAI requests that omit `service_tier`, add a `service_tier=missing + force_priority` rule. Missing tiers are not injected for other platforms. The request can use Fast even when Codex does not display its Fast status.

The Codex Fast control depends on the client model catalog declaring `additional_speed_tiers: ["fast"]` and the corresponding `service_tiers`. If these fields are absent when connecting through an API Key or a custom provider, configuring `force_priority` and restarting Codex will not make the Speed option appear.

Set the default request tier in `~/.codex/config.toml`:

```toml
service_tier = "fast"

[features]
fast_mode = true
```

`service_tier = "fast"` includes the Fast setting in requests; `features.fast_mode` only enables the client feature. Without Fast capability in the model catalog, `/fast` may remain unavailable. Check Sub2API usage records to confirm whether the final `service_tier` is `priority`.

### OpenAI Responses WebSocket limits and HTTP fallback

`gateway.openai_ws` bounds the lifetime and aggregate count of client-facing
Responses WebSocket sessions. These safeguards apply independently from
per-turn user and account concurrency slots, which are released between turns.

```yaml
gateway:
  openai_ws:
    # Total time to receive and decompress the first client message.
    client_first_message_timeout_seconds: 30
    # Close a client socket idle between completed turns; 0 disables this safeguard.
    ingress_inter_turn_idle_timeout_seconds: 300
    # Distributed API-key limit for live client ingress sessions; 0 disables it.
    max_ingress_connections_per_api_key: 64
```

The first-message timeout is a total read deadline. Deployments that accept
large contexts or image-heavy requests over slower links can raise it to
120-300 seconds. It expires before HTTP bridge routing, so bridge mode does not
override this limit.

The connection cap is coordinated through Redis using a 60-second lease that
is refreshed every 20 seconds. A process that cannot confirm a lease for a
full lease lifetime closes its local WebSocket rather than continuing outside
the global cap.

Enable the v2 mode router before selecting an account-level WS mode such as
`http_bridge`:

```yaml
gateway:
  openai_ws:
    mode_router_v2_enabled: true
```

Or set `GATEWAY_OPENAI_WS_MODE_ROUTER_V2_ENABLED=true` in the environment.
Use `http_bridge` for client-WebSocket/upstream-HTTP operation when rolling out
or mitigating upstream WebSocket issues.

#### Force OpenAI upstream HTTP/SSE

When an egress proxy or network repeatedly reconnects OpenAI Responses
WebSockets, set the global fallback in the persisted deployment configuration:

```yaml
gateway:
  openai_ws:
    force_http: true
```

For Compose and Apple container deployments, the equivalent `.env` setting is:

```bash
GATEWAY_OPENAI_WS_FORCE_HTTP=true
```

This selects HTTP/SSE for OpenAI upstream Responses traffic that would
otherwise use WebSocket. It does not change the client-facing protocol or force
HTTP/1.1; configure `gateway.openai_http2.enabled` (or
`GATEWAY_OPENAI_HTTP2_ENABLED=false`) separately when a proxy is incompatible
with HTTP/2. Unlike the account-level `http_bridge` mode, this global fallback
takes effect without enabling `mode_router_v2_enabled`. Keep the setting in the
deployment's persisted `.env` or `config.yaml`, rather than inside a running
container, so it is read again after an image update or container recreation.

### HTTP/2 (h2c) and HTTP/1.1 fallback

The backend's plaintext port supports h2c with HTTP/1.1 fallback for WebSockets and older clients. Browsers generally do not support h2c; it primarily benefits proxy and internal-network traffic. A Caddy upstream transport can use:

```caddyfile
transport http {
    versions h2c h1
}
```

Verify with an HTTP/2-capable curl; the WebSocket check requires an administrator token:

```bash
curl --http2-prior-knowledge -I http://localhost:8080/health
curl --http1.1 -I http://localhost:8080/health
websocat -H="Sec-WebSocket-Protocol: sub2api-admin, jwt.<ADMIN_TOKEN>" ws://localhost:8080/api/v1/admin/ops/ws/qps
```

</details>

### Simple Mode

Simple Mode is designed for individual developers or internal teams who want quick access without full SaaS features.

- Enable: Set environment variable `RUN_MODE=simple`
- Default groups are seeded on each startup. Set `SIMPLE_MODE_AUTO_CREATE_DEFAULT_GROUPS=false` (or YAML `simple_mode.auto_create_default_groups: false`) to manage groups yourself. The default is `true`; disabling it does not delete existing groups or change runtime auto-binding or admin concurrency setup.
- Difference: Hides SaaS-related features and skips billing process
- Optional key windows: Set `SIMPLE_MODE_KEY_RATE_LIMIT_ENABLED=true` to enforce each API key's configured 5-hour, daily, and 7-day spending windows. The default is `false`; balance and subscription debit remain bypassed when enabled.
- Window enforcement uses the database as its source of truth and records only API-key window usage. It is a post-request soft cap, so concurrent in-flight requests can overshoot by their final costs. Historical simple-mode usage is not backfilled.
- Security note: In production, you must also set `SIMPLE_MODE_CONFIRM=true` to allow startup

## Providers and compatibility

### v0.2.17 compatibility updates

- **OpenAI subscription models:** ChatGPT OAuth and SetupToken accounts exclude retired GPT-5.5 targets from routing and model catalogs. API-key accounts remain eligible; explicit mappings to supported replacements remain valid. Review client defaults and compact-only mappings; settings are not rewritten automatically. See [migration notes](docs/OPENAI_GPT55_RETIREMENT.md).
- **Gemini and Antigravity:** Adds Gemini 4 Argon catalogs and pricing, forwards thinking settings and streams thinking summaries through compatibility endpoints, preserves tool images and thought signatures, and handles Vertex service-account failures and shared-capacity retries.
- **Gateway compatibility:** Responses objects include `created_at`; `gateway.min_output_tokens` controls the conversion floor (default `128`, minimum `16`). Codex imports distinguish workspaces, Anthropic forwarding honors protocol base URLs, and OpenAI API-key accounts can use web-search emulation on `/v1/messages`.
- **Payments and quotas:** EasyPay notifications are confirmed before crediting; pending refunds keep deductions until their outcome is resolved. Stripe resumes use the local client-secret snapshot, and GLM Coding Plan usage windows are corrected.

### v0.2.16 compatibility updates

- **Gemini explicit context caching**: create, list, retrieve, extend, and delete caches through `/v1beta/cachedContents`, then reference them with `cachedContent` in generation requests. AI Studio, Agent Platform (Vertex), and self-hosted upstream gateways with explicit caching enabled are supported. `gateway.gemini_cached_content_max_ttl_seconds` defaults to `86400`; storage fees are charged on creation or extension.
- **Gemini accounts**: Agent Platform (Vertex) supports API Key authentication and upstream model synchronization. Third-party gateway API keys and Vertex accounts no longer use AI Studio tiers or simulated quotas.
- **Model compatibility**: adds `claude-haiku-5-5`, raises the GPT-6.1 Sol generated context default to 500k, and removes long-context surcharges for subscription accounts. Administrators should review existing model mappings and allowlists.
- **Group localization**: administrators can maintain translated group names and descriptions. User-facing APIs select text using `Accept-Language`, with the original text as the fallback.

### Routing, video and plugin compatibility

- **Command Code and Cline:** Both support API-key accounts. Command Code selects native Messages, Responses or Chat Completions endpoints using model capabilities. Cline uses Chat Completions and converts incoming Messages/Responses; Cline credit and ClinePass limits affect their respective models independently.
- **Platform catalog:** Account creation, routing and default platform quotas share a catalog. Database platform CHECK constraints were removed by migration; the application validates registered platforms.
- **Model allowlists:** Group allowlists constrain both listing and request admission. The migration from `groups.models_list_config` to `model_allowlist` preserves data; review previously enabled lists because they now restrict requests too.
- **Seedance (Ark):** Native task creation, query and deletion require the Seedance capability on an OpenAI API-key account and group media permission. Billing happens when a query first observes `succeeded`, using `usage.completion_tokens`. There is no background polling or callback settlement; preserve Redis bindings and query completed tasks within their lifetime. See [Seedance configuration and billing](docs/media/seedance-api.md).
- **Plugin host services:** Redis KV is isolated by plugin identity; account-directory access is capability-limited and `plugin.status` is read-only. Optional negotiation remains compatible with older plugins. See the [Plugin API](backend/pkg/pluginapi/README.md) and [UI bridge](backend/pkg/pluginapi/docs/ui-bridge.md).
- **Antigravity attribution:** Only the Anthropic Messages-to-Antigravity conversion removes leading Claude attribution metadata. Native Anthropic forwarding is unchanged; do not globally disable attribution to address it.

<details>
<summary>Image models, Grok / xAI, TypeSafe / Jev and Antigravity details</summary>

### OpenAI Image Models

Supports `gpt-image-2.5-flare`, `gpt-image-2.5-sunburst`, and their `2026-09-08` snapshots through `/v1/images/generations` and `/v1/images/edits`. Supported `quality` values include `xhigh`, `max`, and `auto`. Valid custom sizes and detailed image usage are passed through unchanged.

OAuth / Setup Token image requests use a Responses orchestrator model to call the `image_generation` tool. The default orchestrator is `gpt-5.6-luna`. Set `SUB2API_IMAGES_MAIN_MODEL` to use a text model supported by the account; Docker Compose users must run `docker compose up -d` after changing `.env` to recreate the container. This setting does not replace the selected image model or override a text orchestrator model already provided in a `/v1/responses` request.

After upgrading, accounts without model restrictions support the new models automatically. Existing explicit account mappings or group allowlists must include the two 2.5 models, with dated snapshots added as needed; upgrading does not automatically expand administrator-defined model permissions. Built-in pricing covers the official text input, image input, and image output token rates, and is used for the 2.5 models if the remote pricing table has not been updated. Per-request or per-token billing still follows the existing group/channel configuration.

### Asynchronous Image Tasks

Long-running OpenAI/Grok image generation and editing can be submitted through `/v1/images/generations/async` or `/v1/images/edits/async`, then polled at `/v1/images/tasks/{task_id}` without holding a CDN connection open. See [Asynchronous Image Tasks](docs/media/ASYNC_IMAGE_TASKS.md) for request and response examples.

### Grok / xAI Support

Sub2API supports both Grok subscription accounts through xAI OAuth and standard xAI API-key accounts. Both account types forward OpenAI-compatible Responses traffic to xAI.

#### Supported Scope

- Platform name: `grok`
- Account types: OAuth subscription accounts and xAI API-key accounts
- Public Responses targets: `/v1/responses`, `/responses`, and `/backend-api/codex/responses`, forwarded to the Grok subscription proxy for OAuth accounts or `https://api.x.ai/v1/responses` for API-key accounts
- Public Claude-compatible target: `/v1/messages`, converted to xAI Responses and returned as Anthropic Messages output for Claude CLI style clients
- Public Chat Completions targets: `/v1/chat/completions` and `/chat/completions`, forwarded to the account-type-specific xAI upstream
- Codex CLI style Responses WebSocket ingress is accepted on the Responses targets and bridged to xAI HTTP/SSE Responses upstream
- Text models: `grok-4.5`, `grok-4.3`, `grok-build-0.1`, `grok-composer-2.5-fast`, `grok-4.20-0309-reasoning`, `grok-4.20-0309-non-reasoning`, and `grok-4.20-multi-agent-0309`
- Media targets for Grok groups: `/v1/images/generations`, `/images/generations`, `/v1/images/edits`, `/images/edits`, `/v1/videos/generations`, `/videos/generations`, `/v1/videos/edits`, `/videos/edits`, `/v1/videos/extensions`, `/videos/extensions`, `/v1/videos/{request_id}`, and `/videos/{request_id}`. Generation, editing, and extension requests require the group image-generation permission.
- Media models: `grok-imagine`, `grok-imagine-image-quality`, `grok-imagine-image`, `grok-imagine-image-2.0`, `grok-imagine-edit`, `grok-imagine-video`, and `grok-imagine-video-1.5`
- JSON image-edit and video-generation requests accept image references in `image`, `images`, `reference_images`, and `mask` objects. Use `url` for xAI-compatible payloads; the legacy `image_url` field remains accepted and is normalized to `url` before forwarding.
- Out of scope for this provider: TTS, transcription, browser automation, cookies, and Grok web scraping

#### OAuth Configuration

The Grok OAuth flow uses PKCE and does not require committing private secrets. The default client details follow the public xAI OAuth flow used by compatible clients, and every value can be overridden by environment variable:

| Variable | Default |
|----------|---------|
| `XAI_OAUTH_CLIENT_ID` | Public xAI OAuth client ID |
| `XAI_OAUTH_SCOPE` | `openid profile email offline_access grok-cli:access api:access` |
| `XAI_OAUTH_REDIRECT_URI` | `http://127.0.0.1:56121/callback` |
| `XAI_OAUTH_AUTHORIZE_URL` | `https://auth.x.ai/oauth2/authorize` |
| `XAI_OAUTH_TOKEN_URL` | `https://auth.x.ai/oauth2/token` |
| `XAI_BASE_URL` | `https://api.x.ai/v1`; runtime-diagnostics override (account `base_url` controls request forwarding) |
| `XAI_GROK_CLI_VERSION` | `0.2.114`; optional override for the client identity sent to `cli-chat-proxy.grok.com`. The pinned value is also the floor: an override below it is dropped |

Administrators can create Grok OAuth or API-key accounts from the dashboard. OAuth authorization and reauthorization are also available through the admin API:

| Endpoint | Purpose |
|----------|---------|
| `POST /api/v1/admin/grok/oauth/auth-url` | Generate an xAI OAuth authorization URL |
| `POST /api/v1/admin/grok/oauth/exchange-code` | Exchange a callback URL, query string, or code for OAuth credentials |
| `POST /api/v1/admin/grok/oauth/refresh-token` | Validate or refresh a Grok refresh token |
| `POST /api/v1/admin/grok/accounts/:id/refresh` | Refresh an existing Grok account |

OAuth credential storage reuses the existing account JSON fields: `access_token`, `refresh_token`, `token_type`, `expires_at`, `base_url`, optional `email`, optional `subscription_tier`, and `entitlement_status`. OAuth inference defaults to `https://cli-chat-proxy.grok.com/v1`; existing OAuth accounts that stored the old `https://api.x.ai/v1` default are redirected to the subscription proxy at runtime. Explicit custom upstreams remain unchanged.

For API-key accounts, select **Grok → API Key** in the create-account dialog. The official base URL defaults to `https://api.x.ai/v1`; credentials use the existing `base_url` and `api_key` account fields. OAuth accounts continue to use the subscription flow above.

#### Grok Build CLI Configuration

1. In the Sub2API admin dashboard, add either a `grok` OAuth account and complete xAI authorization, or add a Grok API-key account.
2. Create a Grok group, attach the account to it, then create a Sub2API API key assigned to that group.
3. In the user API-key page, click **Use Key** and select **Grok CLI**. The modal generates the correct file and base URL for macOS/Linux or Windows. It also provides an OpenCode configuration on the **OpenCode** tab.
4. If configuring manually, save the following as `~/.grok/config.toml` (Windows: `%USERPROFILE%\.grok\config.toml`):

```toml
[models]
default = "grok"
web_search = "grok"

[model."grok"]
model = "grok-4.5"
base_url = "https://your-sub2api.example.com/v1"
name = "Grok 4.5"
api_key = "sk-your-sub2api-key"
api_backend = "responses"
context_window = 1000000
supports_backend_search = true
```

Back up an existing `config.toml` before merging the entry. The file contains a Sub2API API key, so keep it private and restrict its permissions where supported. Verify the effective configuration and make a smoke request:

```bash
grok inspect
grok -p "Reply with sub2api-ok" -m grok
```

The `base_url` above is the public Sub2API URL ending in `/v1`, not `api.x.ai` or the internal xAI OAuth proxy URL.

#### Usage And Quota Display

xAI quota is passive. Sub2API does not invent subscription quota values; it records whitelisted xAI rate-limit headers from successful or rate-limited upstream responses when xAI sends them. Before the first usable upstream response, the dashboard shows quota as unknown and still displays local Sub2API usage stats.

`401` responses temporarily remove accounts with invalid credentials from scheduling. `403` responses are treated as access or entitlement failures instead of token-refresh loops. `429` responses use `Retry-After` or a short cooldown to temporarily remove the account from scheduling.

New Grok image and video generation requests use a media-specific eligibility check. API-key accounts remain eligible. OAuth accounts with explicit Free or forbidden billing evidence are excluded from new media generation. Missing or malformed observations are probed before dispatch; a successful but incomplete billing response is treated as `billing_inconclusive` and remains eligible for backwards compatibility, because an unknown billing schema is not proof that the account lacks media entitlement. Operators can quarantine a known-bad account with `extra.grok_media_eligible=false` or force-enable a verified account with `true`. Imports run the billing-first quota probe proactively. Chat requests and video status lookups are not affected by this media-only quarantine. If no eligible account remains, the media endpoint returns HTTP `503` with error type `grok_media_no_eligible_account`.

Administrators can override automatic media eligibility through the account create/update API by setting `extra.grok_media_eligible` to `false` (exclude) or `true` (force eligible). On update, set it to `null` to remove the override and return to automatic probe-based behavior; omitting the field preserves the current override. A weekly allowance period alone is not treated as a paid tier signal. Successful image responses must contain at least one actual image output; empty HTTP `200` responses trigger account failover instead of being counted and returned as successful generations.

### TypeSafe / Jev Support

Sub2API supports TypeSafe API-key accounts through Jev's native, non-streaming System One protocol.

- Platform: `typesafe`; account type: API Key
- Default upstream: `https://api.typesafe.ai`
- Public endpoint: `POST /v1/systemone`
- Model: `jev-latest`, also returned by `/v1/models` for TypeSafe groups
- Questions: `noul`, `choice`, and `score`

Requests and successful responses retain the native System One JSON structure. This endpoint is not compatible with Chat Completions, Responses, Anthropic Messages, or streaming clients.

Question validation follows the TypeSafe OpenAPI wire schema (also used by SDK v0.5.7). `instructions` may be omitted or `null` for all question types. Noul `criteria` may be omitted or `null`; its `true`/`false` descriptions and Choice descriptions accept strings, objects, arrays, or `null`. Score `criteria` must be a non-empty array of string, object, or array descriptions; a single level is valid. SDK integer-keyed Score maps are normalized to arrays by the SDK before sending.

```bash
curl https://your-sub2api.example.com/v1/systemone \
  -H 'Authorization: Bearer sk-your-sub2api-key' \
  -H 'Content-Type: application/json' \
  --data '{"model":"jev-latest","state":"Text to evaluate","questions":{"safety":{"type":"noul","instructions":"Evaluate whether the text is unsafe"}}}'
```

The built-in `jev-latest` price is `$0.042` per million input tokens and `$0` for output tokens. Channel pricing can override both values. Credential, billing, permission, rate-limit, overload, server, and network failures (`401`, `402`, `403`, `429`, `529`, `5xx`, transport errors) use the existing account error policy (including custom error codes and temporary-unschedulable rules) and fail over to another account; request errors (`400`, `413`, and `422`) are returned without retrying another account and never change account state. TypeSafe groups (and Composite requests routed to TypeSafe) reject Messages, Chat Completions, Responses, and count_tokens requests with `404`.

### Antigravity Support

Sub2API supports [Antigravity](https://antigravity.so/) accounts. After authorization, dedicated endpoints are available for Claude and Gemini models.

#### Dedicated Endpoints

| Endpoint | Model |
|----------|-------|
| `/antigravity/v1/messages` | Claude models |
| `/antigravity/v1beta/` | Gemini models |

#### Claude Code Configuration

```bash
export ANTHROPIC_BASE_URL="http://localhost:8080/antigravity"
export ANTHROPIC_AUTH_TOKEN="sk-xxx"
```

#### Hybrid Scheduling Mode

Antigravity accounts support optional **hybrid scheduling**. When enabled, the general endpoints `/v1/messages` and `/v1beta/` will also route requests to Antigravity accounts.

> **⚠️ Warning**: Anthropic Claude and Antigravity Claude **cannot be mixed within the same conversation context**. Use groups to isolate them properly.

</details>

## Development

The current backend requires **Go 1.27.3** (`backend/go.mod`). CI uses **Node.js 24 and pnpm 9**; use those versions and the committed lockfile. The frontend uses Vue 3.5, TypeScript, Vite 5, Pinia and Tailwind CSS; the backend uses Gin, Ent, SQL repositories, PostgreSQL and Redis. Compose currently ships PostgreSQL 18 and Redis 8.

### Build a binary with the Web UI

With PostgreSQL and Redis running and the required toolchains installed:

```bash
git clone https://github.com/DevKuAI/devku-sub2api.git sub2api
cd sub2api
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend run build
cd backend
VERSION="$(./scripts/resolve-version.sh)"
go build -tags embed -ldflags="-X main.Version=${VERSION}" -o sub2api ./cmd/server
./sub2api
```

Vite writes the frontend to `backend/internal/web/dist/`; `-tags embed` bundles it into the server. Without that tag, the binary does not serve the Web UI. On a fresh first run, complete the setup wizard at `http://localhost:8080`; let it create `config.yaml` and restart the process after setup. Use the example YAML to review settings, rather than copying it before initialization.

### Local development

Install dependencies first, then run these commands in separate terminals from the repository root:

```bash
# Backend: initialize through CLI setup first on a fresh environment.
cd backend
go run ./cmd/server --setup
go run ./cmd/server
```

```bash
# Frontend: Vite serves on port 3000 and proxies to localhost:8080.
pnpm --dir frontend run dev
```

Set `VITE_DEV_PROXY_TARGET` if the backend runs elsewhere. `go run` does not provide automatic backend hot reload. After changing Ent schemas or Wire providers, run `make -C backend generate` and review the generated diff.

### Validation

```bash
make -C backend test-unit
make -C backend test-integration
(cd backend && golangci-lint run ./...)
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run test:run
pnpm --dir frontend run build
make -C backend build
git diff --check
```

Integration tests require Docker and live test containers; skipped integration tests are not a pass. `make test-frontend` runs lint, typecheck and a selected critical Vitest set; `test:run` above runs the full frontend suite. `make -C backend build` is a backend-only build; use the explicit `-tags embed` command for a binary with the UI. See the [development guide](docs/development/DEV_GUIDE.md) and [CI workflow](.github/workflows/backend-ci.yml).

## Documentation

| Topic | Entry |
| --- | --- |
| Documentation index | [docs/README.md](docs/README.md) |
| Deployment and security | [Deployment](deploy/README.md), [edge security](deploy/EDGE_SECURITY.md), [Apple container](deploy/APPLE_CONTAINER.md) |
| Development | [Developer guide](docs/development/DEV_GUIDE.md) |
| Repository organization | [Directory layout](docs/development/REPOSITORY_LAYOUT.md), [Agent skills](.agents/skills/) |
| Desktop APIs | [API v1](docs/desktop/DESKTOP_API_V1.md), [OpenAPI](docs/desktop/openapi/DESKTOP_API_V1.openapi.yaml), [sessions/conversations v2](docs/desktop/DESKTOP_SESSION_CONVERSATION_V2.md), [daily reports](docs/desktop/DESKTOP_DAILY_REPORTS.md) |
| Desktop resources | [MCP/Skill distribution](docs/desktop/desktop-resources-v1.md) |
| Accounting | [Design, implementation/configuration and validation](docs/accounting-management/README.md) |
| BI | [API contracts, configuration, integration and acceptance](docs/mobile-bi/README.md) |
| Payments and routing | [Payments](docs/payments/PAYMENT.md), [composite groups](docs/gateway/COMPOSITE_GROUPS.md) |
| Plugins | [Development guide](docs/development/PLUGIN_DEVELOPMENT.md), [Plugin API](backend/pkg/pluginapi/README.md) |

```text
sub2api/
├── backend/    # Go service, migrations, embedded Web UI and server tools
├── frontend/   # Vue application and tests
├── deploy/     # Installation scripts, Compose templates and operations guides
├── docs/       # Development guides, feature designs, API contracts and validation
├── assets/     # README images
├── tools/      # Repository maintenance utilities
├── .github/    # CI and release workflows
└── .agents/    # Project Agent skills
    └── skills/
```

## License

This project is licensed under the [GNU Lesser General Public License v3.0](LICENSE) (or later).

Copyright (c) 2026 DevKu.ai
