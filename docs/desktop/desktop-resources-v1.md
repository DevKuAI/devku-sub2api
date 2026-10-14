# Desktop 资源发布与分发契约 v1

本契约覆盖 sub2api 后端与 Web 管理后台；Desktop Rust 安装和独立图片 MCP 需另行联调。HTTP 定义见 [OpenAPI 3.1](openapi/desktop-resources-v1.json)，登录与企业模型配置见 [Desktop API](DESKTOP_API_V1.md)。

## 范围与权限

MCP 和 Skill 共用资源目录，以 `kind` 区分。MCP 是本机 stdio 可执行包，Skill 是文件包，不接收远程 HTTP MCP。服务器仅校验、存储与分发，不执行包内程序，不安装依赖，不接收本机安装记录、Model Token 或生成图片。

- 第一版只允许管理员发布公共资源，复用管理员 Bearer / `x-api-key` 鉴权、合规检查和操作审计。
- Desktop 目录、详情及下载 API 使用成员 User Token。v2 `dks_` Session 必须携带匹配的 `X-Installation-ID`；旧版 JWT 可省略。Model Token 不能用于资源 API。
- `scope=public` 返回已启用公共资源；`scope=enterprise` 返回空数组。没有企业发布入口，客户端不能指定企业归属。
- 所有资源接口和后台菜单受 `desktop.enabled` 控制。关闭时不注册资源路由。

**资源 ZIP 使用 private bucket，不提供长期公共地址。** 复用 `desktop_update_storage` 的 endpoint、region、凭据、prefix 和 path style，新增 `resource_bucket` 指定资源私有桶。必须设置 `DESKTOP_UPDATE_STORAGE_RESOURCE_BUCKET`，不会回退到安装包的 bucket；资源模块不要求 public_base_url。对象位于 `<prefix>/resources/<key>/<version>/<platform>/<UUID>.zip`。存储桶必须关闭匿名读取及公共域名，复用凭据需具备该桶的 PutObject / GetObject 权限；应用不会自动修改云端 Bucket policy。

鉴权后按次签发 S3 presigned URL，有效期固定 5 分钟。签名地址到期前可重复访问，不是严格单次消费链接。停用资源或撤销成员权限后不再签发新链接，已签发链接到期失效。URL 不写入数据库、不进入列表/详情或审计日志。客户端收到 URL 后直接从 S3 下载，不携带 User Token 或 Model Token。

## 发布与后台管理

管理员路由前缀：`/api/v1/admin/desktop/resources`。

| Method / 路径 | 行为 |
| --- | --- |
| `POST /validate` | 原始 ZIP 预检，不写对象存储和数据库；返回 manifest、sha256、sizeBytes、当前资源或 null |
| `POST /` | 原始 ZIP 上传并立即登记发布，返回 `{"data": Resource}`，仅含本次上传平台 |
| `GET /` | 管理员列表，包含停用资源；支持 page、page_size、kind、status、search、精确 key |
| `GET /{resource_id}` | 当前资源、所有当前平台及启停信息 |
| `GET /{resource_id}/versions` | 历史版本，按数字版本降序，包含平台、哈希与发布者 |
| `POST /{resource_id}/versions/{version}/artifacts/{platform}/download-url` | 管理员鉴权后为已启用资源签发下载地址 |
| `PATCH /{resource_id}/status` | `{"status":"disabled","reason":"原因"}` 或 `{"status":"active"}` |

两个 ZIP 接口均使用 `Content-Type: application/zip`，不使用 multipart 或 base64。其他管理接口沿用 `{code,message,data}` 信封。管理分页默认 20，page_size 最大 100，offset 最大 10000。列表按 updated_at DESC、id ASC 排序。

管理页面流程为选择 ZIP、预检、核对元数据与版本影响、确认发布。预检不预留版本，确认时重新校验。元数据只取自 manifest，不通过表单覆盖。发布进度达到 100% 只表示请求上传完成，仍需等待服务端确认。

上传流程：临时文件 → ZIP 校验 → 对象存储上传 → 数据库短事务登记 Resource / Version / Artifact。只有事务提交成功的记录才进入目录。对象存储与 PostgreSQL 不构成同一事务；数据库失败可能留下未被引用的对象。服务端日志记录对象 key 与 SHA-256 供核实，不在提交结果不确定时自动删除对象。运维清理必须先确认数据库没有引用。

发布超时、5xx 或 409 时，不自动重传或直接提示成功。后台按精确 key 查询资源，再在历史版本中核对版本、平台、SHA-256 和 sizeBytes。匹配则确认文件已发布；无法确认则保留「发布结果待核实」，可再次点击核实。v1 不提供 Idempotency-Key。

## 版本与启停

- 公共 key 唯一，跨版本保留同一 `res_` + 32 位小写十六进制 ID；kind 不可改变。
- `(resource_id, version, platform)` 不可变，重复上传即使字节相同也返回 409。
- 只允许新版本，或为当前版本补充平台；当前版本之前的版本不允许新增平台。
- 同版本的 kind、name、description、sourceUrl 必须一致；entry、args、targets 和文件允许因平台不同而不同。
- 新版本首个平台发布后立即成为当前版本；未上传的其他平台暂不可安装，不混用旧版本。历史版本仍可通过固定版本 API 下载。
- 新资源默认 active。停用后，Desktop 列表过滤该资源，详情和所有历史下载 API 返回相同的 404。后台保留管理能力，允许发布修复版本；上传不改变 disabled 状态。
- 停用原因必填，最多 2000 UTF-8 字节；恢复清空当前原因，操作记录保留在审计日志。停用不删除文件、不终止已授权的下载、不影响本机已安装文件。停止签发新下载地址，已签发的 presigned URL 最长 5 分钟后失效。
- v1 不包含删除、草稿、审核、回退、多平台原子发布和自动升级。

## Desktop 读取接口

| Method / 路径 | 返回 |
| --- | --- |
| `GET /api/desktop/v1/resources` | `{"data": Resource[]}`，空列表为数组 |
| `GET /api/desktop/v1/resources/{resource_id}` | `{"data": Resource}`，当前版本的所有平台 |
| `GET /api/desktop/v1/resources/{resource_id}/versions/{version}/artifacts/{platform}` | 兼容接口：固定版本 ZIP 二进制 |
| `POST /api/desktop/v1/resources/{resource_id}/versions/{version}/artifacts/{platform}/download-url` | 成员鉴权后签发固定版本的临时下载地址 |

列表参数与原 v1 保持一致：scope 缺省 public；kind 缺省全部，只接受 mcp / skill；search 最多 200 UTF-8 字节，匹配 name / description，不区分大小写，`%`、`_` 是 ILIKE 通配符。limit 为 1–100，缺省 50，非法值回退到 50；offset 为 0–10000，缺省或非数字为 0，越界返回 422。客户端通常每页取 20 条。

列表无 total / hasMore / cursor，以资源为分页单位，当前所有平台放在 artifacts 中。发布可能改变排序，不保证跨页快照，刷新时重置分页。Resource / Artifact / Manifest 使用 camelCase。管理员专属状态和发布者不会出现在 Desktop 响应中；列表与详情不携带任何存储 URL。

目录、详情及签发接口返回 `Cache-Control: no-store`。兼容下载接口返回 `application/zip`、`Cache-Control: private, no-store`、`X-Content-Type-Options: nosniff`、`Content-Disposition: attachment; filename="resource.zip"`、`X-Artifact-SHA256` 和 Content-Length。后端通过 S3 SDK 读取对象，不向对象存储转发 User Token。v1 不承诺 Range、断点续传、ETag 或 304。

签发请求不需要请求体。Desktop 响应示例（管理员接口外层为标准 `{code,message,data}`）：

```json
{"data":{"url":"https://private-storage.example.com/resources/package.zip?X-Amz-Signature=...","expiresAt":"2026-09-20T03:05:00Z","expiresIn":300,"sha256":"<原始 ZIP SHA-256>","sizeBytes":123456}}
```

客户端不得记录、持久化或转发签名 URL，也不得向存储域名发送 Desktop 鉴权 Header。签名请求固定 attachment 文件名、ZIP Content-Type 和 `private, no-store`。地址到期后重新向签发 API 鉴权申请；S3 的错误格式可能是 XML，不能按 Desktop JSON 信封解析。后台「下载 ZIP」每次点击都会获取新链接，停用状态下禁用该按钮。

Rust 安装预览应固定资源 ID、版本、平台、SHA-256 和 sizeBytes；确认后下载该固定版本，不静默切换。必须比对实际字节数、目录预期哈希、包内 manifest 和每个文件哈希；下载 Header 不能替代预期哈希。无匹配架构或 targets 不包含目标客户端时不可安装。

## ZIP 与 Manifest

ZIP 包含根目录 `manifest.json` 与运行所需文件。完整 schema 和 MCP 示例见 OpenAPI。

| 字段 | 规则 |
| --- | --- |
| schemaVersion | 固定 1，拒绝未知字段和重复 JSON key |
| key | `^[a-z][a-z0-9_-]{1,63}$` |
| kind | mcp / skill |
| name | 必填，去空白后非空，最多 200 UTF-8 字节 |
| description / sourceUrl | 必须显式提供，允许空字符串，分别最多 2000 / 2048 UTF-8 字节；非空 sourceUrl 必须 HTTPS 且不含用户凭据 |
| version | x.y.z，每段最多 6 位非负整数，无多余前导零、预发布或构建后缀 |
| platform | MCP 为 darwin-arm64 / darwin-x64 / windows-x64 / linux-x64；Skill 为 any |
| entry | 安全相对路径，最多 240 UTF-8 字节，必须存在于 files |
| args | 可选，最多 32 项，每项最多 1000 UTF-8 字节，不含 NUL |
| credentialMode | 可选，空或 enterprise_model |
| targets | 1–2 项，workbuddy / chatgpt_codex，不重复 |
| files | 文件相对路径 → 64 位小写 SHA-256；不包含 manifest.json 自身 |

ZIP ≤64 MiB，单文件 ≤64 MiB，展开总量 ≤128 MiB，≤1024 项（含目录和 manifest），manifest ≤128 KiB。仅接受普通文件和目录，拒绝路径穿越、绝对路径、反斜杠、符号链接、加密条目、大小写重名、文件/目录冲突及 Windows 保留名称。每个普通文件必须被 files 覆盖；拒绝缺失、额外文件及哈希不符。

Skill 必须 `platform=any`、`entry=skill/SKILL.md`，文件全部位于 `skill/`，args 和 credentialMode 为空或省略。仅校验包结构，不判断 Skill 内容行为；Desktop 的目标客户端支持情况由本机检测。MCP 作为 stdio 子进程启动，args 不经过 shell；`{binding}` 由 Rust 替换为不包含 Token 的本机绑定文件路径。

离线校验与服务器复用同一实现：

```sh
cd backend
go run ./cmd/desktop-resource-check /path/to/resource.zip
```

命令成功输出 manifest、原始 ZIP 哈希与字节数，失败返回非零退出码，不修改包或执行程序。

## 错误与联调

发布与 Desktop 读取使用 `{error:{code,message,request_id,retryable,details}}`。预检及其余管理接口使用后台错误信封；管理员中间件可能返回 401、403、423，代理也可能返回非 JSON 错误。后台兼容两类信封。

| HTTP | code / 行为 |
| --- | --- |
| 400 | RESOURCE_PACKAGE_INVALID，修复包后重试 |
| 401 | UNAUTHENTICATED，按现有登录协议处理 |
| 403 | MEMBERSHIP_REVOKED / SESSION_INSTALLATION_MISMATCH，停止资源操作 |
| 404 | RESOURCE_NOT_FOUND，不区分资源、版本、平台不存在或资源已停用 |
| 409 | RESOURCE_VERSION_CONFLICT，版本/平台重复、版本倒退、类型或元数据冲突 |
| 413 | PAYLOAD_TOO_LARGE，超过 ZIP 请求体限制 |
| 422 | VALIDATION_FAILED 或既有安装标识错误，修正请求 |
| 503 | RESOURCE_STORAGE_UNAVAILABLE，检查共享连接配置、resource_bucket 或服务状态 |

部署前执行迁移并配置既有 Desktop 连接凭据及私有资源桶 `DESKTOP_UPDATE_STORAGE_RESOURCE_BUCKET`。验收顺序为预检与发布、后台历史核对、成员列表/详情、签发地址与固定版本下载的哈希比对、停用/恢复和历史下载、Desktop 真实安装握手。安装握手不得隐式触发生图；收费图片请求单独确认执行。Web 后台测试和 API 测试不代表 Rust 安装或真实对象存储已通过联调。
