# 微信扫一扫加入企业：Web 与小程序完整交接

日期：2026-10-17。后端和 Web 管理端已在本地实现，尚未部署；小程序由小程序前端工程继续接入。本文取代此前的建议稿，接口以本次更新的三份 OpenAPI 为准。

目标流程：**微信扫一扫 → 打开小程序 → 完成微信登录 → 查看企业邀请并主动确认 → 进入目标企业**。小程序内移除“扫码加入企业”，保留“我的企业”和“手动输入邀请码”。

## 产品约定与实现边界

- 使用微信官方小程序码，业务凭证通过 `query.scene` 传入，不使用 URL Scheme。
- 保留主动确认。扫码、打开页面、登录、预览及页面重新显示均不能自动领取；只有点击“确认加入”才发送 `confirm_join: true`。
- 微信身份是独立 BIUser。加入企业不会绑定后台管理账号，也不会创建 Desktop 员工身份。预设姓名只是企业内称呼，不校验领取者身份。
- 默认 24 小时、单人一次性邀请，以及企业状态、创建者权限、身份状态、撤销和重复领取规则继续有效。微信码长期可识别不代表业务邀请长期有效。
- 一个邀请对应两种凭证：原完整 `token` 用于旧客户端和手动输入；新 `scene` 用于微信小程序码。两者共享同一条邀请和领取事务，不能被不同用户各领一次。
- 本地代码及模拟微信测试通过不代表已发布小程序、真实 AppID 出码或真机扫码已验收。

## 后端已经实现

| 能力 | 约定 |
| --- | --- |
| 创建邀请 | 原响应增加 `scene`；`token` 和旧 `path` 保留 |
| 扫码凭证 | 24 字节密码学随机数，Base64URL 编码后固定 32 字符，区分大小写 |
| 存储 | migration 265 增加可空的 `scene_hash` 和唯一索引，仅保存 SHA-256 摘要 |
| 预览与领取 | 请求 JSON 中 `token` 或 `scene` 二选一，复用原接口、认证及授权事务 |
| 微信出码 | 新增管理端 POST 接口；服务端调用 `getwxacodeunlimit` |
| 微信接口凭据 | 使用现有 BI AppID/AppSecret 获取 stable access token；进程内缓存、并发刷新合并、过期提前刷新 |
| 凭据失效恢复 | 遇到微信 40001／40014／42001 时重新获取凭据并最多重试一次；采用普通刷新模式，不强制使其他实例凭据失效 |
| 图片边界 | 校验真实 PNG/JPEG、尺寸和大小；上游错误、非图片及异常响应返回受控错误 |

旧邀请记录的 `scene_hash` 为 NULL，原邀请码仍可使用；不会自动补发短凭证。新建邀请的 `token`、`scene` 仅创建响应返回，列表不会回传，服务端不能从摘要还原。页面刷新后如已丢失凭证，只能使用此前保存的码／邀请码，或撤销待领取邀请并重新创建。

## Web 管理端接入

以下 Web 行为已实现，平台管理员与企业负责人共用同一套邀请结果组件：

- 创建成功自动请求 `release` 小程序码，显示企业名称、邀请有效期、出码加载状态；图片加载完成后可保存 PNG/JPEG。
- 展示完整邀请码，复制按钮只复制 `token`，不混入旧 `path` 或 `scene`；页面提示接收者使用微信扫一扫，登录后主动确认。
- 出码失败保留邀请及手动输入备用方式，可重新生成同一邀请的小程序码；不会重新创建邀请。验证取消后允许重试；429 按响应头倒计时，无可用响应头时等待 60 秒。
- 本地有效期到达或出码返回 404 时隐藏当前凭证；切换企业和离开结果页使旧请求结果失效并清除内存凭证。列表刷新更新邀请状态，但结果页不会后台轮询领取状态，最终有效性由后端预览／领取校验。
- 历史列表提示凭证不能找回；旧后端创建结果若没有 `scene`，只保留邀请码，不伪造普通二维码。

凭证仅保存在当前组件内存中。完整刷新或离开邀请结果页后，旧结果不再可查看；历史列表不提供重新查看凭证的入口。

### Web 页面路由

| 页面 | 平台管理员 | 企业负责人 |
| --- | --- | --- |
| 访问管理列表 | `/admin/desktop/organizations/:organizationId/bi` | `/desktop/organization/bi` |
| 创建邀请 | 上述路径 + `/invitations/new` | 同左 |
| 编辑使用者权限 | 上述路径 + `/users/:biUserId/edit` | 同左 |
| 本次邀请结果 | 上述路径 + `/invitations/:invitationId` | 同左 |

运营页和企业详情页均导航进入独立访问列表。创建成功以 replace 进入结果页，避免后退重新提交；编辑成功、取消均返回访问列表，列表的返回入口再回原运营标签页或企业详情。未保存修改离开、刷新时提示；保存请求进行中阻止页面跳转。冲突不清空草稿，可明确放弃草稿后重新加载最新权限。

邀请结果只在当前页面会话的内存中传递，URL 只有邀请 ID，不带 token／scene；刷新或离开后再打开结果链接只显示凭证无法找回的说明，不偷偷新建邀请。直接打开编辑链接会从当前授权列表按用户 ID 查找，包含翻页，不依赖前一页传对象。权限和企业归属仍由服务端校验。

### 1. 创建邀请

两类管理端沿用各自已有前缀：

| 使用者 | 管理前缀 |
| --- | --- |
| 平台管理员 | `/api/v1/admin/desktop/operations/organizations/{organization_id}` |
| 企业负责人 | `/api/v1/desktop/organization/bi` |

向 `<管理前缀>/invitations` 发送现有创建请求。需要 Web Bearer 和现有 step-up 验证。返回仍为 `{ code, message, data }`，`data` 包含完整 Invitation，并增加以下字段：

```json
{
  "token": "bii_<完整原邀请码>",
  "path": "/pages/settings/index?invite=bii_<完整原邀请码>",
  "scene": "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef"
}
```

上述值仅演示格式，不是真实邀请。`scene` 的校验表达式为 `^[A-Za-z0-9_-]{32}$`。不能截断 `token`、自行生成 `scene` 或使用企业 ID 代替。

### 2. 生成官方小程序码

```http
POST <管理前缀>/invitations/{invitation_id}/qrcode
Authorization: Bearer <Web access token>
Content-Type: application/json
```

```json
{
  "scene": "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef",
  "env_version": "release"
}
```

`invitation_id` 使用创建响应的 `id`，不是邀请码。`scene` 必填；`env_version` 可省略，默认 `release`，只接受 `release`、`trial`、`develop`。未知请求字段会被拒绝。正常业务 UI 使用 `release`，不需要给最终用户增加环境选择。

后端校验当前操作者是有效平台管理员或本企业负责人，并校验邀请属于该企业、短凭证匹配、邀请待领取且未过期、创建者仍有管理权限。微信请求返回后会再次核实状态，避免期间发生撤销却仍返回图片。该接口同样要求 step-up，并实施每 IP 总计 600 次／分钟、该出码操作每 IP 120 次／分钟的限流。

成功返回：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "image_base64": "<图片 Base64，不含 data: 前缀>",
    "mime_type": "image/png",
    "page": "pages/invitation/index",
    "env_version": "release",
    "expires_at": "<邀请到期时间，RFC3339>"
  }
}
```

`mime_type` 可能为 `image/png` 或 `image/jpeg`。Web 可以使用 `data:${mime_type};base64,${image_base64}` 展示图片。使用现有 HTTP 客户端、Web Bearer 和 step-up 流程发 POST；不要把凭证放进图片 GET URL，也不要直接请求微信 API。

响应带 `Cache-Control: private, no-store`。图片本身携带领取能力，不写日志、公共缓存、分析事件或浏览器持久化存储。现有邀请码及图片由管理员自行分发，后端不自动发送消息。

### 3. 创建和出码分别管理状态

创建成功后先保存本次创建结果，再请求出码。出码失败时保留结果并提供“重新生成小程序码”，只重试 qrcode 请求，不重复创建邀请。step-up 过期时完成既有验证后继续同一请求；429 按 `Retry-After` 等待。

图片加载成功后提示：

> 使用微信扫一扫，登录后确认加入企业。

已删除 `BIGrantManagement.vue` 对邀请路径的 `QRCode.toString()` 调用及“小程序设置页扫码领取”文案，由 `BIInvitationResult.vue` 展示后端官方图片，保留复制完整邀请码作为备用方式。原 `path` 只为兼容旧客户端，不是可直接由微信扫一扫打开的普通二维码内容。

若刷新页面丢失了 `scene`，列表中的邀请 ID 无法重新取码；不要偷偷创建新邀请。明确提示使用已保存的邀请，或撤销旧邀请后重新创建。创建／编辑表单已按项目规范迁移到独立路由页面。

## 小程序接口

### 1. 登录

复用 `/api/bi/v1/bootstrap`、`POST /api/bi/v1/auth/wechat` 及现有版本检查，微信 code 兑换得到的 Bearer 用于以下接口。无需改变登录请求或索取新的微信个人资料。

### 2. 预览邀请

```http
POST /api/bi/v1/invitations/preview
Authorization: Bearer <小程序 access token>
Content-Type: application/json
```

扫码入口发送：

```json
{ "scene": "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef" }
```

手动输入或旧路径发送：

```json
{ "token": "bii_<完整原邀请码>" }
```

只能发送其中一个字段，不要同时传另一字段的空串或 null。移动接口返回原始 Invitation DTO，没有管理端 `{ code, message, data }` 包装；主要字段为 `id`、`organization_id`、`organization_name`、`recipient_name`、`capabilities`、`status`、`expires_at`、`claimed_by`。预览不消费邀请，也不返回扫码凭证。

### 3. 主动确认领取

```http
POST /api/bi/v1/invitations/accept
Authorization: Bearer <小程序 access token>
Content-Type: application/json
```

```json
{
  "scene": "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef",
  "confirm_join": true
}
```

手动入口把 `scene` 替换为 `token`。缺少确认或为 false 返回 400。成功返回原始 Access DTO，使用返回的 `organization_id` 刷新授权、选择目标企业、进入首页，不按企业列表第一项选择。

邀请及企业授权状态仍在领取事务中重新检查，不能依赖预览结果。已由当前用户领取的邀请可重复读取／领取，但撤销后的企业权限不会恢复；其他用户不能领取同一邀请。已有目标企业授权时沿用当前授权，不扩权，不覆盖称呼。

## 小程序页面与入口

### 企业与设置

保留企业列表、企业切换、现有权限展示和登录退出功能。移除“扫码加入企业”“扫码领取”按钮、扫码／手动模式切换，以及企业邀请相关 `Taro.scanCode()` 分支。

没有企业时展示：

> 暂未加入企业
>
> 请使用微信扫一扫，扫描管理员提供的企业邀请小程序码。

保留次要入口“手动输入邀请码”；已有企业的用户也可以用它加入其他企业。入口导航到独立邀请页面，不在弹窗或抽屉中放输入表单。

### 独立邀请页

**必须注册 `pages/invitation/index`，与后端固定的出码 page 一致。** 微信参数不带前导 `/` 或查询串；小程序内部导航可使用 `/pages/invitation/index`。

- 扫码入口：接收 `query.scene`，完成登录后自动只读预览，展示确认区域，不要求再输入或再扫码。
- 手动入口：显示邀请码输入框，“查看邀请”执行校验及预览；支持现有完整邀请码和带 `invite` 参数的小程序路径。保留旧设置页 `invite` 路径，转交新邀请页。
- 确认区域：展示服务端企业名称、企业内称呼（如有）、授权内容、有效期；提供“确认加入”和“暂不加入”。修改输入后清除旧预览。
- 已有访问：核实当前目标企业权限后显示“进入企业”，不必重复领取。权限被撤销时不能据旧列表进入。
- 取消：返回来源页；微信直接打开、没有页面栈时回首页，不消费邀请。有未提交的手动输入时处理离开提示。
- 成功：刷新授权并选中目标企业后进入首页；`claimInvitation()` 的 `readable` 为 false 时应展示已加入但内容加载失败，不能当作领取失败。

当前 `InvitationPanel` 点击提交后直接调用领取，尚无独立预览页面。必须补齐上述确认流程，不能只把 `Taro.scanCode()` 换成读取参数。

上述小程序现状来自本次交接前的代码核查；小程序工程独立修改后，以其最新代码为准。Web 访问管理、创建邀请、编辑权限和邀请结果已全部改为独立路由，业务表单不再放在弹窗中。

## 启动参数与会话处理

1. 同时覆盖冷启动和小程序在后台时再次扫码；统一分发参数，不只在 App 首次挂载时读取一次。
2. 读取 `query.scene`，不要使用微信启动参数顶层的数字 `scene`（来源场景值）。按微信约定解码一次；结合 Taro 实际参数避免重复解码，并捕获非法编码。
3. 新 scene 严格符合固定 32 字符 Base64URL 格式，旧 `bii_...` 用原规则解析。保留大小写，不截断、不补前缀；新旧参数同时出现时拒绝。
4. 邀请参数只在内存中保留，跨越初始化及登录步骤，不写 Storage、日志或分析事件。取消或主动退出时清除。
5. 用户曾主动退出时保留“微信登录”按钮，点击后继续待处理邀请；不绕过退出标记。登录失败可重试，成功后回到该邀请。
6. 预览和领取必须等待有效 Bearer。新扫码、身份变化或取消使旧预览和在途响应失效；复用请求代次校验，防止旧结果覆盖新邀请。
7. 重复生命周期回调不发送领取请求；提交时禁用按钮，只有本次用户主动确认可发送 mutation。

### 请求类型与页面状态建议

在小程序 API 层使用互斥类型，预览和领取共享同一份内存凭证，避免组装出两个字段：

```ts
type InvitationCredential =
  | { scene: string; token?: never }
  | { token: string; scene?: never }

type AcceptInvitationBody = InvitationCredential & { confirm_join: true }

// 扫码后的只读预览；沿用已有小程序 HTTP 客户端和响应 DTO。
await previewInvitation(credential)

// 仅放在用户点击“确认加入”的处理器中，不放进 onShow/useEffect。
const body: AcceptInvitationBody = { ...credential, confirm_join: true }
const access = await acceptInvitation(body)
// 刷新当前授权，并按 access.organization_id 选择企业，再进入首页。
```

代码仅说明请求契约，不要求照搬函数名。页面至少区分“等待登录／输入邀请码／预览中／待确认／提交中／已加入／不可用／可重试错误”。新凭证进入时先清空旧预览和错误，再进行只读预览。网络失败可重试预览；领取结果未知按下表先只读核实。登录、预览、领取任一步失败都不能自动切到企业列表第一家。

## 错误与恢复

业务错误沿用 BI Error：`code`、`message`、`request_id`、`retryable`、`details` 等；管理端失败也不要按成功包装解析。认证和 step-up 中间件的响应沿用现有客户端处理。

| HTTP / 情况 | 处理 |
| --- | --- |
| 400 `INVALID_ARGUMENT` | 短凭证格式、双凭证、环境参数或主动确认不合法；不自动重试 |
| 401 | 登录／会话失效；按原登录机制恢复，不在旧身份下继续操作 |
| 403 / 404 | 无权访问或邀请不可用；不要恢复已撤销权限 |
| 404 `NOT_FOUND` | 可包括不存在、过期、撤销、其他人领取、企业／创建者失效或出码归属不匹配；统一提示“邀请不可用，请联系管理员重新获取”，不猜测具体原因 |
| 423 | Web 出码或创建需要 step-up；用现有验证流程恢复 |
| 429 `RATE_LIMITED` | 按 `Retry-After` 等待，不连续重试；移动邀请仍沿用每身份 30 次／分钟的限流 |
| 503 `UPSTREAM_UNAVAILABLE` | 微信登录／出码不可用；出码失败保留已创建邀请，只重试出码 |
| 503 `DATA_UNAVAILABLE` | 后端数据服务不可用，显示可重试提示 |
| 领取超时或 5xx、结果未知 | 只读核实是否已由当前身份领取及当前权限，不自动再发 accept |
| 已加入但企业内容读取失败 | 提示“已加入，企业内容加载失败”，只重试加载 |
| H5 打开 | 提示在微信小程序中打开，不模拟登录或请求生产；测试夹具不进入发布包 |

官方出码错误不会透传微信 `errmsg`、AppSecret 或 access token。开发环境排查时核对 AppID、服务端出口 IP 白名单、页面路径、版本和配额；生产密钥只由服务端持有。

## 修改位置与协作顺序

| 工程 / 文件 | 工作 |
| --- | --- |
| Web `frontend/src/api/bi.ts` | 已完成：创建响应 scene、出码请求及图片 DTO |
| Web `frontend/src/api/client.ts` | 已完成：结构化错误保留 Retry-After |
| Web `frontend/src/components/bi/BIGrantManagement.vue` | 已完成：独立访问列表、创建／编辑路由入口、撤销确认和历史凭证说明 |
| Web `frontend/src/router/biRoutes.ts`、`frontend/src/views/admin/BIWorkspaceView.vue` | 已完成：管理员／负责人独立路由、企业上下文与来源页返回、结果凭证仅内存保留 |
| Web `frontend/src/components/bi/BIGrantForm.vue`、`BIInvitationDetails.vue` | 已完成：创建／编辑独立表单、未保存离开保护、成功跳转结果页、刷新后凭证遗失说明 |
| Web `frontend/src/components/bi/BIInvitationResult.vue` | 已完成：官方图片展示／保存、复制邀请码、加载／失败／重试、验证／限流和过期处理 |
| 小程序 `src/app.config.ts` | 注册 `pages/invitation/index` |
| 小程序 `src/pages/invitation/index.tsx`（新增） | 承接扫码、手动输入、预览确认、取消和恢复 |
| 小程序 `src/components/native/settings.tsx` | 移除内部扫码、保留手动入口和企业列表、兼容旧 invite 路径 |
| 小程序 `src/app.tsx` 与页面生命周期 | 冷／热启动参数统一分发和去重 |
| 小程序 `src/domain/invitations.ts` | 区分 scene 与 token，解析并拒绝非法参数 |
| 小程序 `src/api/bi.ts` | 请求体二选一，主动确认领取 |
| 小程序 `src/domain/nativeState.ts` | 复用登录、会话代次、未知结果核实和目标企业选择 |

Web 已按正式版接好。小程序先完成落地页，在开发／体验版验证后，再协调正式切换。在 `trial`／`develop` 环境，后端出码使用 `check_path=false`，仍需在对应版本实际注册页面；正式版使用 `check_path=true`，页面必须已经发布。正式切换入口时协调后端迁移、Web 和小程序版本，旧完整邀请码继续可用。本文不授权上传、审核、发布或部署。

### 本地测试与正式小程序码

本地后端可以调用微信生成 `release` 正式小程序码，生成位置并不决定小程序版本。但须同时满足：

1. 运行包含本次改动的后端、完成 migration 265，启用 BI 并配置真实小程序 AppID/AppSecret；出口 IP 符合微信平台要求。
2. 同一 AppID 的正式小程序已发布 `pages/invitation/index`。只在开发工具新增页面不满足正式版 `check_path=true`。
3. 扫码打开的小程序所用 API 能访问创建邀请的同一环境、同一份邀请数据；本地库创建的邀请不能拿到生产库领取。真机的 localhost 也不是开发电脑。

2026-10-17 本地后端 `127.0.0.1:18080` 已重启并加载 BI 配置，公开设置为 `bi_enabled:true`，bootstrap 返回 200。真实微信凭据获取成功；正式出码返回 `41030 invalid page`，需确认同一 AppID 的正式版已发布 `pages/invitation/index`。此状态不代表正式码已验收。端口 8080 属于其他项目，不能作为本项目默认联调后端。

小程序尚未正式发布页面时，可用 API 手动指定 `trial`／`develop` 联调相应版本，测试账号需满足微信对应版本的访问权限；Web 业务页面固定使用 `release`，没有版本选择器。测试时分别记录小程序版本、AppID 对应关系、API 地址和邀请所属数据库，不在记录中写密钥或真实凭证。

## 验证与验收

小程序修改后在 `devku-mobile/miniprogram/` 核实 Node/npm 路径和版本，执行：

```bash
npm run typecheck
npm test
npm run build:weapp
npm run build:h5
```

当前小程序没有独立 lint script，不把 typecheck 当作 lint。H5 做页面与导航检查，微信开发者工具做参数模拟，真机验证微信扫一扫和完整链路；分别记录。

Web 修改按 [开发指南](../development/DEV_GUIDE.md) 运行 typecheck、tests、lint:check 和 build。验收至少覆盖：

- 微信冷／热启动，登录前后邀请不丢失；取消和退出清除待处理参数。
- 预览不消费邀请；只有主动确认发送 accept，重复点击不重复提交。
- 单企业、多企业、重复扫描已领取邀请均定位正确目标，不展示旧企业缓存。
- 新 scene、完整邀请码和旧路径兼容；非法编码、超长值、双凭证被拒绝。
- 邀请过期／撤销／被他人领取、企业不可用、身份禁用、访问撤销、限流均正确处理。
- 出码失败不重复创建邀请；凭证遗失不凭邀请 ID 取回或偷偷重建。
- 领取结果未知只读核实；已加入但内容读取失败可单独恢复。
- 小程序企业邀请入口不再调用相机扫码；手动输入使用独立路由。
- 真机分别记录正式／体验版、首次登录、已登录及主动退出后进入的表现。

### 本次后端验证记录（2026-10-17）

在 backend 目录实际使用 Go 1.27.2（go.mod 自动选择的工具链）、golangci-lint 2.14.0（以 Go 1.27.2 构建）及仓库 `.golangci.yml`；隔离 PostgreSQL 由 testcontainers 启动。Node 为 v24.20.0，仅用于实际小程序 TypeScript 测试客户端；本次没有执行 Web 前端构建。

| 检查 | 结果 |
| --- | --- |
| 全量 `env -u OPENAI_API_KEY go test -tags=unit ./...` | 通过，60 个有测试的 package 成功 |
| 全量 `go test -tags=integration ./...`，下述严格联调环境 | 最终通过，55 个有测试的 package 成功 |
| 实际小程序 HTTP 与响应 schema | 严格模式通过，未跳过；验证现有客户端 token 链路兼容 |
| scene 新链路 | 真实 PostgreSQL + Go HTTP handlers 通过；覆盖摘要存储、双凭证／null 拒绝、主动确认、过期／撤销、旧邀请兼容、跨凭证并发领取、出码归属、失败重试和状态复核 |
| 微信 HTTP 适配 | 本地替身通过；覆盖并发 token 缓存、过期刷新、失效凭据单次重试、非图片／超大响应／上游失败处理 |
| 全量 `/Users/mac/go/bin/golangci-lint run --timeout=30m ./...` | 通过，0 issues |
| `go build ./...` | 通过 |
| 四份 OpenAPI、scene/token 请求互斥及文档链接 | 通过 |

全量单元套件有 18 项既有测试跳过，集成套件有 17 项既有测试跳过：包括需显式启用的外部 TLS／内容审核／OpenAI 调用、独立 Prompt Audit Redis/PostgreSQL、插件包、会计浏览器夹具、Linux 抓包样本、历史 NULL 数据、已有占位测试，以及已标记 TODO 的 `TestConcurrencyCacheSuite/TestGetAccountsLoadBatch`。这些跳过不计为功能通过；本次新增测试和严格移动端 HTTP 测试均未跳过。

首轮全量集成测试曾因现有 Desktop 测试夹具短编码碰撞失败，未完成初始化的夹具又使分组数量断言失败；未更改业务代码或该夹具，重跑全量集成测试通过。该既有测试稳定性问题仍可能复现。

严格联调执行环境（命令从 backend 运行，实际使用 `-json` 留存输出后检查 pass/fail/skip）：

```sh
uv run --with jsonschema==4.25.1 --with PyYAML==6.0.3 -- \
  env -u OPENAI_API_KEY \
  BI_MOBILE_PROJECT=/Users/mac/Site/devku/devku-mobile/miniprogram \
  BI_REQUIRE_MOBILE_HTTP=1 BI_REQUIRE_CONTRACT_VALIDATION=1 \
  go test -tags=integration ./...
```

上述后端验证未覆盖：真实 AppID 的微信出码、微信真机扫一扫、小程序新入口和正式发布页面。迁移仅在隔离测试数据库执行，未部署或修改生产数据。当时的 TypeScript 客户端尚未支持 scene，不能把旧 token 客户端联调通过当作小程序新流程已完成。

### 官方码 Web 首轮验证记录（2026-10-17）

使用 Node v20.20.2 和 pnpm 9.15.9，与 CI 的 Node 20／pnpm 9 主版本一致。保留用户原有 lockfile 和 workspace 文件改动；现有 workspace 文件缺少 pnpm 9 所需的 packages 配置，本次命令使用 `pnpm --ignore-workspace --dir frontend ...`，未安装依赖或重写锁文件。

| 检查 | 结果 |
| --- | --- |
| `lint:check` | 全量通过 |
| `typecheck` | 通过 |
| `test:run` | 434 个测试文件、3361 项测试通过 |
| `build`（含 `check:i18n`） | 通过；构建仍提示已有较大分包和 Browserslist 数据陈旧 |
| 官方码组件与 API 测试 | 覆盖两个管理前缀、release 请求、PNG/JPEG、复制 token、失败重试、429 等待、验证与取消、过期／撤销、旧邀请无 scene、切换企业、迟到验证错误和列表刷新不重复出码 |
| 浏览器交互 | Playwright + Chromium，在独立测试页加载真实 `BIGrantManagement` 组件、生产样式和中文文案，以模拟接口检查创建 → 出码失败 → 手动重试 → 图片／保存入口；创建请求 1 次、同一邀请出码 2 次，剪贴板只有完整 token，保存文件名为 `wechat-invitation.png` |
| 桌面与窄屏 | 1440×1000、390×844；页面非空、无框架错误遮罩、无横向溢出，文案和按钮可用；最终页面控制台 0 errors／0 warnings |

浏览器入口为临时 `http://127.0.0.1:3429/__bi-invite-qa`，只作组件交互检查。测试使用明确标注“不可扫码”的图片夹具，不调用微信、不创建真实邀请；未验证真实管理账号认证、微信出码或真机扫码。lint 首次与 Vitest 启动并行时遇到临时配置文件删除竞争，串行重跑全量 lint 通过；以上表格记录最终版本结果。

小程序侧接好后仍需按上面的验收清单完成真实后端、微信开发者工具和真机联调。当前 Web 固定请求正式版，正式页面未发布时应看到可重试的出码错误，不能用前端普通二维码兜底冒充官方码。

### 独立路由改造验证记录（2026-10-17）

沿用上述 Node／pnpm 版本及 `--ignore-workspace` 运行方式，本轮只调整 Web 页面与交接文档，没有修改后端代码。

| 检查 | 结果 |
| --- | --- |
| `lint:check` | 全量通过 |
| `typecheck` | 通过 |
| `test:run` | 435 个测试文件、3368 项测试通过 |
| `build`（含 `check:i18n`） | 通过；保留已有大分包和 Browserslist 提示 |
| 路由与表单测试 | 覆盖管理员／负责人入口、直接打开编辑页与分页查找、权限冲突保留草稿、安全验证取消与重试、未保存离开保护、成功跳转与凭证清除、直接打开结果链接 |
| 浏览器交互 | 在 `http://127.0.0.1:3417` 登录真实本地管理员，验证运营入口 → 访问列表 → 创建页 → 结果页；取消时选择留下保留草稿；出码失败重试共创建 1 次、出码 2 次；编辑保存 1 次后返回列表并显示更新值；刷新结果页显示凭证无法找回的说明 |
| 桌面与窄屏 | 1440×1000、390×844 检查，无业务表单弹窗、无横向溢出、无框架错误遮罩 |

浏览器测试使用真实本地认证和企业读取，邀请创建、权限更新及列表使用明确标注的模拟数据，没有创建真实邀请或修改真实用户权限。出码故意返回 503 以检查重试，控制台的两条错误均为这两次预期 HTTP 503，没有额外运行时错误。微信真实出码与手机扫码仍需按前述环境要求联调。

## 契约与依据

- [小程序 OpenAPI](openapi.yaml)、[平台管理 OpenAPI](admin.openapi.json)、[企业负责人 OpenAPI](desktop.openapi.yaml)
- [身份与邀请协议](protocol.md)、[本地联调](integration-runbook.md)
- [微信官方：无限量小程序码与 scene 限制](https://developers.weixin.qq.com/miniprogram/dev/server/API/qrcode-link/qr-code/api_getunlimitedqrcode.html)
- [微信官方：稳定版接口调用凭据](https://developers.weixin.qq.com/miniprogram/dev/server/API/mp-access-token/api_getstableaccesstoken.html)
