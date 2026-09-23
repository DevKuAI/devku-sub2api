# BI 配置参考

当前服务包含 7 个 `bi.*` 配置项。后端配置加载、启动校验、[YAML 示例](../../deploy/config.example.yaml)、[环境变量示例](../../deploy/.env.example) 以及 `docker-compose.yml`、`docker-compose.local.yml`、`docker-compose.standalone.yml`、`docker-compose.dev.yml` 均覆盖这些字段。独立进程可读取 YAML 或环境变量；Compose 中的 `.env` 变量通过模板传入应用容器。

非空环境变量优先于 YAML。Compose 为 `BI_ENABLED`、`BI_MIN_CLIENT_VERSION` 和 `BI_REPORT_RETENTION_MONTHS` 设置了显式默认值；使用 Compose 时在 `.env` 中修改这些值，避免挂载 YAML 中的同名设置被默认环境值覆盖。空字符串环境变量按当前 Viper 行为忽略，仍可回落到 YAML 或默认值。

## 服务配置

| YAML 键 | 环境变量 | 默认值 | 校验与用途 |
| --- | --- | --- | --- |
| `bi.enabled` | `BI_ENABLED` | `false` | 启用 BI 路由及服务内 worker |
| `bi.appid` | `BI_APPID` | 空 | 启用时必填；实际小程序 AppID |
| `bi.app_secret` | `BI_APP_SECRET` | 空 | 启用时必填；仅服务端访问 code2Session |
| `bi.jwt_secret` | `BI_JWT_SECRET` | 空 | 启用时必填；标准 base64 解码至少 32 字节；小程序 JWT 签名 |
| `bi.identity_secret` | `BI_IDENTITY_SECRET` | 空 | 启用时必填；标准 base64 解码至少 32 字节；微信身份 HMAC 索引及摘要 |
| `bi.min_client_version` | `BI_MIN_CLIENT_VERSION` | `0.1.0` | 启用时不能为空；随 bootstrap 返回，服务端不据此比较或阻断客户端版本 |
| `bi.report_retention_months` | `BI_REPORT_RETENTION_MONTHS` | `24` | 报告可读期限，1–120 个自然月；兼容值 0 按 24 个月处理 |

两份 BI 密钥必须彼此独立，并与原网页 JWT、Desktop JWT 密钥分离。`identity_secret` 应跨重启保持稳定，轮换需要身份索引迁移方案。生成示例为 `openssl rand -base64 32`，每份密钥分别生成；不在文档或前端保存实际值。

配置只校验格式与必要条件，真实 AppID/AppSecret 是否可用、域名是否合法仍需微信环境验证。当前没有原站设置页面编辑这些部署配置，需通过配置文件或部署环境维护。原站页面负责微信绑定、企业管理授权及隐私说明维护。

## 隐私说明（后台维护）

隐私说明由程序内页面 `/legal/bi-privacy` 展示，无需登录。管理员在「系统设置 → 登录条款 → BI 小程序隐私说明」维护独立的站点地址、标题、版本号和 Markdown 正文，点击「保存并发布隐私说明」后立即生效，无需重启。该按钮单独保存隐私说明，页面底部的系统设置保存按钮不提交正文。

| 项目 | 规则 |
| --- | --- |
| 站点地址 `site_url` | 必填，最多 2048 字节；填写承载本站隐私说明页面的 HTTPS origin（域名及可选端口），例如 `https://bi.example.com`。允许末尾 `/`，保存时去除；不能包含用户凭证、业务路径、查询参数或 fragment |
| 标题 | 必填，最多 80 个字符 |
| 版本号 | 必填，最多 64 个字符，不含控制字符；修改标题或正文时必须更换版本号 |
| Markdown 正文 | 必填，最多 200 KiB；程序过滤渲染结果中的不安全 HTML |
| 更新时间 | 服务端自动生成 UTC RFC3339 时间 |
| `privacy_notice_url` | 仅使用 BI 隐私说明中保存的 `site_url`，固定追加 `/legal/bi-privacy`；不读取或回退到 `frontend_url`、`server.frontend_url` |
| `privacy_notice_version` | bootstrap 返回当前已发布文档的版本号，与公开页面一致 |

首次发布前，公开文档接口返回 404。隐私说明未发布、站点地址不符合上述规则或设置读取失败时，bootstrap 返回 `503 DATA_UNAVAILABLE`。发布时须同时提交有效的站点地址和正文；非法地址返回 `400 INVALID_BI_PRIVACY_SITE_URL`，保留原配置。本地可直接预览相对路径页面。程序不会根据请求的 Host 或转发头拼接地址。

隐私说明单独存入 `settings.bi_privacy_notice`，站点地址、标题、正文、版本和更新时间通过一次写入保存；不影响原站前端地址、登录条款及其确认版本。仅修改站点地址时保留文档版本和更新时间，新地址立即生效。公开文档和 bootstrap 每次读取当前记录，响应禁用缓存。当前仅维护最新版本，不提供历史版本或隐私同意记录。

**从旧版迁移：** `bi.privacy_notice_url`、`bi.privacy_notice_version` 及对应的 `BI_PRIVACY_NOTICE_URL`、`BI_PRIVACY_NOTICE_VERSION` 已由后台设置替代，不再读取。升级前准备原隐私正文，升级后在 BI 隐私说明中填写独立的 HTTPS 站点地址并发布。已有文档记录如果缺少 `site_url`，须在此补填；程序不会自动复制原站前端地址。旧外部 URL 不会被自动抓取或沿用。运营主体、联系方式和实际数据政策需由运营方填写，程序不预置正式法律条款。

接口及请求示例见 [隐私说明 OpenAPI](./privacy-notice.openapi.yaml)。这些接口沿用原站 `/api/v1` envelope 和管理员认证，不改变小程序 API v1.1.0 的响应字段。

## 采集工具配置

| 名称 | 范围与用途 |
| --- | --- |
| `BI_DATABASE_DSN` | `bi-connector` CLI 的 PostgreSQL 连接配置；不是服务端 `bi.*` 配置，也不自动传入应用容器 |
| `BI_CONNECTOR_TOKEN` | CLI 执行 `sync-sub2api` 时使用的接入凭证；由运维签发并绑定组织、source、namespace 及记录类型 |
| `-organization`、`-source`、`-namespace`、`-kinds`、`-ttl` | `issue` 操作参数；凭证默认有效 24 小时 |
| `-since`、`-limit` | `sync-sub2api` 操作参数；起点为 RFC3339，单批 1–500 条，默认 500 |
| `-id` | `revoke` 操作的 credential ID |

[联调操作说明](./integration-runbook.md) 中的 `BI_ORGANIZATION_ID`、`BI_SOURCE_ID`、`BI_NAMESPACE`、`BI_ALLOWED_KINDS`、`BI_USAGE_SINCE` 是示例命令传参变量，不是后端自动读取的配置键。

## 当前固定值及尚未配置的策略

以下行为已有实现，但当前不是部署配置项：access Token 15 分钟、refresh 12 小时活动续期、绑定挑战 10 分钟、Context/分页快照 30 分钟、报告命令幂等回执 24 小时、分享默认及最长 7 天、认证每 IP 每分钟 30 次、主 BI API 请求执行预算 30 秒、worker 租约 2 分钟；指标时区固定为 Asia/Shanghai。具体请求限制仍以 OpenAPI 为准。

业务事实、内容历史和审计的物理保留策略尚待业务方确认，未设置或启用通用业务清理期限。`report_retention_months` 只控制报告可读期限；失败导入的未发布投影和过期临时状态按各自规则清理。

## 配置验证

`TestBIConfigLoadsEveryEnvironmentSetting` 从环境加载全部 7 项并核对结果；`TestBIConfigRequiresIndependentSecretsAndRealEnvironment` 检查必填与密钥隔离。部署检查已验证四套 Compose 的默认值与自定义覆盖值，并通过既有 application environment/security 脚本。上述检查不启动服务、不调用微信，也不证明真实凭证已配置。
