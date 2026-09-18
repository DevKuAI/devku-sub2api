# Desktop v2 会话、对话上报与后台回看

本合同用于 Desktop 后端、桌面客户端和 Hook 联调。接口实现与 Web 管理页面位于当前仓库；桌面端和 Hook 需另行接入。本文不表示已经部署生产环境。

可导入的完整合同：[OpenAPI 3.1](DESKTOP_SESSION_CONVERSATION_V2.openapi.json)。旧客户端合同见 [Desktop API v1](DESKTOP_API_V1.md)。

## 启用与兼容

- 沿用 `DESKTOP_ENABLED`、企业名单、承载 User 和现有模型配置；功能关闭时不上线相关客户端和管理路由。
- `POST /api/desktop/v1/auth/login` 无版本头或 `X-Desktop-Auth-Version: 1` 时仍返回 JWT、refresh token 和 `expires_in`，不改变旧 refresh 轮换行为。
- `X-Desktop-Auth-Version: 2` 请求稳定会话；其他版本返回 `422 AUTH_VERSION_UNSUPPORTED`。v2 的 `X-Installation-ID` 必须为小写、带连字符的 UUID。
- `/me`、`/model-configuration`、`/usage/summary` 和 `/auth/logout` 同时接受 v1/v2，v2 请求均须携带绑定的安装 ID；校验失败不切换验证器。
- 新客户端必须确认登录响应的 `auth_version` 为 2，不能将旧 JWT 当成长效凭证，也不能按 `idle_expires_at` 快照定时退出。

## 企业对话上报开关

管理员在「创建企业」和「编辑企业」中勾选「上报对话记录」，对应 `conversation_reporting_enabled`。新企业默认关闭；migration 240 将已有企业也设为关闭，需要管理员明确开启。企业管理账号只读，不能自行修改此开关。

`GET /api/desktop/v1/model-configuration` 的 `data` 增加必返布尔字段：

```json
{
  "data": {
    "configuration_version": "cfg_example",
    "base_url": "https://gateway.example.com/v1",
    "model_token": "<Model Token>",
    "targets": {},
    "conversation_reporting_enabled": false
  }
}
```

上例仅展示字段位置，`targets` 的实际内容沿用原模型配置合同。Desktop 仅在该字段明确为 `true` 时启用对话采集与上报；缺失视为关闭。开关参与 `configuration_version` 和 ETag，切换后旧 ETag 不会导致误返回 304，不要求用户重新登录。

后端在读取上报正文前拒绝已关闭的企业，并在入库事务内锁定企业行、再次检查开关。正常关闭请求不消耗上报限额、不 touch 会话；并发请求若此前已经 touch，但在入库前遇到关闭，也不会保存记录。

关闭时返回 `403 CONVERSATION_REPORTING_DISABLED`。用户企业管理页隐藏「对话记录」Tab，直接访问用户查询 API 也返回 403。已进入该 Tab 的页面再次加载到关闭配置后回到「成员」。管理员仍可查询历史记录，关闭不删除已存数据。

## 登录与 15 天滑动会话

```http
POST /api/desktop/v1/auth/login
X-Desktop-Auth-Version: 2
X-Installation-ID: 4cd825ab-f923-4515-9d0f-a2a9c968fbaf
Content-Type: application/json

{"organization_code":"example","name":"测试员工","phone":"+8613800000000"}
```

```json
{
  "data": {
    "auth_version": 2,
    "token_type": "Bearer",
    "access_token": "dks_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
    "session_id": "session_example",
    "idle_timeout_seconds": 1296000,
    "idle_expires_at": "2026-10-03T00:00:00Z"
  }
}
```

示例凭证仅为占位。实际凭证使用 32 字节 CSPRNG 随机数，不包含身份信息。后端仅用 SHA-256 摘要定位 Redis 中的 `desktop_session_v2:{token_sha256}`，不保存原文，不维护每次活动的历史 key。

每次业务访问先检查会话、当前成员/企业/承载 User、auth version 和安装 ID，再完成参数校验及适用的限流，最后原子 touch。活动时间和期限均使用 Redis 时间。持续活动没有 30 天绝对上限；`now >= idle_expires_at` 时失效，不能通过请求复活。配置 `304` 也计入活动。

合法上报在数据库事务前续期，因此存储失败可已续期。格式、身份、参数或限流失败不续期。通过校验后发现重复 record ID 的请求可已续期，但不覆盖数据。

`POST /api/desktop/v1/auth/logout` 无需正文，只删除当前 v2 会话，不续期。删除后到达的 touch 必须失败；已经通过鉴权的在途请求可以完成。其他设备会话不受影响。v2 不使用 `/auth/refresh`，模型 Gateway 调用及公开更新检查不延长 v2 会话。

Redis 不可用返回 `503 DESKTOP_AUTH_STORE_UNAVAILABLE`；记录丢失、到期或注销返回 401。停用成员/企业/承载 User 或 auth version 不匹配返回 `403 MEMBERSHIP_REVOKED`。

## 一轮对话写入

```http
POST /api/desktop/v1/conversation-records
Authorization: Bearer <v2 access_token>
X-Installation-ID: 4cd825ab-f923-4515-9d0f-a2a9c968fbaf
Content-Type: application/json
Cache-Control: no-store
```

```json
{
  "schemaVersion": 2,
  "recordId": "0fbf4d44-2fc6-4e94-99f3-beb6bf40a656",
  "client": "workbuddy",
  "installationId": "4cd825ab-f923-4515-9d0f-a2a9c968fbaf",
  "organizationId": "org_example",
  "memberId": "mem_example",
  "sessionId": "original-ai-conversation",
  "sourceTurnId": null,
  "startedAt": "2026-09-18T00:00:00Z",
  "stoppedAt": "2026-09-18T00:01:00Z",
  "cwd": "/workspace/example",
  "prompts": [{"text": "解释这个函数", "truncated": false}],
  "response": {"text": "这个函数用于……", "truncated": false},
  "captureStatus": "captured"
}
```

仅接受 v2。一次请求保存一轮，不提供批量、分片、上传重试队列或幂等回执缓存；`Idempotency-Key` 不参与处理。事务提交后返回 `200 {"data":{"record_id":"…","received_at":"…"}}`。客户端超时可能发生在提交之后，本协议不承诺客户端获知最终交付状态。

| 字段或限制 | 规则 |
| --- | --- |
| `schemaVersion` | 必填整数 2，与 HTTP 路径版本独立 |
| `recordId` | 小写 UUID v4；同企业重复返回 409 |
| `client` | `workbuddy` 或 `chatgpt_codex` |
| `installationId` | 与请求头、会话完全一致的小写 UUID |
| `organizationId`、`memberId` | 各 1–64 字符；匹配当前鉴权身份，入库外键取自服务端 |
| `sessionId` | 原始 AI 对话 ID，1–512 字符，不是登录 `session_id` |
| `sourceTurnId` | 可省略或 null，否则 1–512 字符 |
| `startedAt`、`stoppedAt` | UTC RFC 3339，以 `Z` 结尾，开始不晚于结束；不限制距当前时间的间隔 |
| `cwd` | 可省略或 null，最多 4,096 字符；仅作文本保存 |
| `prompts` | 1–64 项，保留原顺序 |
| `text`、`truncated` | 每段均必填；允许空字符串和 false，不能为 null |
| 单段 `text` | 解码后 UTF-8 最多 2 MiB（2,097,152 字节） |
| `response`、`captureStatus` | 均必填；`captured` 对应响应对象，`response_missing` 对应显式 null |
| 整体请求 | 含 JSON 转义和空白最多 32 MiB（33,554,432 字节），分块传输同样限制 |

所有对象拒绝未知字段、重复字段、字段名大小写别名和尾随 JSON。拒绝无效 UTF-8、无效 Unicode 转义和 PostgreSQL 无法保存的 NUL（U+0000），返回 422，不静默替换正文。只支持 `application/json`；`Content-Encoding` 必须缺省或 `identity`。

上报独立限流：默认每成员每分钟 60 次，同一成员的设备和客户端合并计数，使用首次请求开始的 60 秒窗口。通过 `desktop.conversation_member_per_minute` 或 `DESKTOP_CONVERSATION_MEMBER_PER_MINUTE` 配置正整数；不计入登录失败次数。

所有客户端响应带 `Cache-Control: no-store` 和 `X-Request-ID`，错误结构沿用 Desktop：

```json
{"error":{"code":"UNAUTHENTICATED","message":"authentication required","request_id":"request-id","retryable":false,"details":{}}}
```

| HTTP | code |
| --- | --- |
| 401 | `UNAUTHENTICATED`、`SESSION_AUTH_REQUIRED` |
| 403 | `MEMBERSHIP_REVOKED`、`SESSION_INSTALLATION_MISMATCH`、`CONVERSATION_IDENTITY_MISMATCH`、`CONVERSATION_REPORTING_DISABLED` |
| 409 | `CONVERSATION_RECORD_EXISTS` |
| 413 | `PAYLOAD_TOO_LARGE` |
| 415 | `UNSUPPORTED_MEDIA_TYPE` |
| 422 | `VALIDATION_FAILED` |
| 429 | `RATE_LIMITED` |
| 503 | `DESKTOP_AUTH_STORE_UNAVAILABLE`、`CONVERSATION_STORAGE_UNAVAILABLE` |

Hook 对所有结果均结束本轮并清理临时正文，不根据 `retryable` 或 `Retry-After` 刷新、重发或补传。关闭采集仅影响新记录，不删除后台历史。

## 后台查询与回看

后台沿用 Web Bearer 认证及 `{code,message,data}` 包装，不使用 Desktop 会话凭证。

| 角色 | 集合地址 |
| --- | --- |
| 平台管理员 | `/api/v1/admin/desktop/organizations/{organization_id}/conversation-records` |
| 企业管理账号 | `/api/v1/desktop/organization/conversation-records` |

集合地址 `GET` 返回分页元数据，追加 `/{record_id}` 的 `GET` 返回完整一轮。企业端归属由当前登录 User 推导，不能由参数切换企业。详情始终同时匹配企业和 record ID；跨企业返回 404。

列表筛选参数：`member_id`、`member_search`（姓名或 ID 子串）、`client`、`capture_status`、`record_id`、`source_session_id`、`installation_id`、`received_from`、`received_to`。时间按服务端接收时间过滤，起点包含、终点不含，必须起点早于终点。

`page` 默认 1，`page_size` 默认 20、最多 100；`sort_order` 为 `asc` 或 `desc`，默认 `desc`。先按 `received_at`，再按内部 ID 排序。列表不读取或返回 `prompts` 和 `response`。

原始对话回看固定「企业、成员、客户端、安装 ID、原始对话 ID」五项，并采用升序分页；同名原始对话不会跨设备或客户端合并。详情中的 `schema_version`、元数据使用 snake_case，文本段仍为 `text`、`truncated`。

企业详情页和企业自管理页共享「对话记录」Tab。正文按需加载，以纯文本显示，长文本展开不会修改存储内容。成员 soft delete 不删除历史；列表与详情会显示 `member_deleted`。当前不提供会话管理、记录修改、导出或删除功能。

## 存储、日志与部署配置

- 增量 migration `239_desktop_conversation_records.sql` 创建完整单行存储、企业内唯一索引、企业接收时间索引及对话回看索引。成员、企业外键不级联删除；暂无按时间清理任务。
- 上报不挂载正文审计，不进入 Gateway 提示词采集；专用日志仅记录请求标识、记录 ID、状态、字节数和耗时。后台全文读取审计记录操作者和记录归属，不复制正文。
- 仓库 Caddy 示例针对上报独立限制 33,554,432 字节并格式化代理 413；其他路由保持原限制。应用登录路由仍为 8 KiB。全局应用入口限制不得小于对话路由上限。
- 使用其他反向代理时，需配置相同字节上限，并对代理拒绝返回相同的 Desktop 413 JSON、`no-store` 和 request ID。生产代理配置需在部署时另行核对。
- Redis 丢失需要重新登录；增加对话表不迁移旧 refresh 会话。上线顺序为后端兼容实现、桌面端接入、用户显式 v2 登录并重新应用 Hook。

## 验证

后端覆盖会话边界/并发撤销、严格合同/32 MiB HTTP 边界、限流、数据库提交失败、重复并发写入、企业隔离和已删除成员历史。前端覆盖正文按需读取、纯文本展示、对话归属及企业切换竞态，并纳入关键 CI 测试。

代理集成测试使用临时 Docker Caddy 和本地测试上游，不连接生产服务：

```sh
python3 deploy/tests/desktop-conversation-proxy-test.py
```
