# API v1.1.0 实施记录

本目录的六份原始交接文件来自用户提供的 `devku mobile api文档.zip`。原件缺少 `protocol.md`、`examples.json`、`contract-cases.json`、`interaction-map.md`、校验脚本和校验报告，不能把这些文件视为已取得。补充材料及真实数据源文档已在任务中询问。

五份 Markdown 与压缩包原件逐字节一致。`openapi.yaml` 仅修正导入操作的 Idempotency-Key 描述：由“用户/企业/operationId，保留 24h”改为“企业/source/operationId/key，凭证轮换不改变幂等域，实体 revision 长期去重”，与原 `ingestion-and-history.md` 的明确约定一致。其余路径、字段和 Schema 未改动；`MetricDefinition.unit` 的待确认枚举也未扩充。

## 交付边界

完整目标是 59 个操作、BI 数据层、首批数据源适配、必要的原站管理入口和验收。BI 在 sub2api 内实现，保持现有网页和 Desktop API 的兼容性。知识、评估、报告与分享属于完整首发。

## 当前进度

- API v1.1.0 的 59 个操作均已接入路由；路由与 OpenAPI 的方法/路径逐项对齐，所有 Bearer 入口的未认证请求返回 401。
- 身份：独立 audience、微信绑定审批/兑换、可撤销会话、refresh 轮换与重放检测、企业授权，以及原站绑定与授权管理页面。
- 采集：17 类记录、长期版本去重、checkpoint 冲突、单 source 在途约束、worker 租约隔离、outbox 与原子发布；ACL 拒绝覆盖层在读取视图构建失败后仍有效。
- 分析：冻结 Context、自然周期、期末可使用人员、事件归属 Token、稳定使用、首次队列留存、频率、趋势、团队/岗位/成员/应用/场景，以及缺失数据的 null/partial 语义。留存观察同时检查 Membership 和实际调用的授权团队，范围外调用不能作为可观察证据。
- 日聚合：按 Shanghai 日期、source、团队、actor、应用、场景和 requested model 汇总请求及 Token，随 data revision 原子发布。更正日期或撤回最后一条事件时写入空日期标记，避免旧汇总重新出现；Context coverage 使用日聚合。活跃人数仍按人员去重，不相加每日人数。
- 内容：知识与版本、案例、固定历史来源、评估样本、检索、引用、收藏及 ETag 条件更新的个人 note。note 不写入交互评分。
- 简报：异步不可变快照、24 小时幂等回执、生成任务恢复、归档读取、分享创建/列表/解析/撤销；分享同时检查创建者和接收者当前权限。报告权限依赖包括本期实际引用的历史知识版本及来源，当前版本切换不能绕过旧来源撤权。
- sub2api 适配：从企业历史 API Key 关联读取 UsageLog，保留稳定外部映射，避免因提交乱序漏掉较小的日志 ID；不复制密钥、手机号或请求正文，不猜测 actor、应用、场景或结果。
- 临时状态：分批清理过期 refresh、session、绑定挑战、微信 code 摘要、Context、分页缓存及命令回执。已清理 Context 仍可通过绑定身份的签名 ID 返回 410，其他身份返回 404；报告归档不依赖临时 Context 行。
- 读取与并发：内容接口按周期加载调用事实，先选事件 ID 再解析最新修订，避免修正到其他周期的旧事件重新计入；授权编辑按固定账号顺序加锁，交叉编辑企业授权不会形成相反锁顺序。
- 已通过：BI PostgreSQL 集成测试、分析/内容/报告响应 Schema 校验、122 项分页数据测试、来源与分享撤权、note 并发更新、前端相关 39 个测试、前端 typecheck/lint/build，以及完整后端 unit 测试（移除可选实时测试凭证，按 CI 条件运行）。
- 完整后端 integration、全仓 golangci-lint、后端 build、前端 lint/build 均通过；完整前端测试为 324 个文件、2,417 项测试通过。逐项业务与外部联调验收继续执行。

路由齐全不代表生产验收完成。真实微信/真机、外部组织/知识/评估生产者的适配和联调、容量目标及业务数据物理保留策略仍待材料和环境确认。尚未连接业务生产数据库、推送或部署；本地提交不代表生产验收完成。

## 当前实现决策

- 本周截至昨日，并与上周相同数量的完整自然日比较；周一两边均无完整日期，返回 `not_observable`。此口径已由用户明确确认。
- 新网页 JWT 签发 `iss=devku-sub2api`、`aud=sub2api-web`，既有网页校验保留旧 Token 兼容；BI 的原站绑定入口必须经过原网页 JWT/会话策略并匹配新 audience。
- 小程序 access Token 使用 `aud=bi-miniprogram`，有效期 900 秒；refresh 轮换后有效期 43200 秒，采用活动续期。服务端仍逐次检查数据库会话、绑定及原账号状态。活动续期属于当前实现默认值，需要与缺失的原协议核对。
- 绑定挑战默认 10 分钟有效；同一 AppID/openid 的绑定及会话写操作按身份串行化。用户撤销绑定时同时撤销该身份所有待处理/已审批挑战及绑定下全部会话。
- 服务端仅保存 openid 的 HMAC 索引、绑定票据及 refresh Token 的摘要。`bi.identity_secret` 必须稳定保管，其轮换需要迁移方案。
- 管理列表快照有效 30 分钟，游标绑定用户、操作、分页大小及快照。权限收紧或绑定撤销后旧分页不可继续读取。
- 原站授权接口位于 `/api/v1/admin/bi/organizations/{organization_id}/grants`，沿用管理员鉴权、审计及合规门控，采用原站响应 envelope。GET 使用 page（每页 50）；PUT 使用 user_id、role、all_teams、team_ids、capabilities、expected_revision（首次为 0）；POST `/{manager_id}/revoke` 携带当前 expected_revision。冲突返回 409，避免覆盖并发修改。
- 企业 ID 映射至原 Desktop 企业 public_id；角色名称为展示分类，操作授权以该企业显式 capability 和团队范围为准，不使用 `/me` 的 capability 并集放行。

## 配置与验证

`bi.enabled` 默认 false。启用前必须提供 AppID、AppSecret、两份独立的 base64 密钥及真实 HTTPS 隐私说明 URL/版本。参见 `deploy/config.example.yaml` 的 `bi` 配置。新增迁移为 `242_bi_identity.sql` 至 `248_bi_daily_usage.sql`。

可重复执行的阶段验证：

```bash
cd backend
env -u OPENAI_API_KEY go test ./internal/bi ./internal/config ./internal/service -run 'Test(Mobile|WeChat|ErrorEnvelope|Cursor|BIConfig|WebAudience|Acceptance)' -count=1
env -u OPENAI_API_KEY go test -tags=integration ./internal/bi -count=1
```

集成测试创建独立 PostgreSQL 18 容器并运行真实迁移；测试完成后清理测试容器，不连接部署数据库。

采集凭证由运维工具管理，不向小程序提供签发接口。配置 `BI_DATABASE_DSN` 后，使用 `go run ./cmd/bi-connector -operation issue -organization <企业 public_id> -source <稳定 source_id> -namespace <实体前缀> -kinds usage -ttl 24h` 签发。工具仅在签发时输出 bearer；数据库保存摘要。使用 `-operation revoke -id <credential_id>` 撤销。轮换凭证复用 source 与 namespace，不重置 checkpoint，也不能借轮换扩大来源允许的记录类型。

验证证据、各验收项的覆盖范围和剩余条件记录在 [验收进度](./acceptance-progress.md)。后端测试清除 `OPENAI_API_KEY`，避免触发仓库中可选的外部实时对比测试。

## 本地容量基线

2026-09-23 在 MacBookPro18,1（arm64、10 核、16 GiB 内存），Go 1.27.1、OrbStack Docker（约 4 GiB 内存）、PostgreSQL 18.1 独立测试容器中执行：

```bash
cd backend
env -u OPENAI_API_KEY go test -tags='integration bi_capacity' ./internal/bi -run TestBICapacityBaseline -count=1 -v
```

| 项目 | 本次实测 |
| --- | --- |
| 导入样本 | 单企业新增 10,000 条自动调用，20 批，每批 500 条；同一天、10 个 model |
| 导入总耗时 | 40.587 秒，包含校验、事实/视图构建及逐批发布 |
| 当前周期日聚合分组 | 16 组，包含原有边界样本 |
| Context 创建 | 20 次顺序请求，p50 6.164 ms，p95 7.735 ms |
| Overview 读取与序列化 | 20 次顺序请求，p50 62.349 ms，p95 64.408 ms |
| 数值断言 | 本期 10,007 次请求、1,701,500 Token，均精确通过 |

该测试直接调用服务层，不包含 HTTP 网络、移动端、并发用户、多企业或多年历史。Overview 仍读取完整调用历史用于激活与留存，内容元数据也未完成大规模索引优化。本次结果是可复现的本地基线；生产容量与 SLA 必须按真实数据分布、并发规模和 PRD 阈值另行验收。

## sub2api 数据源适配

运维侧设置 `BI_DATABASE_DSN` 与 `BI_CONNECTOR_TOKEN` 后，可运行：

```bash
cd backend
go run ./cmd/bi-connector -operation sync-sub2api -since 2026-09-01T00:00:00Z -limit 500
```

每次最多提交一批，得到 202 对应的任务信息后等待 applied/rejected，再读取下一批。该适配器应作为所选网关调用的唯一 usage producer；后续可信工作入口的身份/应用/场景归因应合并至相同 source 和稳定事件 ID，不能把相同调用另导一份。

UsageLog.created_at 用作对账时间；requested_model 缺失时保留 unknown。明确 token 计费且具有非零测量桶的记录按 exclusive_buckets 导入；无法证实测量的全零记录与非 Token 媒体计费保留 unavailable。该日志并不覆盖全部失败尝试，因此导入维持 complete_through=null、initial_backfill_complete=false，不据此宣称完整请求历史或人员采用。

## 尚需确认的契约与交付条件

- 原交接引用的 protocol.md、examples.json、contract-cases.json、interaction-map.md、原校验工具和 PRD 未随包提供。已询问完整目录路径。
- 本周对比口径已经确认；refresh 活动续期、绑定挑战默认 10 分钟及角色配置规则仍须与完整协议复核。
- MetricDefinition.unit 缺少 application。已询问是否扩充该枚举；确认前不在指标字典中把活跃应用数误标为请求次数，统计响应本身正常提供 active_applications。
- 尚缺真实组织/Eligibility、工作入口归因、知识/案例/评估来源的接口或样本，不能凭通用 ImportBatch 接口声称这些生产系统已完成适配。
- report_retention_months 默认 24，控制报告可读期限；业务事实、内容历史和审计的物理删除规则尚未启用，需先落实实际保留期限及引用保留要求。临时缓存清理不替代业务数据保留策略。
- 未执行真实 AppID、HTTPS 合法域名和微信 iOS/Android 真机验证，也未作没有 PRD 容量基线的 SLA 承诺。
