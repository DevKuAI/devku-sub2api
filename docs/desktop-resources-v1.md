# Desktop 资源发布与分发契约 v1

状态：与当前本地实现对齐，可用于后端、Desktop 和发布工具联调；尚未部署。HTTP 定义见 [OpenAPI 3.1](openapi/desktop-resources-v1.json)。本文件是资源接口的事实来源，[Desktop API 总合同](DESKTOP_API_V1.md) 负责登录、企业、成员和模型配置。

## 1. 范围与责任

MCP 和 Skill 共用 `resources` 目录，以 `kind` 区分。v1 发布能力为“本机 stdio MCP 可执行包”与“Skill 文件包”；不把任意远程 MCP URL 当作可执行包。图片 MCP 是第一个资源，不单独设计图片资源分发接口。

| 参与方 | 负责内容 |
| --- | --- |
| 发布工具 / 管理后台 | 生成平台 ZIP 和 manifest，上传公共资源，展示发布成功或错误 |
| 企业后端 | 鉴权、校验 ZIP、持久化元数据及字节、不可变版本、公共/企业可见性 |
| Desktop Rust | 使用 User Token 读目录、下载校验、检测本机安装、预览、备份、安装/移除、握手 |
| Desktop WebView | 两个列表、筛选、详情、安装确认；通过 Tauri 命令调用 Rust |
| 独立图片 MCP | 工具调用时读取已部署企业模型凭证，向企业网关发出生图请求 |

资源目录、图片运行程序和 Skill 内容不随 Desktop 安装包内置。后端不接收本机安装记录、配置文件、Model Token 或生成图片。`installed` / `canInstall` 等状态由 Rust 本机检测，不属于云端 Resource。

## 2. 鉴权与隔离

- 公共发布：`Authorization: Bearer <admin-access-token>`，沿用管理员角色、合规和审计规则。既有 `x-api-key` 管理员鉴权方式同样适用。
- 目录/详情/下载：`Authorization: Bearer <desktop-access-token>`，即员工 User Token。公共表示所有已授权企业成员可见，不表示匿名开放。
- Desktop 继续发送 `X-Installation-ID`；资源读取接口当前不要求该 Header，登录/刷新要求见总合同。
- `scope=public` 只返回公共资源；`scope=enterprise` 只返回当前员工所属企业资源。二者由前端分别请求。
- 请求不得携带 `organizationId`、`createdBy` 来指定权限。未来企业上传时也必须从已鉴权成员推导所属企业和上传者。
- 列表直接过滤不可见资源；详情和下载对“其他企业资源”和“不存在资源”统一返回 404。
- User Token 只用于目录服务；Model Token 只用于企业模型/图片网关，二者不可互换。WebView 不接触这两类 Token。

## 3. HTTP 接口

| 操作 | Method / Path | 成功返回 |
| --- | --- | --- |
| 发布公共资源 | `POST /api/v1/admin/desktop/resources` | 200，`{"data": Resource}` |
| 读取列表 | `GET /api/desktop/v1/resources` | 200，`{"data": Resource[]}` |
| 读取当前详情 | `GET /api/desktop/v1/resources/{resource_id}` | 200，`{"data": Resource}` |
| 下载固定版本 | `GET /api/desktop/v1/resources/{resource_id}/versions/{version}/artifacts/{platform}` | 200，ZIP 二进制 |

接口随 `desktop.enabled` 开关启用。未启用时不注册资源路由。

### 3.1 发布

```http
POST /api/v1/admin/desktop/resources
Authorization: Bearer <admin-access-token>
Content-Type: application/zip

<ZIP binary>
```

每次上传一个资源、一个版本、一个平台。请求体是原始 ZIP，不是 JSON、base64 或 multipart。name、key、version 等元数据全部来自 ZIP 的 `manifest.json`，避免表单和包内元数据不一致。

服务器在事务内写入 Resource 与 Artifact 后才返回成功。**上传成功即发布并可读**，v1 没有草稿、审核、定时发布或下架状态。返回 Resource 的 `artifacts` 只包含本次上传的平台；查看全部平台需再调用详情。

同一公共 key 跨版本保留同一个 resource ID。`kind` 创建后不允许修改。重复上传相同 `(resource_id, version, platform)` 返回 409，即使字节完全相同也不覆盖。

上传超时不代表事务失败。发布工具不能把重试得到的 409 直接当作成功，应读取目录/详情并核对 key、版本、平台和 SHA-256；无法确认时展示“发布结果待核实”。v1 不提供 `Idempotency-Key` 支持。

### 3.2 列表与分页

```http
GET /api/desktop/v1/resources?scope=public&kind=mcp&search=%E5%9B%BE%E7%89%87&limit=20&offset=0
Authorization: Bearer <desktop-access-token>
```

| 参数 | 客户端约定 | 当前服务端行为 |
| --- | --- | --- |
| scope | `public` / `enterprise`，建议总是显式传入 | 缺省 public，其他值 422 |
| kind | 缺省或空字符串为全部；`mcp` / `skill` | 其他值 422 |
| search | UTF-8，最多 200 字节，需 URL 编码 | name / description 不区分大小写匹配；当前 PostgreSQL ILIKE 中 `%`、`_` 具有通配符含义 |
| limit | 1–100；Desktop 固定 20 | 缺省 50，超出范围/非数字回退到 50 |
| offset | 0–10000；下一页为已读取数量 | 缺省 0，超出范围 422；非数字当前解析为 0 |

按 `updated_at DESC, id ASC` 排序，以资源为分页单位，多平台不会占多个列表行。当前版本的全部已发布平台放在 `artifacts` 中。空列表为 `{"data": []}`，不是 null，也不是错误。

v1 没有 total / nextCursor / hasMore。客户端收到不足 limit 项即可停止加载；恰好 limit 项时可继续请求下一页。发布或更新可能改变排序，v1 不保证跨页快照一致；刷新时重置 offset 和旧列表。

### 3.3 详情与下载

详情返回**当前版本**的全部平台。安装前重新读取详情，选择匹配本机系统和目标客户端的制品，并把资源 ID、版本、平台、SHA-256 固定在此次预览中。安装确认时按该固定版本下载，不静默切换到新版本。

下载路径是受鉴权保护的 API，不返回存储桶地址、本地服务器路径或长期公开链接。历史版本可按固定版本路径下载。不存在、版本不存在、平台不存在或无权限均为 404。

```http
HTTP/1.1 200 OK
Content-Type: application/zip
Cache-Control: private, no-store
X-Content-Type-Options: nosniff
Content-Disposition: attachment; filename="resource.zip"
X-Artifact-SHA256: <64 lowercase hex characters>

<ZIP binary>
```

Rust 校验下载字节数和目录中记录的 SHA-256，再校验包内 manifest 与每个文件的哈希；下载 Header 不能替代目录中的预期哈希。不跟随重定向，不把 User Token 发给存储服务。v1 不承诺 Range、断点续传、ETag 或 304。目录/详情使用 `Cache-Control: no-store`。

## 4. 数据模型

Resource、Artifact、Manifest 字段使用 camelCase；既有错误信封仍使用 `request_id`。具体 schema 与完整成功示例在 OpenAPI 文件中。

| 对象 | 字段 | 含义 |
| --- | --- | --- |
| Resource | id | 后端生成的 `res_` + 32 位小写十六进制，下载与安装的稳定标识 |
| Resource | key | 范围内唯一的资源标识，如 devku_image；不同企业可有同名 key |
| Resource | kind | mcp / skill |
| Resource | name / description / sourceUrl | 名称、描述、HTTPS 来源地址；sourceUrl 可为空 |
| Resource | version | 当前版本，数字 x.y.z |
| Resource | scope | public / enterprise，不返回其他企业 ID |
| Resource | artifacts | 当前版本可用的平台及安装清单 |
| Artifact | platform | darwin-arm64 / darwin-x64 / windows-x64 / linux-x64；Skill 为 any |
| Artifact | sha256 / sizeBytes | 原始 ZIP 的 SHA-256 和字节数 |
| Artifact | manifest | 与包内 JSON 语义一致的安装清单；可选空字段可能被后端省略 |

`Artifact.manifest.key/kind/version` 必须与 Resource 一致，`manifest.platform` 必须与 Artifact 一致。无匹配平台或 targets 不包含选定客户端时不可安装；不能用其他架构的包兜底。

## 5. ZIP 与 Manifest v1

示例：

```json
{
  "schemaVersion": 1,
  "key": "devku_image",
  "kind": "mcp",
  "name": "Devku 图片生成",
  "description": "通过企业网关生成图片",
  "sourceUrl": "https://github.com/hiccup711/devku_imagegen",
  "version": "0.1.0",
  "platform": "darwin-arm64",
  "entry": "bin/devku-image-mcp",
  "args": ["--mcp-server", "devku_image", "--binding", "{binding}"],
  "credentialMode": "enterprise_model",
  "targets": ["workbuddy", "chatgpt_codex"],
  "files": {
    "bin/devku-image-mcp": "<该文件的 SHA-256>",
    "LICENSE.txt": "<该文件的 SHA-256>"
  }
}
```

| 字段 | 规则 |
| --- | --- |
| schemaVersion | 固定 1；未知字段/版本拒绝安装，不能静默猜测 |
| key | `^[a-z][a-z0-9_-]{1,63}$` |
| kind | mcp 或 skill |
| name | 必填，去空白后非空，最多 200 UTF-8 字节 |
| description / sourceUrl | 发布者应显式提供，允许空字符串；分别最多 2000 / 2048 UTF-8 字节 |
| version | 三段非负整数，每段最多 6 位，无多余前导零，无预发布/构建后缀 |
| entry | 包内相对路径；必须存在于 files |
| args | 可选，最多 32 项，每项最多 1000 UTF-8 字节，不含 NUL |
| credentialMode | 省略/空表示不需要企业模型绑定；enterprise_model 表示复用本机已部署企业模型 |
| targets | 1–2 项，workbuddy / chatgpt_codex，不重复 |
| files | 包内相对文件路径 → 64 位小写 SHA-256，不包含 manifest.json 自身 |

ZIP 包含根目录 `manifest.json` 和运行所需的全部文件，不能在安装时执行在线依赖安装。MCP 的 entry 作为 stdio 子进程启动，args 逐项传递，不经过 shell；`{binding}` 替换为 Rust 生成的不含 Token 的绑定文件路径。没有 enterprise_model 时模块不得依赖企业绑定字段。

Skill 必须使用 `platform=any`、`entry=skill/SKILL.md`，所有文件均在 `skill/` 下，args / credentialMode 必须为空或省略。Skill 内容遵循目标客户端 SKILL.md 格式；v1 云端校验包结构与哈希，不校验技能语义或行为。当前仅启用 Codex Skill 自动安装。

ZIP ≤64 MiB，单文件 ≤64 MiB，展开总量 ≤128 MiB，≤1024 项（含目录及 manifest），manifest ≤128 KiB。只接收普通文件和目录，禁止路径穿越、绝对路径、符号链接、大小写重名和 Windows 保留名称。每个文件都必须被 files 覆盖，哈希错误或额外文件都拒绝。

OpenAPI JSON Schema 的 maxLength 以字符计；Go 服务实际字节限制以本表为准。Schema 覆盖字段结构，ZIP 内容、跨字段对应关系和业务版本约束由服务器额外验证。

## 6. 版本发布规则

- `(scope, key)` 唯一；企业 scope 的唯一性还包含当前企业 ID。
- `(resource_id, version, platform)` 不可变；更新需递增版本。
- 同版本多平台包的 kind、name、description、sourceUrl 必须一致；entry、args、targets 和文件内容允许因平台不同而不同。
- 当前实现按包发布：例如从 0.1.0 升至 0.2.0，首个 0.2.0 包成功即切换 current version，尚未上传的其他平台会显示“暂无适用版本”。不自动混用旧平台包。
- 已安装的旧 MCP 继续使用本机文件；目录更新不自动修改配置。用户查看预览并确认后才安装新版本。
- v1 无服务端下架、删除、版本回退或批量原子发布；本机“移除”只是移除本机安装。多平台原子发布建议在下一版增加草稿/发布状态，不能把尚未实现的状态发给旧客户端。

## 7. 错误与重试

资源 Handler 与 Desktop 读取接口的错误信封：

```json
{"error":{"code":"RESOURCE_VERSION_CONFLICT","message":"resource version already exists or is older than the current version","request_id":"","retryable":false,"details":{}}}
```

| HTTP | code | 调用方处理 |
| --- | --- | --- |
| 400 | RESOURCE_PACKAGE_INVALID | 校验失败；修复包，不自动重试 |
| 401 | UNAUTHENTICATED | Desktop 刷新登录凭证后重试一次 |
| 403 | MEMBERSHIP_REVOKED | 清理本地登录态与预览，停止下载/安装 |
| 404 | RESOURCE_NOT_FOUND | 刷新目录，不提示资源属于哪个企业 |
| 409 | RESOURCE_VERSION_CONFLICT | 包不可覆盖、版本倒退或元数据冲突；核对后发布新版本 |
| 413 | PAYLOAD_TOO_LARGE | 压缩包过大，不重试 |
| 422 | VALIDATION_FAILED | 修正请求参数或请求体读取错误 |
| 5xx | 以服务端 code 为准 | 展示暂不可用；读取可重试，发布先核实结果 |

发布端沿用既有管理员中间件，鉴权/合规失败可能使用 `{"code": ..., "message": ...}` 而非 Desktop error 信封；例如 423 的 `ADMIN_COMPLIANCE_ACK_REQUIRED`。发布工具须按 HTTP 状态处理，并兼容这两类信封。代理或全局限流返回也不保证资源错误信封；不得将其当作成功。

Rust 下载/安装错误如 `RESOURCE_PACKAGE_INVALID`、`MCP_CONFIG_CHANGED`、`MCP_HANDSHAKE_FAILED` 是本机错误，不能混同云端 HTTP 成功。只下载成功不能展示安装成功。

## 8. WebView ↔ Rust 约定

| Tauri 命令 | 参数 | 返回 |
| --- | --- | --- |
| list_mcp_modules | scope, kind, search, offset | Resource 数组，补充本机 targets 状态；每页 20 项 |
| preview_mcp_module | moduleId（Resource.id）, target, operation（install/remove） | previewId, target, operation, configPath, gateway, changes |
| apply_mcp_module | previewId | message；成功前完成哈希校验、MCP 握手、配置写入校验或 Skill 文件校验 |
| test_mcp_image | moduleId, target | message；仅图片 MCP，用户明确确认后才生图 |

targets 状态包含 `target, installed, canInstall, canRemove, canTest, statusMessage`。安装预览五分钟有效、单次消费；配置变化需重新预览。安装包和两类 Token 均不经 WebView。生产 WebView 不直接 fetch 后端资源 API。

## 9. 后续兼容边界

企业员工上传共享、上传者管理、审核、资源下架、远程 HTTP MCP、配置表单、安装版本展示与游标分页均未纳入此版。不在前端假设这些字段或调用未实现的路由。后续企业空间可复用 Resource / Artifact 及下载路径，但上传权限与归属必须另立契约。

HTTP v1 响应可兼容增加非必填展示字段。Manifest v1 在后端和 Rust 中均拒绝未知字段；扩展安装协议需新 schemaVersion 和支持它的 Desktop 版本，不可仅添加字段后直接发布。

## 10. 联调顺序

1. 后端启用 Desktop 并完成迁移；上传前用 `go run ./cmd/desktop-resource-check /path/to/resource.zip` 校验。
2. 用管理员身份上传独立图片 ZIP，取得 resource ID、版本、平台和 SHA-256。
3. 用员工身份读取公共列表及详情，核对 metadata 与上传结果；当前无企业发布数据时企业列表应为空。
4. 下载固定版本，核对字节数和 SHA-256；改用其他企业身份请求专属资源应返回 404。
5. Desktop 在页内预览、确认安装，检查真实 MCP 握手及配置；安装期间不得发送生图请求。
6. 单独确认测试生图，使用企业 Model Token 调用企业网关。重复发布同版本/平台应 409；升级新版本后重新预览安装。

已有后端服务/路由测试、隔离 PostgreSQL 测试及 Rust 下载到子进程握手测试覆盖关键链路；真实环境发布和收费图片请求需在发布联调时执行。
