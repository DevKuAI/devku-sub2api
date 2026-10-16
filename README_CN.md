<div align="center">

<img src="assets/logo.svg" alt="Sub2API Logo" width="128" />

# Sub2API · DevKu

[![Go](https://img.shields.io/badge/Go-1.27.2-00ADD8.svg)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3.5-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D.svg)](https://redis.io/)

**AI API 网关、Desktop 企业管理、财务账目与 BI**

[English](README.md) | 简体中文

[项目仓库](https://github.com/DevKuAI/devku-sub2api) · [发布版本](https://github.com/DevKuAI/devku-sub2api/releases) · [上游仓库](https://github.com/Wei-Shaw/sub2api)

</div>

## 重要提醒

部署或使用 Sub2API 前，请确认以下事项：

- **上游服务条款**：使用 Sub2API 可能违反 Anthropic 等上游服务商的服务条款。使用前应阅读相关服务商的用户协议，并自行承担由此产生的风险。
- **合法合规**：仅在符合所在国家或地区法律法规的前提下使用 Sub2API，禁止用于违法违规活动。
- **免责声明**：本项目仅供技术学习与研究。作者不对因使用本项目导致的账户封禁、服务中断、数据丢失或其他直接、间接损失承担责任。
- **未授权商业运营**：开发者未授权任何个人或组织以本项目名义或基于本项目开展商业运营。相关纠纷、损失和法律责任由运营方自行承担。

## 项目概述

Sub2API 是可自行部署的 AI API 网关，用于管理上游 OAuth 和 API Key 账号、路由请求、分配配额，以及记录用量和成本。本仓库是 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) 的 DevKu 分支，扩展了 Desktop 企业管理、账户侧订阅查看、企业账目管理和 BI 运维功能。

从本文选择部署方式并完成服务配置。功能说明和 API 契约见 [文档索引](docs/README.md)。下列部署命令使用 DevKu 的发布包和镜像；从 `main` 安装的代码可能包含尚未发布的改动。

[功能](#功能) · [部署](#部署) · [配置](#配置) · [Provider 与兼容性](#provider-与兼容性) · [开发](#开发) · [文档入口](#文档入口) · [许可证](#许可证)

## 功能

| 模块 | 能力 |
| --- | --- |
| Gateway | Messages、Responses、Chat Completions、Gemini 及 Provider 专用协议；请求路由、粘性会话、负载均衡和复合分组。协议支持范围取决于所选 Provider。 |
| 账号与 API Key | OAuth/API Key 上游账号、用户 API Key 创建数量限制、模型 allowlist、账户绑定，以及订阅账号状态和配额的只读查看。 |
| 用量与限制 | Token 和成本统计、用户级/账号级并发、请求/Token 速率限制、API Key 消费窗口、输出 TPS 和运维看板。输出 TPS 与系统吞吐量分开统计；长上下文计费标签展示实际采用的价格档位。 |
| 在线支付 | EasyPay 易支付、支付宝、微信支付和 Stripe，支持用户自助充值；见 [支付配置](docs/payments/PAYMENT_CN.md)。 |
| Desktop 企业 | 企业与成员、企业挂载 Web 账户、成员 Model Token、模型配置、用量统计、会话/对话上报和日报。对话上报由企业级开关控制。 |
| Desktop 资源与更新 | MCP/Skill 包管理与鉴权分发，以及基于 S3 兼容存储的 Desktop 更新发布。私有资源与公开更新文件使用独立 bucket。 |
| 企业账目 | 到账与税务、VCC 分配/消费/退款、合同、发票、AI Invoice、支出与分次付款、对账、CSV 导出、私有附件和审计。不同币种分别记账。 |
| 企业 BI | 独立微信身份、企业邀请与功能授权，读取 Desktop 用量及日／周／月总结，提供管理端 metadata 运维。微信设备和生产容量验收仍按 [BI 文档](docs/mobile-bi/README.md) 跟踪。 |
| 管理与扩展 | Web 管理后台、运行监控、插件宿主服务，以及通过 iframe 接入外部系统。 |

### 企业账户入口

- Desktop 企业通过 `gateway_user_id` 绑定现有 Web 账户；绑定账户可在 `/desktop/organization` 查看本企业。
- 管理员在 `/admin/accounting` 管理所有企业的账目。绑定企业的 Web 账户显示「财务账目」，进入 `/organization/accounting`；没有账目时仍显示入口和空状态。绑定企业的 Admin 在「我的账户」下查看。
- 企业页面只读，仅展示本企业的到账、卡片消费及明确共享的合同/发票，不公开内部支出、利润或敏感卡片信息。Desktop 成员会话不能访问账目。
- 财务账目独立于在线支付、Gateway 余额和 API 用量计费；录入到账不会给 Gateway 账户充值。

## 部署

| 方式 | 前置条件 | 初始化方式 |
| --- | --- | --- |
| **Docker Compose（推荐）** | Docker 20.10+、Compose v2+ | 包含 PostgreSQL 和 Redis，首次启动自动初始化。 |
| Linux 二进制 | Linux amd64/arm64、systemd、安装时需要 root；已运行的 PostgreSQL 15+ 和 Redis 7+ | Web setup 向导或 CLI `--setup`。 |
| Apple `container` | Apple silicon、macOS 26、`container` 1.1.0+ | 本地三服务环境；见 [操作说明与限制](deploy/APPLE_CONTAINER.md)。 |
| 源码编译 | Go 1.27.2、Node.js 20、pnpm 9、PostgreSQL 和 Redis | 见 [开发](#开发)。 |

### Docker Compose 快速开始

在新的部署目录中执行：

```bash
mkdir -p sub2api-deploy
cd sub2api-deploy
curl -fsSL https://raw.githubusercontent.com/DevKuAI/devku-sub2api/main/deploy/docker-deploy.sh -o docker-deploy.sh
bash docker-deploy.sh
docker compose up -d
docker compose ps
docker compose logs -f sub2api
```

准备脚本将本地目录版 Compose 下载为 `docker-compose.yml`，生成包含 PostgreSQL/JWT/Desktop/TOTP 密钥的 `.env`，并创建 `data/`、`postgres_data/` 和 `redis_data/`。脚本会显示生成的凭证；终端输出和 `.env` 均应妥善保管。

访问 `http://YOUR_SERVER_IP:8080`。容器部署会自动初始化；`ADMIN_EMAIL` 或 `ADMIN_PASSWORD` 留空时，可从首次启动日志中取得生成的登录凭证：

```bash
docker compose logs sub2api | grep "Generated admin"
```

生产环境应先配置 HTTPS 和反向代理，再开放服务。完成管理员初始化后，添加上游账号和分组，再创建绑定目标分组的用户 API Key。API Key 页面的「使用密钥」提供受支持客户端的配置。

<details>
<summary>手动部署 Compose</summary>

```bash
git clone https://github.com/DevKuAI/devku-sub2api.git sub2api
cd sub2api/deploy
cp .env.example .env
chmod 600 .env
```

启动前编辑 `.env`。分别使用 `openssl rand -hex 32` 生成 `POSTGRES_PASSWORD`、`JWT_SECRET` 和 `TOTP_ENCRYPTION_KEY`，重启和恢复时保留 JWT 与加密密钥。按需填写 `ADMIN_EMAIL`、`ADMIN_PASSWORD` 和 `SERVER_PORT`（默认 `8080`），避免使用容易猜测的管理员凭证。

```bash
mkdir -p data postgres_data redis_data
docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml ps
docker compose -f docker-compose.local.yml logs -f sub2api
```

仓库中的 `docker-compose.local.yml` 使用本地目录，`docker-compose.yml` 使用 named volumes。后续操作保持相同的 `-f` 文件选择。准备脚本生成的 `docker-compose.yml` 已采用本地目录方式。

</details>

### Linux 二进制安装

安装脚本将 DevKu 发布包安装到 `/opt/sub2api`，并配置系统用户和 systemd unit：

```bash
curl -fsSL https://raw.githubusercontent.com/DevKuAI/devku-sub2api/main/deploy/install.sh -o install.sh
sudo bash install.sh
sudo systemctl start sub2api
sudo systemctl enable sub2api
sudo journalctl -u sub2api -f
```

全新安装时，访问 `http://YOUR_SERVER_IP:8080`，完成数据库、Redis 和管理员配置。不要预先创建 `config.yaml`，否则会跳过向导。使用 CLI 初始化时，应在服务用户和服务数据目录的上下文中运行已安装的二进制并传入 `--setup`。历史 YAML 字段 `default.admin_email` / `default.admin_password` 不负责初始化管理员。

### 升级与备份

Compose 部署应先备份数据库和持久化数据，再重新创建容器：

```bash
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 sub2api
```

手动部署本地目录版时，命令应带上 `-f docker-compose.local.yml`。需要可复现部署时，固定具体的 DevKu 镜像 tag。二进制安装可使用后台「检查更新」及其支持的回滚流程；Compose 升级应更新镜像。

备份范围包括 PostgreSQL、应用数据/配置、`.env` 和加密密钥；保留会话及媒体任务绑定依赖的 Redis 状态。不要直接复制运行中的数据库目录：使用数据库备份工具，或停服后归档本地目录。私有 S3 文件需要独立的备份和保留策略。迁移及生命周期命令见 [部署操作说明](deploy/README.md)。

## 配置

容器配置参考 [deploy/.env.example](deploy/.env.example)，二进制配置参考 [deploy/config.example.yaml](deploy/config.example.yaml)。非空环境变量覆盖 YAML；Compose 模板还可能提供默认值。修改 `.env` 后执行 `docker compose up -d` 重新创建容器，仅重启不会载入新的环境变量。

| 配置项 | 用途 |
| --- | --- |
| `JWT_SECRET` | 固定 Web 会话签名密钥，使用 `openssl rand -hex 32` 生成。 |
| `TOTP_ENCRYPTION_KEY` | 2FA 和敏感 VCC 字段的固定加密密钥，使用 `openssl rand -hex 32` 生成，恢复时必须保留。 |
| `DESKTOP_ENABLED` | 默认 `false`。启用前配置独立的 `DESKTOP_JWT_SECRET`（`openssl rand -base64 32`）和外部可访问、以 `/v1` 结尾的 HTTPS `DESKTOP_PUBLIC_GATEWAY_BASE_URL`。控制 Desktop 及企业账户账目访问。 |
| `DESKTOP_UPDATE_STORAGE_RESOURCE_BUCKET` | MCP/Skill 和账目附件共用的私有 bucket，复用 `DESKTOP_UPDATE_STORAGE_*` S3 连接。账目文件位于 `accounting/{organization_id}/{attachment_id}/`，无需独立账目 bucket。 |
| `BI_ENABLED` | 默认 `false`。需配置真实 `BI_APPID`/`BI_APP_SECRET`、独立的 base64 `BI_JWT_SECRET`/`BI_IDENTITY_SECRET`，并发布带独立 HTTPS 站点地址的 BI 隐私说明。详见 [BI 配置](docs/mobile-bi/configuration.md)。 |

未配置 S3 时，基础账目记录仍可使用；上传及依赖凭证的确认返回 `ACCOUNTING_STORAGE_UNAVAILABLE`。敏感卡片信息录入要求固定加密密钥，读取接口仅返回卡号后四位。记录与附件的共享开关独立，默认不共享。详见 [账目配置与 API 规则](docs/accounting-management/implementation.md)。

### 反向代理与安全

- 根据部署环境配置 HTTPS、CORS origin、CSP 和上游 URL 校验。关闭 URL allowlist 时，默认允许 HTTP 上游地址；生产环境应显式设置 `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=false`，仅允许 HTTPS。HTTP 会使密钥和数据面临窃听及 MITM 风险。
- `security.trust_forwarded_ip_for_api_key_acl` 为兼容升级默认 `true`。此模式下应限制源站仅接受 CDN/代理访问，并由边缘代理覆盖可信请求头。需要使用 Gin 的信任边界时，关闭该配置，将 `server.trusted_proxies` 设为直接连接服务的准确代理 CIDR。
- `SECURITY_FORWARDED_CLIENT_IP_HEADERS=True-Client-IP,X-CDN-Client-IP` 最多配置 16 个经过校验、标准化和去重的请求头名称；后台安全设置支持无需重启更新。关闭兼容接管模式后，忽略自定义和内置原始转发头。详见 [边缘安全与迁移说明](deploy/EDGE_SECURITY.md)。
- 关闭 URL 校验或响应头过滤时，应在网络层限制上游出口、阻止 private/loopback/link-local 目标、强制 TLS，并在代理层移除敏感响应头。生产环境还应核对 `billing.circuit_breaker` 和 `turnstile.required`。

Nginx 默认丢弃 `session_id` 等包含下划线的请求头，会影响 Codex 粘性路由。在 `http` block 中启用：

```nginx
underscores_in_headers on;
```

<details>
<summary>Codex Fast/Flex、WebSocket 与 h2c 配置</summary>

### Codex Fast/Flex 策略

后台 Gateway 设置中的 OpenAI Fast/Flex 策略控制请求体 `service_tier`，不会改变 Codex 客户端模型目录，也不会自动显示 Speed 或 `/fast`。

- `pass`：保留客户端 `service_tier`，将 `fast` 标准化为上游使用的 `priority`。
- `filter`：移除 `service_tier`，按普通优先级请求。
- `block`：拒绝匹配的 Fast/Flex 请求。
- `force_priority`：将匹配请求设为 `priority`，按 Priority/Fast 价格计费。为保留既有规则语义，`all` 只匹配显式提供 tier 的请求。要升级未传 `service_tier` 的 OpenAI 请求，添加 `service_tier=missing + force_priority` 规则；其他平台不注入缺省 tier。即使 Codex 未显示 Fast 状态，请求也可能已使用 Fast。

Codex 的 Fast 控件依赖客户端模型目录声明 `additional_speed_tiers: ["fast"]` 及对应 `service_tiers`。通过 API Key 或自定义 Provider 连接时，若目录缺少这些字段，设置 `force_priority` 或重启 Codex 不会使 Speed 选项出现。

可在 `~/.codex/config.toml` 设置默认请求 tier：

```toml
service_tier = "fast"

[features]
fast_mode = true
```

`service_tier = "fast"` 会在请求中携带 Fast 设置；`features.fast_mode` 只启用客户端功能。模型目录缺少 Fast 能力时，`/fast` 仍可能不可用。应在 Sub2API 用量记录中确认最终 `service_tier` 是否为 `priority`。

### OpenAI Responses WebSocket 限制与 HTTP 回退

`gateway.openai_ws` 限制客户端 Responses WebSocket 的连接时长和总数量。这些限制独立于每轮请求的用户/账号并发槽位；并发槽位会在轮次之间释放。

```yaml
gateway:
  openai_ws:
    # Total time to receive and decompress the first client message.
    client_first_message_timeout_seconds: 30
    # Close a socket idle between completed turns; 0 disables the limit.
    ingress_inter_turn_idle_timeout_seconds: 300
    # Distributed live-session limit per API key; 0 disables the limit.
    max_ingress_connections_per_api_key: 64
```

首条消息超时是读取与解压的总期限。大上下文、图片较多或网络较慢时，可将其提高到 120–300 秒。该限制在 HTTP bridge 路由前生效，bridge 模式不能绕过。

连接上限通过 Redis 协调，租约为 60 秒，每 20 秒刷新。进程在完整租约周期内无法确认租约时，会关闭本地 WebSocket，避免继续使用未计入全局上限的连接。

选择账号级 `http_bridge` 等 WS 模式前，先启用 v2 mode router：

```yaml
gateway:
  openai_ws:
    mode_router_v2_enabled: true
```

也可设置 `GATEWAY_OPENAI_WS_MODE_ROUTER_V2_ENABLED=true`。`http_bridge` 保留客户端 WebSocket，使用上游 HTTP/SSE，适合逐步启用或缓解上游 WebSocket 问题。

#### 强制 OpenAI 上游 HTTP/SSE

出口代理或网络导致 OpenAI Responses WebSocket 反复重连时，可在持久化部署配置中启用全局回退：

```yaml
gateway:
  openai_ws:
    force_http: true
```

Compose 和 Apple `container` 可在 `.env` 中设置：

```bash
GATEWAY_OPENAI_WS_FORCE_HTTP=true
```

该配置将原本使用 WebSocket 的 OpenAI 上游 Responses 流量改为 HTTP/SSE，不改变客户端协议，也不强制 HTTP/1.1。代理不兼容 HTTP/2 时，另行配置 `gateway.openai_http2.enabled` 或 `GATEWAY_OPENAI_HTTP2_ENABLED=false`。

全局 `force_http` 无需启用 `mode_router_v2_enabled`。将设置保存在部署的 `.env` 或 `config.yaml` 中，避免只修改运行中容器而在更新后丢失。

### HTTP/2（h2c）与 HTTP/1.1 回退

后端明文端口默认支持 h2c，并保留 HTTP/1.1 回退用于 WebSocket 与旧客户端。浏览器通常不支持 h2c，性能收益主要在反向代理或内网链路。

**反向代理示例（Caddy）：**

```caddyfile
transport http {
	versions h2c h1
}
```

**验证：**

```bash
# h2c prior knowledge.
curl --http2-prior-knowledge -I http://localhost:8080/health
# HTTP/1.1 fallback.
curl --http1.1 -I http://localhost:8080/health
# WebSocket fallback requires an administrator token.
websocat -H="Sec-WebSocket-Protocol: sub2api-admin, jwt.<ADMIN_TOKEN>" ws://localhost:8080/api/v1/admin/ops/ws/qps
```

</details>

### 简易模式

简易模式适合个人开发者或内部团队快速使用，不依赖完整 SaaS 功能。

- 启用方式：设置环境变量 `RUN_MODE=simple`
- 默认每次启动会补齐默认分组。设置 `SIMPLE_MODE_AUTO_CREATE_DEFAULT_GROUPS=false`（或 YAML `simple_mode.auto_create_default_groups: false`）可自行管理分组。默认值为 `true`；关闭后不删除已有分组，不改变运行时自动绑定或管理员并发设置。
- 功能差异：隐藏 SaaS 相关功能，跳过计费流程
- 可选密钥窗口：设置 `SIMPLE_MODE_KEY_RATE_LIMIT_ENABLED=true` 后，按每个 API Key 配置的 5 小时、1 天、7 天消费窗口进行限制，默认值为 `false`；启用后仍跳过余额和订阅扣费。
- 窗口限制以数据库为准，只记录 API Key 窗口用量。它在请求完成后记账，并发中的请求可能以各自最终费用超过窗口上限。启用前的历史用量不会自动补算。
- 安全注意事项：生产环境需同时设置 `SIMPLE_MODE_CONFIRM=true` 才允许启动

## Provider 与兼容性

### v0.2.17 兼容性更新

- **OpenAI 订阅模型**：ChatGPT OAuth 和 SetupToken 账号不再调度已退役的 GPT-5.5 目标，模型列表也会过滤这些目标。API Key 账号仍可使用；指向受支持替代模型的显式映射继续生效。升级时核对客户端默认模型及 compact 专用映射，系统不会自动改写配置。见 [迁移说明](docs/OPENAI_GPT55_RETIREMENT.md)。
- **Gemini 与 Antigravity**：新增 Gemini 4 Argon 模型目录和计费，兼容入口转发思考档位并流式返回思考摘要，保留工具图片和 thought signature，修复 Vertex 服务账号失效及共享容量重试处理。
- **网关兼容性**：Responses 对象补齐 `created_at`；`gateway.min_output_tokens` 控制协议转换后的输出下限，默认 `128`、最小 `16`。Codex 导入按 workspace 区分账号，Anthropic 转发遵循协议 base URL，OpenAI API Key 账号可在 `/v1/messages` 使用 web-search emulation。
- **支付与配额**：EasyPay 通知核验后才入账；退款结果待定时继续保留扣减。Stripe 恢复支付使用本地 client-secret 快照，并修正 GLM Coding Plan 用量窗口。

### v0.2.16 兼容性更新

- **Gemini 显式上下文缓存**：支持 `/v1beta/cachedContents` 的创建、列表、查询、延长和删除，生成请求通过 `cachedContent` 引用缓存。支持 AI Studio、Agent Platform (Vertex) 和启用显式缓存的自建网关上游；`gateway.gemini_cached_content_max_ttl_seconds` 默认值为 `86400`，存储费在创建或延长时收取。
- **Gemini 账号**：Agent Platform (Vertex) 支持 API Key 认证和上游模型同步；第三方网关 API Key 与 Vertex 账号不再套用 AI Studio 等级和模拟配额。
- **模型兼容性**：新增 `claude-haiku-5-5` 支持；GPT-6.1 Sol 的生成上下文默认值调整为 500k，订阅账号不再收取长上下文附加费。已有模型映射和白名单仍需由管理员核对。
- **分组多语言**：管理端可维护分组名称和描述的译文；用户侧接口按 `Accept-Language` 返回对应文本，未命中时使用原文。

### 路由、视频与插件兼容性

- **Command Code 与 Cline**：均支持 API Key 账号。Command Code 根据模型能力选择原生 Messages、Responses 或 Chat Completions 端点；Cline 使用 Chat Completions，并转换入站 Messages/Responses。Cline 积分与 ClinePass 限额分别作用于对应模型。
- **Platform catalog**：账号创建、路由和默认平台配额共用目录。迁移已移除数据库 platform CHECK 约束，由应用校验已注册平台。
- **模型 allowlist**：同时限制模型列表和请求准入。`groups.models_list_config` 迁移为 `model_allowlist` 时保留数据；升级前核对已有列表，因为启用的列表现在也会限制请求。
- **Seedance（Ark）**：支持原生任务创建、查询和删除；要求 OpenAI API Key 账号开启 Seedance 能力，分组开启媒体权限。查询首次观察到 `succeeded` 时按 `usage.completion_tokens` 计费，无后台轮询或回调自动结算。应保留 Redis 任务绑定，并在生命周期内查询已完成任务。见 [Seedance 配置与计费](docs/media/seedance-api.md)。
- **插件宿主服务**：Redis KV 按插件身份隔离，账号目录受声明能力限制，`plugin.status` 只读。宿主协商为可选功能，兼容旧插件。见 [Plugin API](backend/pkg/pluginapi/README.md) 和 [UI bridge](backend/pkg/pluginapi/docs/ui-bridge.md)。
- **Antigravity attribution**：仅在 Anthropic Messages 转 Antigravity 时移除开头的 Claude attribution metadata。原生 Anthropic 转发保持不变，不应全局关闭 attribution。

<details>
<summary>图片模型、Grok / xAI、TypeSafe / Jev 与 Antigravity 说明</summary>

### OpenAI 图片模型

支持 `gpt-image-2.5-flare`、`gpt-image-2.5-sunburst` 及其 `2026-09-08` 日期快照，可通过 `/v1/images/generations`、`/v1/images/edits` 调用。`quality` 支持 `xhigh`、`max`、`auto`，合法自定义尺寸和图片 usage 明细保持透传。

OAuth / Setup Token 图片请求使用 Responses 主控模型调用 `image_generation` 工具，默认主控为 `gpt-5.6-luna`。可设置 `SUB2API_IMAGES_MAIN_MODEL` 切换为账号支持的文本模型；Docker Compose 用户修改 `.env` 后执行 `docker compose up -d` 重建容器。该配置不会替换所选图片模型，也不会覆盖 `/v1/responses` 请求中已经提供的文本主控模型。

升级后，无模型限制的账号自动支持新模型。已有显式账号映射或分组白名单需要加入两个 2.5 模型（日期快照按需加入）；升级不会自动扩大管理员设置的模型权限。新模型内置价格包含官方文本输入、图片输入和图片输出 token 费率，远端价格表尚未更新时使用内置 2.5 价格；实际按次或按 token 计费仍由既有分组/渠道配置决定。

### 异步图片任务

OpenAI/Grok 长时间运行的图片生成与编辑可提交到 `/v1/images/generations/async` 或 `/v1/images/edits/async`，再通过 `/v1/images/tasks/{task_id}` 轮询，无需持续占用 CDN 连接。请求响应示例见 [异步图片任务](docs/media/ASYNC_IMAGE_TASKS.md)。

### Grok / xAI

支持 xAI OAuth 订阅账号和标准 xAI API Key 账号，两类账号均可转发 OpenAI 兼容的 Responses 请求。

#### 接口范围

- 平台名称：`grok`；账号类型：OAuth 订阅账号和 API Key 账号。
- Responses：`/v1/responses`、`/responses` 和 `/backend-api/codex/responses`。OAuth 转发到订阅代理，API Key 转发到 `https://api.x.ai/v1/responses`。
- Claude 兼容入口：`/v1/messages`，转换为 xAI Responses，并以 Anthropic Messages 格式返回，适用于 Claude CLI 类客户端。
- Chat Completions：`/v1/chat/completions` 和 `/chat/completions`，根据账号类型转发到对应 xAI 上游。
- 接受 Codex CLI 风格的 Responses WebSocket 入站请求，并桥接到 xAI HTTP/SSE Responses 上游。
- 文本模型：`grok-4.5`、`grok-4.3`、`grok-build-0.1`、`grok-composer-2.5-fast`、`grok-4.20-0309-reasoning`、`grok-4.20-0309-non-reasoning` 和 `grok-4.20-multi-agent-0309`。
- Grok 分组媒体接口：`/v1/images/generations`、`/images/generations`、`/v1/images/edits`、`/images/edits`、`/v1/videos/generations`、`/videos/generations`、`/v1/videos/edits`、`/videos/edits`、`/v1/videos/extensions`、`/videos/extensions`、`/v1/videos/{request_id}` 和 `/videos/{request_id}`。生成、编辑和扩展操作要求分组开启图片生成权限。
- 媒体模型：`grok-imagine`、`grok-imagine-image-quality`、`grok-imagine-image`、`grok-imagine-image-2.0`、`grok-imagine-edit`、`grok-imagine-video` 和 `grok-imagine-video-1.5`。
- JSON 图片编辑与视频生成中的 `image`、`images`、`reference_images` 和 `mask` 对象使用 `url` 引用图片；兼容旧 `image_url` 字段，转发前会标准化为 `url`。
- 不包含 TTS、转录、浏览器自动化、cookie 或 Grok 网页抓取。

#### OAuth 配置

Grok OAuth 使用 PKCE，无需向仓库提交私有密钥。默认值遵循兼容客户端使用的公开 xAI OAuth 流程，均支持通过环境变量覆盖：

| 变量 | 默认值 |
| --- | --- |
| `XAI_OAUTH_CLIENT_ID` | 公开的 xAI OAuth client ID |
| `XAI_OAUTH_SCOPE` | `openid profile email offline_access grok-cli:access api:access` |
| `XAI_OAUTH_REDIRECT_URI` | `http://127.0.0.1:56121/callback` |
| `XAI_OAUTH_AUTHORIZE_URL` | `https://auth.x.ai/oauth2/authorize` |
| `XAI_OAUTH_TOKEN_URL` | `https://auth.x.ai/oauth2/token` |
| `XAI_BASE_URL` | `https://api.x.ai/v1`；用于运行诊断，请求转发由账号 `base_url` 控制。 |
| `XAI_GROK_CLI_VERSION` | `0.2.114`；可覆盖发往 `cli-chat-proxy.grok.com` 的客户端标识，但低于此固定版本的覆盖值会被忽略。 |

管理员可从后台创建 OAuth 或 API Key 账号，也可通过 Admin API 授权或重新授权：

| 接口 | 用途 |
| --- | --- |
| `POST /api/v1/admin/grok/oauth/auth-url` | 生成 xAI OAuth 授权 URL。 |
| `POST /api/v1/admin/grok/oauth/exchange-code` | 通过回调 URL、查询串或授权码换取凭证。 |
| `POST /api/v1/admin/grok/oauth/refresh-token` | 校验或刷新 Grok refresh token。 |
| `POST /api/v1/admin/grok/accounts/:id/refresh` | 刷新已有 Grok 账号。 |

OAuth 凭证复用已有账号 JSON 字段：`access_token`、`refresh_token`、`token_type`、`expires_at`、`base_url`，以及可选的 `email`、`subscription_tier` 和 `entitlement_status`。OAuth 推理默认访问 `https://cli-chat-proxy.grok.com/v1`；存储了旧 `https://api.x.ai/v1` 默认地址的 OAuth 账号会在运行时转向订阅代理，显式自定义地址保持不变。

API Key 账号在创建弹窗中选择「Grok → API Key」，默认 base URL 为 `https://api.x.ai/v1`，凭证使用已有 `base_url` 和 `api_key` 字段。

#### Grok Build CLI 配置

1. 在后台添加 `grok` OAuth 账号并完成 xAI 授权，或添加 Grok API Key 账号。
2. 创建 Grok 分组、关联上游账号，再创建绑定该分组的 Sub2API API Key。
3. 在 API Key 页面点击「使用密钥」，选择「Grok CLI」。弹窗提供 macOS/Linux 或 Windows 的文件路径与 base URL；「OpenCode」Tab 也提供对应配置。
4. 手动配置时，编辑 `~/.grok/config.toml`（Windows 为 `%USERPROFILE%\.grok\config.toml`）：

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

合并配置前备份已有文件。文件含 Sub2API API Key，应妥善保管并在系统支持时限制访问权限。检查生效配置并发起 smoke request：

```bash
grok inspect
grok -p "Reply with sub2api-ok" -m grok
```

这里的 `base_url` 是以 `/v1` 结尾的 Sub2API 公开地址，不是 `api.x.ai` 或内部 OAuth 代理。

#### 配额、状态与媒体资格

xAI 配额采用被动观测，不推测订阅额度。上游提供相关请求头时，从成功或限流响应中记录 allowlist 内的 rate-limit headers。首次有效响应前配额显示为未知，仍展示本地 Sub2API 用量。

`401` 会使无效凭证账号暂时退出调度；`403` 按访问权限或 entitlement 问题处理，不进入 token 刷新循环；`429` 使用 `Retry-After` 或短暂 cooldown 暂停调度。

新建图片和视频请求使用媒体资格检查。API Key 账号可参与调度；OAuth 账号存在明确 Free 或禁止计费证据时会被排除。缺失或格式异常的观测先探测再调度；成功但不完整的计费响应标记为 `billing_inconclusive`，为兼容既有行为仍允许调度，因为未知 schema 不代表没有权限。导入账号时会主动执行计费优先的配额探测。聊天和视频状态查询不受此媒体隔离规则影响。没有可用媒体账号时返回 HTTP `503`，错误类型为 `grok_media_no_eligible_account`。

管理员可通过账号创建/更新 API 设置 `extra.grok_media_eligible=false` 排除账号，或设为 `true` 强制允许。更新时设为 `null` 可恢复自动探测；省略字段保留原覆盖值。只有周额度周期不能作为付费档位证据。图片成功响应必须包含实际图片输出，空 HTTP `200` 会触发账号 failover，不计为成功生成。

### TypeSafe / Jev 使用说明

Sub2API 支持使用 TypeSafe API Key 账户，通过 Jev 原生、非流式的 System One 协议调用模型。

- 平台：`typesafe`；账号类型：API Key
- 默认上游：`https://api.typesafe.ai`
- 对外端点：`POST /v1/systemone`
- 模型：`jev-latest`，TypeSafe 分组的 `/v1/models` 也会返回该模型
- 问题类型：`noul`、`choice`、`score`

请求和成功响应保持 System One 原生 JSON 结构。该端点不兼容 Chat Completions、Responses、Anthropic Messages 或流式客户端。

问题校验遵循 TypeSafe OpenAPI 的线上协议 schema（SDK v0.5.7 也使用该 schema）。所有问题的 `instructions` 都可以省略或为 `null`。Noul 的 `criteria` 可以省略或为 `null`，其中 `true`/`false` 的描述和 Choice 描述支持字符串、对象、数组或 `null`。Score 的 `criteria` 必须是至少包含一档描述的数组，每档支持字符串、对象或数组；单档也合法。SDK 的整数键 Score 映射会由 SDK 在发送前转换为数组。

```bash
curl https://your-sub2api.example.com/v1/systemone \
  -H 'Authorization: Bearer sk-your-sub2api-key' \
  -H 'Content-Type: application/json' \
  --data '{"model":"jev-latest","state":"待评估文本","questions":{"safety":{"type":"noul","instructions":"评估文本是否不安全"}}}'
```

`jev-latest` 内置价格为输入 `$0.042/百万 tokens`、输出 `$0`，渠道定价可以覆盖。凭据、欠费、权限、限流、过载、服务端和网络错误（`401`、`402`、`403`、`429`、`529`、`5xx`、传输错误）沿用现有账号错误策略（含自定义错误码与临时不可调度规则）并切换账号；请求错误（`400`、`413`、`422`）不会切换账号重试，也不会改变账号状态。TypeSafe 分组（以及路由到 TypeSafe 的 Composite 请求）调用 Messages、Chat Completions、Responses、count_tokens 时返回 `404`。

### Antigravity 使用说明

Sub2API 支持 [Antigravity](https://antigravity.so/) 账户，授权后可通过专用端点访问 Claude 和 Gemini 模型。

#### 专用端点

| 端点 | 模型 |
|------|------|
| `/antigravity/v1/messages` | Claude 模型 |
| `/antigravity/v1beta/` | Gemini 模型 |

#### Claude Code 配置示例

```bash
export ANTHROPIC_BASE_URL="http://localhost:8080/antigravity"
export ANTHROPIC_AUTH_TOKEN="sk-xxx"
```

#### 混合调度模式

Antigravity 账户支持可选的**混合调度**功能。开启后，通用端点 `/v1/messages` 和 `/v1beta/` 也会调度该账户。

> **⚠️ 注意**：Anthropic Claude 和 Antigravity Claude **不能在同一上下文中混合使用**，请通过分组功能做好隔离。

</details>

## 开发

当前后端要求 **Go 1.27.2**（`backend/go.mod`）。CI 使用 **Node.js 20 和 pnpm 9**，应使用对应版本及仓库 lockfile。前端采用 Vue 3.5、TypeScript、Vite 5、Pinia 和 Tailwind CSS；后端采用 Gin、Ent、SQL Repository、PostgreSQL 和 Redis。当前 Compose 使用 PostgreSQL 18 和 Redis 8。

### 编译包含 Web UI 的二进制

先启动 PostgreSQL 和 Redis，并安装所需 toolchain，再执行：

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

Vite 将前端输出到 `backend/internal/web/dist/`，`-tags embed` 将其嵌入服务二进制；不带该 tag 的二进制不提供 Web UI。全新环境首次启动时，访问 `http://localhost:8080` 完成 setup 向导，让向导生成 `config.yaml`，完成后重新启动进程。YAML 示例用于核对配置项，不要在初始化前复制为正式配置。

### 本地开发

先安装依赖，再从仓库根目录打开两个终端分别执行：

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

后端地址不同时设置 `VITE_DEV_PROXY_TARGET`。`go run` 不提供后端自动热重载。修改 Ent schema 或 Wire provider 后，执行 `make -C backend generate` 并检查生成文件 diff。

### 验证

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

集成测试需要 Docker 和实际启动的测试容器；跳过不能计为通过。`make test-frontend` 执行 lint、typecheck 和选定的关键 Vitest 测试，上面的 `test:run` 执行完整前端测试。`make -C backend build` 仅编译后端；包含 UI 的二进制使用前述显式 `-tags embed` 命令。详见 [开发指南](docs/development/DEV_GUIDE.md) 和 [CI workflow](.github/workflows/backend-ci.yml)。

## 文档入口

| 主题 | 入口 |
| --- | --- |
| 文档索引 | [docs/README.md](docs/README.md) |
| 部署与安全 | [部署说明](deploy/README.md)、[边缘安全](deploy/EDGE_SECURITY.md)、[Apple container](deploy/APPLE_CONTAINER.md) |
| 开发 | [开发指南](docs/development/DEV_GUIDE.md) |
| 仓库结构 | [目录说明](docs/development/REPOSITORY_LAYOUT.md)、[Agent skills](.agents/skills/) |
| Desktop API | [API v1](docs/desktop/DESKTOP_API_V1.md)、[OpenAPI](docs/desktop/openapi/DESKTOP_API_V1.openapi.yaml)、[会话/对话 v2](docs/desktop/DESKTOP_SESSION_CONVERSATION_V2.md)、[日报](docs/desktop/DESKTOP_DAILY_REPORTS.md) |
| Desktop 资源 | [MCP/Skill 分发](docs/desktop/desktop-resources-v1.md) |
| 账目管理 | [设计、实现/配置与验证](docs/accounting-management/README.md) |
| BI | [API 契约、配置、联调与验收](docs/mobile-bi/README.md) |
| 支付与路由 | [支付](docs/payments/PAYMENT_CN.md)、[复合分组](docs/gateway/COMPOSITE_GROUPS.md) |
| 插件 | [开发指南](docs/development/PLUGIN_DEVELOPMENT.md)、[Plugin API](backend/pkg/pluginapi/README.md) |

```text
sub2api/
├── backend/    # Go 服务、迁移、嵌入的 Web UI 与服务端工具
├── frontend/   # Vue 应用与测试
├── deploy/     # 安装脚本、Compose 模板与运维文档
├── docs/       # 开发指南、功能设计、API 契约与验证记录
├── assets/     # README 图片
├── tools/      # 仓库维护工具
├── .github/    # CI 与发布 workflow
└── .agents/    # 项目 Agent skills
    └── skills/
```

## 许可证

本项目采用 [GNU Lesser General Public License v3.0](LICENSE)（或更高版本）。

Copyright (c) 2026 DevKu.ai
