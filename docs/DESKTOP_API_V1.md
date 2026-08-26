# Desktop API v1

本文档定义 `devku-sub2api` 的 Desktop 管理接口和 Desktop 客户端接口。Desktop 企业、成员与普通面板 User 属于不同认证域；企业仅通过 `gateway_user_id` 绑定一个现有 active User，作为结算、Group 权限和成员 Model Token owner。

## 启用条件

- 默认配置为 `DESKTOP_ENABLED=false`。
- 关闭时服务端不校验 Desktop Secret，不注册本页接口，Public Settings 返回 `desktop_enabled: false`，Admin 入口隐藏。
- 启用前必须配置 `DESKTOP_JWT_SECRET` 和以 `/v1` 结尾的 HTTPS `DESKTOP_PUBLIC_GATEWAY_BASE_URL`。
- Admin 不提供 Secret 的读取或编辑入口。

## 通用约定

### Admin 鉴权与响应

Admin 接口沿用现有 Admin Bearer Token、全局限流、合规检查和 Audit Middleware。

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

分页响应的 `data`：

```json
{
  "items": [],
  "total": 0,
  "page": 1,
  "page_size": 20,
  "pages": 1
}
```

Admin 错误使用实际 HTTP 状态码，`code` 为数字，`reason` 为稳定字符串：

```json
{
  "code": 409,
  "message": "gateway user is already assigned",
  "reason": "GATEWAY_USER_ALREADY_ASSIGNED",
  "metadata": {}
}
```

### Desktop 鉴权与响应

成功响应：

```json
{
  "data": {}
}
```

错误响应：

```json
{
  "error": {
    "code": "MEMBER_INFORMATION_MISMATCH",
    "message": "member information does not match",
    "request_id": "req_xxx",
    "retryable": false,
    "details": {}
  }
}
```

客户端只根据 `error.code` 分支，不解析 `message`。`429` 响应包含 `Retry-After`，所有含 User Token、Model Token 或配置凭证的响应包含 `Cache-Control: no-store`。

Desktop auth 写请求进入 Audit Middleware，但审计记录只保存脱敏后的结构；`phone`、`refresh_token`、Authorization 等凭证字段不保存原值。

### Body limit

| 接口 | 上限 |
| --- | ---: |
| Desktop `lookup/login/refresh/logout` | 8 KiB |
| Admin 企业和成员创建、修改、删除、轮换 | 16 KiB |
| Admin Model Configuration PUT | 64 KiB |

超过上限返回 HTTP `413` 和 `PAYLOAD_TOO_LARGE`。Admin body limit 在 Audit Middleware 读取 JSON body 前生效。

## DTO

### Organization

```json
{
  "public_id": "org_e8YpYvP3lQ8z4WkTWYx9Rw",
  "code": "dhjy",
  "name": "东恒锦洋房产开发有限公司",
  "status": "active",
  "gateway_user": {
    "id": 1001,
    "email": "carrier@example.com",
    "username": "enterprise-carrier"
  },
  "group": {
    "id": 2001,
    "name": "Enterprise Responses"
  },
  "member_count": 3,
  "target_config_assigned": true,
  "created_at": "2026-08-26T02:00:00Z",
  "updated_at": "2026-08-26T02:30:00Z"
}
```

企业详情和写接口额外返回可空的 `target_config`；列表不返回该字段。

### Member

```json
{
  "public_id": "mem_cH5ro6X8y9xPkQpNZmPrBw",
  "name": "张明",
	"phone": "+8613800000000",
  "status": "active",
  "model_token_status": "active",
  "created_at": "2026-08-26T02:10:00Z",
  "updated_at": "2026-08-26T02:10:00Z"
}
```

`phone` 返回规范化后的 E.164 完整手机号。`model_token_status` 只允许 `active`、`disabled`、`missing`。Admin 响应不返回 raw Model Token。

### Target Config

```json
{
  "schema_version": 1,
  "targets": {
    "chatgpt_codex": {
      "enabled": true,
      "provider_id": "devku_enterprise",
      "display_name": "东恒锦洋企业模型",
      "requested_model": "enterprise-model-1",
      "wire_api": "responses",
      "minimum_app_version": null,
      "restart_required": true
    },
    "workbuddy": {
      "enabled": false,
      "provider_id": "devku_enterprise",
      "display_name": "东恒锦洋企业模型",
      "requested_model": "enterprise-model-1",
      "wire_api": "chat_completions",
      "minimum_app_version": null,
      "restart_required": false
    }
  }
}
```

`chatgpt_codex.wire_api` 固定为 `responses`，对应 Gateway `/responses`。`workbuddy.wire_api` 固定为 `chat_completions`，对应 Gateway `/chat/completions`。当前 Workbuddy 生产配置仍必须为 `enabled: false`。

## Admin API

Admin 基础路径：`/api/v1/admin/desktop`。

### 1. 创建企业

`POST /organizations`

Header：`Idempotency-Key` 必填。首次成功返回 `201`；相同 actor、route、key 和 payload 重放时返回原结果，并包含 `X-Idempotency-Replayed: true`。

```json
{
  "code": "dhjy",
  "name": "东恒锦洋房产开发有限公司",
  "gateway_user_id": 1001,
  "group_id": 2001
}
```

承载 User 必须 active、可绑定目标 active Group，且未承载其他未删除 Desktop 企业。冲突返回 `409 GATEWAY_USER_ALREADY_ASSIGNED`。

### 2. 企业列表

`GET /organizations?page=1&page_size=20&search=dhjy&status=active`

- `search`：企业名称或简码模糊查询。
- `status`：可选 `active`、`disabled`。
- 返回分页 Organization DTO。

### 3. 企业详情

`GET /organizations/{organization_id}`

返回 Organization DTO 和可空 `target_config`。

### 4. 修改企业

`PATCH /organizations/{organization_id}`

```json
{
  "name": "东恒锦洋集团",
  "status": "active",
  "gateway_user_id": 1001,
  "group_id": 2001
}
```

字段均可选。企业已有成员后，`gateway_user_id` 和 `group_id` 不可变更，返回 `409 ORGANIZATION_PROVISIONING_LOCKED`。停用企业会立即撤销 Desktop 授权并暂停符合条件的当前 Model Token；重新启用后，旧 Desktop Access Token 不会恢复有效。

### 5. 更新 Model Configuration

`PUT /organizations/{organization_id}/model-configuration`

```json
{
  "target_config": {
    "schema_version": 1,
    "targets": {
      "chatgpt_codex": {
        "enabled": true,
        "provider_id": "devku_enterprise",
        "display_name": "东恒锦洋企业模型",
        "requested_model": "enterprise-model-1",
        "wire_api": "responses",
        "minimum_app_version": null,
        "restart_required": true
      }
    }
  }
}
```

### 6. 创建成员

`POST /organizations/{organization_id}/members`

Header：`Idempotency-Key` 必填。首次成功返回 `201`。

```json
{
  "name": "张明",
  "phone": "13800000000"
}
```

手机号支持中国大陆 11 位格式或 `+86` 国际格式，统一规范化为 E.164 后明文存储；姓名规范化为 Unicode NFC。成员、独立 Model Token 和当前 Key 归属在同一事务创建，响应只返回 Member DTO。

### 7. 成员列表

`GET /organizations/{organization_id}/members?page=1&page_size=20&search=张明&status=active`

- 普通 `search` 只匹配姓名。
- 输入中国大陆 11 位手机号或对应的 `+86` E.164 完整手机号时，执行企业内手机号精确查询，不支持手机号模糊查询。
- 返回分页 Member DTO。

### 8. 修改成员

`PATCH /organizations/{organization_id}/members/{member_id}`

```json
{
  "name": "张明",
  "phone": "+8613800000000",
  "status": "disabled"
}
```

只接受 `name`、`phone`、`status`。任一凭据或状态变化都会递增 auth version 并撤销 Refresh family。停用成员会同时停用当前 Model Token。

### 9. 删除成员

`DELETE /organizations/{organization_id}/members/{member_id}`

删除为软删除终态，并在同一事务 tombstone 当前 Model Token、退休当前关联。成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": { "deleted": true }
}
```

### 10. 轮换 Model Token

`POST /organizations/{organization_id}/members/{member_id}/model-token/rotate`

Header：`Idempotency-Key` 必填。成功返回 `200` 和 Member DTO。企业或成员被停用时分别返回 `ORGANIZATION_DISABLED`、`MEMBER_DISABLED`。轮换不向 Admin 返回 raw Model Token。

### 可用承载 User 查询

创建或更换承载关系时使用现有接口：

`GET /api/v1/admin/users?available_for_desktop=true&page=1&page_size=30&search=carrier`

仅返回 active 且未承载其他未删除 Desktop 企业的 User。编辑企业时，当前承载 User 应通过 `GET /api/v1/admin/users/{id}` 单独 hydrate。

## Desktop API

Desktop 基础路径：`/api/desktop/v1`。

### 1. 企业查询

`POST /auth/organization-lookup`

Header：`X-Installation-ID: <canonical-uuid>` 必填。

```json
{
  "organization_code": "dhjy"
}
```

成功 `data`：

```json
{
  "public_id": "org_e8YpYvP3lQ8z4WkTWYx9Rw",
  "code": "dhjy",
  "name": "东恒锦洋房产开发有限公司"
}
```

企业不存在或已停用统一返回 `404 ORGANIZATION_NOT_FOUND`。

### 2. 登录

`POST /auth/login`

Header：`X-Installation-ID: <canonical-uuid>` 必填。

```json
{
  "organization_code": "dhjy",
  "name": "张明",
  "phone": "13800000000"
}
```

手机号支持中国大陆 11 位格式或 `+86` 国际格式。姓名、手机号、企业或成员状态不匹配时统一返回 `401 MEMBER_INFORMATION_MISMATCH`，不暴露具体失败字段。成功 `data`：

```json
{
  "access_token": "<desktop-access-token>",
  "refresh_token": "<desktop-refresh-token>",
  "token_type": "Bearer",
  "expires_in": 900
}
```

### 3. 刷新会话

`POST /auth/refresh`

```json
{
  "refresh_token": "<desktop-refresh-token>"
}
```

每次成功都会原子消费旧 Refresh Token 并返回新 Token pair，30 天 family 绝对期限不延长。并发刷新只允许一次成功；旧 Token 重放会撤销整个 family。未知、过期或重放返回 `401 REFRESH_TOKEN_INVALID`，Redis 不可用返回 `503 DESKTOP_AUTH_STORE_UNAVAILABLE`。

### 4. 退出

`POST /auth/logout`

Header：`Authorization: Bearer <desktop-access-token>`。

成功 `data`：`{"logged_out":true}`。退出撤销当前 Refresh family；已签发 Access Token 最长仍可存活至 15 分钟 TTL，但客户端必须立即清理本地 Token。

### 5. 当前成员

`GET /me`

Header：`Authorization: Bearer <desktop-access-token>`。

```json
{
  "data": {
    "public_id": "mem_cH5ro6X8y9xPkQpNZmPrBw",
    "name": "张明",
		"phone": "+8613800000000",
    "organization_id": "org_e8YpYvP3lQ8z4WkTWYx9Rw",
    "organization_code": "dhjy",
    "organization_name": "东恒锦洋房产开发有限公司"
  }
}
```

### 6. Model Configuration

`GET /model-configuration?targets=chatgpt_codex`

Header：

- `Authorization: Bearer <desktop-access-token>`
- `If-None-Match: "cfg_xxx"`，可选，仅用于常规更新检查。

成功响应包含 `Cache-Control: no-store` 和 `ETag`：

```json
{
  "data": {
    "configuration_version": "cfg_5bd012371b9db10a0e8c",
    "base_url": "https://gateway.example.com/v1",
    "model_token": "<raw-model-token>",
    "targets": {
      "chatgpt_codex": {
        "enabled": true,
        "provider_id": "devku_enterprise",
        "display_name": "东恒锦洋企业模型",
        "requested_model": "enterprise-model-1",
        "wire_api": "responses",
        "minimum_app_version": null,
        "restart_required": true
      }
    }
  }
}
```

ETag 匹配时返回 `304` 且无 body。首次部署、本机配置丢失或凭证恢复时不得发送 `If-None-Match`。`configuration_version` 由 canonical config、Gateway URL、API Key ID 和状态生成，不读取 raw Key，也不依赖随用量变化的 `updated_at`。

### 7. 用量摘要

`GET /usage/summary?timezone=Asia/Shanghai`

Header：`Authorization: Bearer <desktop-access-token>`。

```json
{
  "data": {
    "timezone": "Asia/Shanghai",
    "today": {
      "used": 18742,
      "limit": null,
      "remaining": null
    },
    "month": {
      "used": 436210,
      "limit": null,
      "remaining": null
    }
  }
}
```

统计成员当前与全部历史 Model Token 的 Token 数。`limit` 和 `remaining` 固定为 `null`，因为现有 API Key quota 单位为 USD，不是 Token。

## 错误码

### Admin 稳定 reason

| HTTP | reason | 含义 |
| ---: | --- | --- |
| 409 | `GATEWAY_USER_ALREADY_ASSIGNED` | 承载 User 已绑定其他未删除企业 |
| 409 | `ORGANIZATION_PROVISIONING_LOCKED` | 企业已有成员，承载 User 或 Group 已锁定 |
| 409 | `ORGANIZATION_DISABLED` | 企业被停用，不能创建、恢复成员或轮换 Token |
| 409 | `MEMBER_DISABLED` | 成员被停用，不能轮换 Token |
| 409 | `DESKTOP_MANAGED_API_KEY` | 通用 API Key 接口拒绝修改 Desktop 托管 Key |
| 409 | `MODEL_TOKEN_ROTATION_CONFLICT` | Token 轮换并发冲突或幂等参数冲突 |
| 409 | `DESKTOP_ORGANIZATION_DEPENDENCY` | User、Group 或 AllowedGroups 仍被企业依赖 |
| 413 | `PAYLOAD_TOO_LARGE` | 请求体超过接口上限 |
| 422 | `VALIDATION_FAILED` | 请求字段不合法 |

### Desktop error.code

| HTTP | code | retryable | 含义 |
| ---: | --- | :---: | --- |
| 401 | `UNAUTHENTICATED` | false | Access Token 缺失、无效或过期 |
| 401 | `REFRESH_TOKEN_INVALID` | false | Refresh Token 无效、过期或重放 |
| 401 | `MEMBER_INFORMATION_MISMATCH` | false | 登录信息不匹配 |
| 403 | `MEMBERSHIP_REVOKED` | false | 成员、企业、承载 User 或 auth version 已撤销 |
| 404 | `ORGANIZATION_NOT_FOUND` | false | active 企业不存在 |
| 404 | `MODEL_CONFIGURATION_NOT_ASSIGNED` | false | 当前没有可下发的有效配置或 Model Token |
| 413 | `PAYLOAD_TOO_LARGE` | false | 请求体超过 8 KiB |
| 422 | `VALIDATION_FAILED` | false | Header、字段或时区不合法 |
| 429 | `RATE_LIMITED` | false | 登录保护限流，按 `Retry-After` 等待 |
| 503 | `DESKTOP_AUTH_STORE_UNAVAILABLE` | true | 登录保护或 Refresh Store 不可用，fail closed |
| 503 | `USAGE_SOURCE_UNAVAILABLE` | true | 用量聚合不可用 |

## Gateway 调用

服务端不新增 Desktop 专用 Gateway。Desktop 使用 Model Configuration 返回的值调用对应 Gateway 接口。

ChatGPT Codex：

```http
POST {base_url}/responses
Authorization: Bearer <model_token>
Content-Type: application/json
```

Workbuddy：

```http
POST {base_url}/chat/completions
Authorization: Bearer <model_token>
Content-Type: application/json
```

当前 Workbuddy target 仍保持 `enabled: false`；正式启用后才会下发给 Desktop 客户端。

成员、企业或承载 User 撤销后，Desktop API 与 Refresh 通过数据库状态立即拒绝；Model Token 通过 API Key 状态、即时缓存失效和 auth cache invalidation outbox 失效。
