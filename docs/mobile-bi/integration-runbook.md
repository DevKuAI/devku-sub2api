# BI 测试企业与联调操作说明

面向后端、数据源适配和移动端实施人员，适用于当前 API v1.1.0 实现。按以下顺序建立受控测试企业并记录证据。本文中的配置和操作步骤尚未在真实微信或外部生产者环境执行；实际验收状态见 [验收进度](./acceptance-progress.md)。

## 1. 准备测试环境

实施人员需要提供测试服务地址、微信 AppID、合法 HTTPS request 域名、隐私说明 URL、企业管理账号和测试数据来源。完整协议、PRD、数据保留期限及容量阈值仍以业务方提供的材料为准。

使用独立的测试 PostgreSQL/Redis 和脱敏业务数据。通过现有 sub2api 启动流程运行迁移，确认 `242_bi_identity.sql` 至 `248_bi_daily_usage.sql` 已应用。服务的 BI worker 与服务进程一起启动，无需另外启动 `bi-connector` worker。

在部署使用的配置中设置以下字段。环境变量由相应配置键的大写形式构成，点替换为下划线，例如 `bi.app_secret` 对应 `BI_APP_SECRET`。

四套 Compose 模板已透传全部 9 个 BI 配置项，完整字段、默认值与校验见 [配置参考](./configuration.md)。

| 配置 | 要求 |
| --- | --- |
| `bi.enabled` | 测试环境完成配置后设为 `true`；默认 `false` |
| `bi.appid`、`bi.app_secret` | 同一个测试小程序的服务端凭证 |
| `bi.jwt_secret`、`bi.identity_secret` | 各自独立的标准 base64 密钥，每份至少 32 字节；不能复用网页或 Desktop 密钥 |
| 后台 BI 隐私说明及站点地址 | 在「系统设置 → 登录条款」填写独立的 HTTPS 站点地址、标题、版本和 Markdown 正文后发布；详见 [配置参考](./configuration.md#隐私说明后台维护) |
| `bi.min_client_version` | bootstrap 返回给客户端的最低版本，当前默认 `0.1.0` |
| `bi.report_retention_months` | 报告可读期限，默认 24 个自然月；不表示业务事实已执行物理删除 |
| `desktop.enabled` | 使用原站 Desktop 企业管理入口时须启用，并满足原 Desktop 配置要求 |

密钥、AppSecret 和接入 Token 放在环境配置或部署密钥管理中，不写入本目录。`identity_secret` 参与微信身份索引，不能把重生成密钥当作日常重启步骤。

启动后调用 `GET /api/bi/v1/bootstrap`。预期 `demo_available=false`、`binding_enabled=true`，并返回已发布文档的版本及 `https://<BI 隐私说明站点域名>/legal/bi-privacy`。未发布或 BI 站点地址无效时返回 `503 DATA_UNAVAILABLE`；先补齐后台设置。使用未登录浏览器打开返回地址，核对正文和版本；修改正文并更换版本发布后再次调用 bootstrap，确认无需重启即可读到新版本。BI 未启用时不会注册这些路由。

## 2. 建立企业与显式授权

1. 原站管理员通过既有登录、验证码和 2FA 流程进入 `/admin/desktop/organizations`。
2. 选择已批准的测试企业，或通过原企业管理入口创建测试企业。记录企业 `public_id`；BI 使用同一个 ID。企业必须处于 active 状态。现有 group 是模型/计费配置，不作为部门导入。
3. 在企业详情的「小程序企业授权」中，为真实测试管理账号添加授权。账号 ID 来自原站账号管理，不能使用 Desktop 员工 ID、手机号或模型 API Key 代替。
4. 设置角色、团队范围和功能 capability。首个经确认的测试 Owner 可使用 `org_admin`；角色名称不会自动赋予其他 capability。完整链路测试需要 `analytics:read`、`knowledge:read`、`sources:read`、`evaluations:read`、`members:read`、`reports:read`、`reports:create`、`reports:copy` 和 `reports:share`。
5. 为权限验收另设只读账号及受限团队账号。团队 ID 使用组织来源导入的稳定 ID。绑定微信本身不产生企业授权。

通过 API 维护授权时，使用原站管理员认证与响应 envelope：

以下 3 个操作的完整请求、响应及错误分支见 [原站授权管理 OpenAPI](./admin.openapi.json)，可与主 OpenAPI 分别导入 API 客户端。

| 操作 | 路径及要求 |
| --- | --- |
| 读取授权 | `GET /api/v1/admin/bi/organizations/{organization_id}/grants?page=1`，每页 50 条 |
| 新建/更新 | `PUT /api/v1/admin/bi/organizations/{organization_id}/grants`；首次 `expected_revision=0`，编辑须发送当前 revision |
| 撤销 | `POST /api/v1/admin/bi/organizations/{organization_id}/grants/{manager_id}/revoke`；请求体包含当前 `expected_revision` |

若返回 409，重新读取授权后再决定变更，不覆盖他人的新 revision。`all_teams=true` 时 `team_ids=[]`；受限账号显式列出已授权团队。

## 3. 完成微信绑定闭环

1. 移动端通过 `wx.login`（Taro 客户端使用对应封装）取得一次性 code，调用 `POST /api/bi/v1/auth/wechat`，提交 `code` 和 `client_version`。
2. 未绑定时，响应包含 `result=binding_required`、`binding_ticket`、8 位 `user_code`、到期时间和 `/bi/bind`。管理者在原站登录本人账号，打开该路径并确认小程序显示的绑定码。
3. 原站绑定接口要求新签发的网页 JWT：`iss=devku-sub2api`、`aud=sub2api-web`。旧网页 Token 仍可访问原有网页接口；绑定入口返回 401 时，重新完成原站登录或由原登录流程刷新 Token。
4. 移动端获得新的微信 code 后调用 `POST /api/bi/v1/auth/bindings/exchange`，提交 `binding_ticket` 和 `code`。尚未审批时返回 202/PendingBinding，按 `retry_after_seconds` 等待；审批后返回小程序会话。票据过期时重新开始绑定流程。
5. 使用返回的 MobileBearer 调用 `GET /api/bi/v1/me` 和 `GET /api/bi/v1/organizations`。企业列表只能包含显式授权的企业；没有授权时应为空。
6. 分别执行退出、撤销微信绑定、撤销企业授权、停用原账号。检查旧会话、Context、报告和分享的实际响应。企业撤权不等同于账号退出，记录对应的 401、403 或 404，不把这些状态混为一种成功结果。

当前 access Token 为 15 分钟，refresh 采用 12 小时活动续期；绑定挑战默认 10 分钟。精确协议仍需与缺失的 `protocol.md` 核对。refresh 成功后替换旧凭证，不让两个并发请求各自重复刷新。

## 4. 登记来源并导入

先完成第 2 节的授权，使 BI 企业映射存在。适配负责人为每个来源确定稳定 `source_id`、namespace、允许的记录类型，以及首次导入范围。来源 ID 表示生产者，`SourceRecord.id` 表示内容来源对象，两者用途不同。

运维侧配置 `BI_DATABASE_DSN`，并设置以下非敏感操作变量：`BI_ORGANIZATION_ID`、`BI_SOURCE_ID`、`BI_NAMESPACE`、`BI_ALLOWED_KINDS`。在仓库根目录执行：

```bash
cd backend
go run ./cmd/bi-connector -operation issue \
  -organization "$BI_ORGANIZATION_ID" \
  -source "$BI_SOURCE_ID" \
  -namespace "$BI_NAMESPACE" \
  -kinds "$BI_ALLOWED_KINDS" -ttl 24h
```

输出包含 `credential_id`、一次性展示的 `token` 及到期时间。将 Token 放入适配器的受控配置；数据库仅保存摘要。轮换复用原 source/namespace，不改变 checkpoint 或扩大允许的类型。

将 [OpenAPI](./openapi.yaml) 导入 API 客户端，配置测试服务地址和独立的 ConnectorBearer，按下列顺序调用：

| 顺序 | 操作 | 成功条件 |
| --- | --- | --- |
| 1 | `GET /api/bi/v1/internal/ingestion/sources/{source_id}/checkpoint` | 保存当前 checkpoint，首次为 null |
| 2 | `POST /api/bi/v1/internal/ingestion/batches` | 带 `Idempotency-Key` 和当前 `expected_checkpoint`；202 仅表示受理 |
| 3 | `GET /api/bi/v1/internal/ingestion/batches/{batch_id}` | 等待 applied 或 rejected；同 source 在途时不提交下一个新批次 |
| 4 | 再读 checkpoint | 只有 applied 后才前移，并关联已发布的 `data_revision` |

每批最多 500 条记录、2 MiB。首次导入的循环引用对象放在同一批，由服务端原子校验。导入 ID 必须使用登记 namespace；保持同一逻辑对象的 ID，修订时增加 revision。正常无新增事件可提交 0 条记录的水位心跳，但仍需新 checkpoint。

`complete_through` 是生产者声明的完整水位，不是最大事件时间。初次回灌尚不能确认完整性时使用 null，且 `initial_backfill_complete=false`；有证据覆盖历史后才声明完成。

### sub2api 日志适配

为该适配器登记允许 `usage` 的独立来源。配置 `BI_CONNECTOR_TOKEN` 和实际导入起点 `BI_USAGE_SINCE`（RFC3339）后，在 backend 目录执行：

```bash
go run ./cmd/bi-connector -operation sync-sub2api \
  -since "$BI_USAGE_SINCE" -limit 500
```

CLI 直接向数据库提交一个批次并输出任务 JSON，不启动处理 worker，也不输出 HTTP 202。服务端必须已启用 BI 并正常运行。按上表查询任务终态后再取下一批；输出 `status=idle` 表示当前范围没有新的可导入日志。

适配器按历史成员 API Key 关联确定企业归属。它不会提供完整失败请求、可信人员行为、部门、应用或场景归因；这些维度仍为 unknown/null，完整水位仍为 null。组织/Eligibility、知识/案例/评估需要实际生产者另行接入。

## 5. 失败与修订恢复

| 现象 | 操作与验证 |
| --- | --- |
| HTTP 响应丢失 | 使用原 key 和原 body 重试，得到原批次 ID；不生成新 key 重复推进 |
| `IMPORT_BUSY` | 查询已知在途批次至终态，再提交下一批 |
| `CHECKPOINT_CONFLICT` | 重新读取 checkpoint 并核对源游标覆盖，不能用字符串大小推断进度 |
| `IDEMPOTENCY_CONFLICT` | 核对 key 对应的原请求，不能用同一 key 发送不同 body |
| rejected | 读取 errors 与 `record_index`；批次错误 index 为 null。修复后使用新 key，仍从最后 applied checkpoint 开始 |
| 调用人员/时间修订被拒绝 | 同来源在同批修复或撤回失效的评价、引用和重试关系；跨来源先由关联记录拥有者撤回，再修订调用并补正确关联 |
| 发布失败后来源仍不可读 | ACL 拒绝覆盖层按设计继续生效；发布合法修复批次，不手工删除拒绝状态 |
| 服务进程中断 | 恢复已启用 BI 的服务，worker 在租约到期后接手；检查旧 checkpoint 未越过失败批次，恢复后只发布一个完整 revision |

记录 HTTP 状态、错误 code、`X-Request-ID`、batch ID、checkpoint 和 data revision。记录中排除 access/refresh Token、binding ticket、AppSecret 和原始 openid。

## 6. 查询、撤权与验收记录

1. 使用 MobileBearer 读取企业 `data-status`，核对每个来源的覆盖范围和缺失维度。
2. 创建 `analysis-contexts`，分别验证 `period=current`（本周截至昨日）、`week`（上周）和 `month`（上月）。本周对比上周相同数量的完整自然日；周一为 `not_observable`。
3. 使用同一个 Context 读取总览、采用、Token、趋势、团队/成员/应用/场景，按 [验收清单](./acceptance.md) A01–A16 对账。区分期末人员母体和事件主团队，保留 unknown 及 non-cohort 解释。
4. 读取知识版本和来源时，使用真实 `parent_kind`、`parent_id`；分别撤销父对象与来源权限，验证旧 Context、搜索、评估样本和历史报告。
5. 验证报告异步生成、同 key 重试、归档、复制和分享。分享创建者与接收者均须保有当前权限；分享 ID 本身不授予访问权。
6. 按 P01–P24 完成权限、并发与设备操作。微信开发者工具、iOS/Android 真机结果分别记录；浏览器和本地服务层测试不代替设备验收。

每个验收项记录测试企业/账号角色、输入范围、预期结果、实际响应、证据路径和执行人。真实来源对账、真机、保留策略及按 PRD 阈值的容量验收全部通过后，再判断完整首发条件是否满足。
