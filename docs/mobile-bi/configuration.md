# BI 配置参考

当前服务包含 9 个 `bi.*` 配置项。后端配置加载、启动校验、[YAML 示例](../../deploy/config.example.yaml)、[环境变量示例](../../deploy/.env.example) 以及 `docker-compose.yml`、`docker-compose.local.yml`、`docker-compose.standalone.yml`、`docker-compose.dev.yml` 均覆盖这些字段。独立进程可读取 YAML 或环境变量；Compose 中的 `.env` 变量通过模板传入应用容器。

非空环境变量优先于 YAML。Compose 为 `BI_ENABLED`、`BI_MIN_CLIENT_VERSION` 和 `BI_REPORT_RETENTION_MONTHS` 设置了显式默认值；使用 Compose 时在 `.env` 中修改这些值，避免挂载 YAML 中的同名设置被默认环境值覆盖。空字符串环境变量按当前 Viper 行为忽略，仍可回落到 YAML 或默认值。

## 服务配置

| YAML 键 | 环境变量 | 默认值 | 校验与用途 |
| --- | --- | --- | --- |
| `bi.enabled` | `BI_ENABLED` | `false` | 启用 BI 路由及服务内 worker |
| `bi.appid` | `BI_APPID` | 空 | 启用时必填；实际小程序 AppID |
| `bi.app_secret` | `BI_APP_SECRET` | 空 | 启用时必填；仅服务端访问 code2Session |
| `bi.jwt_secret` | `BI_JWT_SECRET` | 空 | 启用时必填；标准 base64 解码至少 32 字节；小程序 JWT 签名 |
| `bi.identity_secret` | `BI_IDENTITY_SECRET` | 空 | 启用时必填；标准 base64 解码至少 32 字节；微信身份 HMAC 索引及摘要 |
| `bi.privacy_notice_url` | `BI_PRIVACY_NOTICE_URL` | 空 | 启用时必填；绝对 HTTPS URL，不能包含 URL 用户凭证 |
| `bi.privacy_notice_version` | `BI_PRIVACY_NOTICE_VERSION` | 空 | 启用时必填；随 bootstrap 返回 |
| `bi.min_client_version` | `BI_MIN_CLIENT_VERSION` | `0.1.0` | 启用时不能为空；随 bootstrap 返回，服务端不据此比较或阻断客户端版本 |
| `bi.report_retention_months` | `BI_REPORT_RETENTION_MONTHS` | `24` | 报告可读期限，1–120 个自然月；兼容值 0 按 24 个月处理 |

两份 BI 密钥必须彼此独立，并与原网页 JWT、Desktop JWT 密钥分离。`identity_secret` 应跨重启保持稳定，轮换需要身份索引迁移方案。生成示例为 `openssl rand -base64 32`，每份密钥分别生成；不在文档或前端保存实际值。

配置只校验格式与必要条件，真实 AppID/AppSecret 是否可用、域名是否合法仍需微信环境验证。当前没有原站设置页面编辑这些部署配置，需通过配置文件或部署环境维护。原站页面负责微信绑定及企业管理授权。

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

`TestBIConfigLoadsEveryEnvironmentSetting` 从环境加载全部 9 项并核对结果；`TestBIConfigRequiresIndependentSecretsAndRealEnvironment` 检查必填、密钥隔离及 HTTPS。部署检查已验证四套 Compose 的默认值与自定义覆盖值，并通过既有 application environment/security 脚本。上述检查不启动服务、不调用微信，也不证明真实凭证已配置。
