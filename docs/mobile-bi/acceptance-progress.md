# API v1.1.0 验收进度

本文件记录已取得的证据和缺口，不替代原 acceptance.md，不把代码存在或 Schema 通过当作真实联调完成。

## 已执行的本地验证

- `TestBIRoutesMatchEveryVersion110OperationAndProtectBearerRoutes`：59 个 operation 的方法/路径与 OpenAPI 对齐，Bearer 入口拒绝未认证请求。
- `TestBIAdminRoutesMatchSupplementalOpenAPI`、`TestBIAdminResponsesMatchSupplementalOpenAPI`：补充的 3 个原站授权操作与路由对齐；实际列表、保存、撤销及参数/并发错误响应通过补充文档 Schema 校验。
- `TestBIConfigLoadsEveryEnvironmentSetting`：全部 9 个环境配置字段经实际 Load 流程正确加载。四套 Compose 的默认/自定义两组配置渲染均通过，既有 application environment/security/storage 检查通过。
- 两份 OpenAPI 均通过 OpenAPI 3.1 规范校验，共 62 个唯一操作；主文档实现状态已更新为 implemented-local，配套文档和配置入口见 [文档索引](./README.md)。
- `TestEmbeddedSchemasMatchOpenAPIVersion110` 及响应校验：嵌入 Schema 与交接契约一致；分析、内容、报告的实际 HTTP 响应通过对应 Schema 校验。
- `TestBindingApprovalRaceAndOneTimeExchange`、`TestWeChatCodeReplayAndUnlinkInvalidatesOutstandingChallenges`、`TestRefreshRaceRevokesFamilyAndLogoutIsImmediate`：绑定争用、一次性兑换、code 重放、解绑和 refresh 并发。
- `TestGrantsAreExplicitScopedAndRevokeImmediately`、`TestAnalysisContextIsImmutableAndNeverWidens`：企业授权隔离、Context 所属用户/企业校验、撤权与过期、固定 data revision。
- `TestImportAdmissionSerializesSourceAndSurvivesCredentialRotation`、`TestImportsResolveCyclicVersionsAndRejectWholeInvalidBatches`：幂等域、source 并发、checkpoint、循环引用和全批原子性。
- `TestImportRecoveryPublishesOnlyAfterViewsAreBuilt`、`TestSourceHistoryAndACLRestrictionsSurviveFailedPublication`：租约隔离、恢复、无半批读取、失败发布不放开 ACL。
- `TestAnalysisCohortTokensAndRetentionRemainDistinct`、`TestSecondaryMembershipFiltersPeopleWithoutChangingPrimaryStatistics`：期末母体、事件团队归属、离职/调岗、稳定使用与本期活跃的区别、辅助归属筛选及去重。
- `TestAnalysisPeriodsUseShanghaiCompletedDays`、`TestAnalysisTokenTotalsPreserveUnknownAndBigIntegers`、`TestAnalysisUnavailableZeroAndPermissionAreDifferent`：本周/上周同期、闰年自然月、大整数、未测量 Token、未知与零值、无知识权限。
- `TestAnalysisPaginationPreservesTotalsAndBindsEntity`：122 个模型、成员应用、反馈应用分组的分页，以及总量不随页变化、游标不能跨成员复用。
- `TestContentContractsAndActualReferenceDeduplication`、`TestContentACLIsIndependentOfTheFrozenRevision`：真实引用去重、案例双版本定位 ID、来源父关系及独立 ACL、样本权限过滤、来源正文不进入检索摘要。
- `TestPersonalNotesEnforceAtomicPreconditionsAndOwnCleanup`：428/412、两端并发、收藏幂等、note 不影响评分、失权后本人可清除残留记录。
- `TestReportReceiptsAndArchiveSurviveContextExpiry`、`TestShareChecksCreatorRecipientAndCurrentEvidence`、`TestReportGenerationLeaseFencesAnInterruptedWorker`：幂等报告、过期 Context 清理后的归档读取、分享双方当前权限、来源撤回、生成恢复。
- `TestSub2apiAdapterPreservesHistoricalOwnershipWithoutInventingIdentity`：历史密钥与已删除成员的企业归属、无关 carrier key 隔离、较小 ID 晚提交、不导出敏感字段、不可确认字段保留 unknown/unavailable。
- `TestReportHistoricalReferenceSourceRevocation`：报告包含本期实际引用的旧知识版本依赖；即使当前版本已切换，旧来源撤权仍拒绝历史报告。
- `TestRetentionCannotObserveCallsOutsideGrantedTeams`：人员在授权团队之外的实际调用不能被当作已完整观察的留存证据。
- `TestPeriodReadUsesLatestEventTimeWithoutResurrectingOldVersions`、`TestDailyUsageSnapshotsTrackCorrectionsAndEmptyDays`：跨周期修订、撤回、空日期及新旧 Context 对账；旧 Context 不变，新 Context 不重计旧事件。
- `TestConcurrentGrantEditorsUseConsistentManagerLockOrder`：20 轮交叉编辑企业授权，按固定账号顺序加锁。
- `TestAcceptanceP11CachedPagesRecheckSourceAndKnowledgeRestrictions`：来源 ACL 已收紧、业务 revision 尚未发布时，旧评估样本分页返回 403，重读仅保留可见样本；知识撤权后旧搜索分页同样拒绝。
- `TestAcceptanceP12ConnectorCannotCrossSourceOrganizationOrManageGrants`：实际 ConnectorBearer 的跨 source/企业请求、未允许的记录类型和管理授权导入被拒绝；越界 namespace 全批拒绝且不推进 checkpoint；凭证轮换不能扩权。
- `TestAcceptanceP18ConcurrentImportAdmissionHasOneBatch`：8 个同时开始的请求，不同幂等键仅一个受理，其余 IMPORT_BUSY；相同键均返回同一批次，仅发布一个 revision。
- `TestRejectedImportDiscardsUnpublishedProjectionsAndPreservesDenials`：在 staged 和 built 阶段模拟不可恢复失败，未发布投影与记录回执均被清理；拒绝覆盖层继续生效，原 checkpoint 和旧 Context 保持，新键修复重放可成功。
- 原站 BI 组件新增 4 项可控异步回归：绑定撤销后的迟到成功/错误，以及同企业授权保存/撤销后的旧列表读取；定向 9 项测试、ESLint 和 typecheck 通过。
- `TestUsageCorrectionsRevalidatePublishedRatings`、`TestUsageCorrectionUsesFinalRatingAndKeepsOldContext`：人员、actor 和时间修订不能保留失效评价；同批修复/撤回可以通过，旧 Context 不变。
- `TestUsageTimeCorrectionRevalidatesReferencesAndRetries`、`TestUsageTimeCorrectionRespectsSameBatchClosedAndRetractedVersion`：事件时间修订重验知识版本有效期和重试顺序；同批关闭并撤回版本也不能绕过有效期。
- `TestUsageCorrectionsPreserveRetractionsAndClosedVersionHistory`、`TestUsageCorrectionCannotRewriteAnotherSourcesRating`：Token 修正、正常生命周期关闭和调用撤回保留约定语义；跨来源只能修复自己拥有的记录。
- `TestAnalysisHistoryCompactionPreservesMetricsAndDirectoryChanges`：与完整事实读取逐项对照采用、留存、Token、日/七日趋势；覆盖本周、上周、上月、日内岗位变化、修订、撤回、旧 Context 及受限团队。
- `TestRetentionUsesStableEventIDForSimultaneousFirstCalls`：同时发生的首次调用按事件 ID 稳定归属，不受数据库返回顺序影响。
- `TestKnowledgeStatusHistoryLoadsOncePerFrameAndPreservesTimeOrder`：同一知识的状态历史每个请求只查一次，保持生效时间、相同时间的修订优先级及不同快照隔离。
- `TestPeriodUsageQueryAvoidsQuadraticWorkAfterBulkImport`：在 PostgreSQL 更新统计信息前后核对实际连接工作量；旧查询已在对照版本中复现约 5,014 万次无效比较，修复后回归通过，不依赖易波动的耗时阈值。
- `TestApplicationSearchKeepsPeriodUsageAfterEligibilityEnds`：当前适用团队变化、Eligibility 结束后，本期实际使用的应用仍可在有权范围内检索。
- 2026-09-23，本次修复后以 `env -u OPENAI_API_KEY GOTOOLCHAIN=go1.27.0` 运行完整后端 `make test-unit`、`make test-integration`、server/bi-connector build 及全仓 `golangci-lint run ./...`，均通过，lint 为 0 issues。
- 前端使用 pnpm 9 重新执行完整测试和 build：324 个文件、2,421 项测试通过；修改文件的 ESLint 和前端 typecheck 通过。

上述结果均为本地验证。后端使用 Go 1.27.0，与仓库 CI 固定版本一致；golangci-lint 为 2.13.0。此前的 10,000 条调用容量基线使用 Go 1.27.1，属于前一轮证据；远端 CI 尚未执行。

查询优化后重新完成后端全量 unit、integration、server/bi-connector build 及全仓 lint（0 issues）。周期查询计划回归已分别验证新查询通过、临时旧查询覆盖失败；性能对照见 [查询优化记录](./query-optimization.md)。本轮仅修改后端及文档，未重跑前端套件。

本次代码复核的范围、两个审查方向及修复结果见 [复核记录](./review-2026-09-23.md)。

已补充 [测试企业与联调操作说明](./integration-runbook.md)，包含配置、显式授权、微信绑定、导入、修订恢复和验收留证步骤。路径、配置键及 CLI 选项按当前实现核对；真实环境尚未按该说明执行。

## 原验收项覆盖情况

以下业务用例已有直接对应的本地断言：

| 项目 | 本地用例与断言 |
| --- | --- |
| A01 | `TestAcceptanceA01AutomaticGrowthDoesNotBecomeAdoptionGrowth`：自动 Token 增长时人员活跃 23→21，采用率降低；总览不生成采用提升解释 |
| A02 | `TestAcceptanceA02GrowingFirstUseCohortsCanHaveLowerRetention`：首次队列 10→20，后续留存 80%→30%；未完成观察周为 null |
| A03 | `TestAcceptanceA03LowVolumePreservesCoverageAndRepeatUse`：低 Token 团队仍显示分母、100% 覆盖与重复使用 |
| A04 | `TestAcceptanceA04MoreUsageAndNegativeFeedbackRemainSeparate`：调用 2→7，helpful 100%→50%；保留 2/5 反馈覆盖率和应用负面票数 |
| A05 | `TestAcceptanceA05CurrentExpiryDoesNotRewriteReferenceTime`：当前已过期但早期引用时有效；过期后引用另计，新旧 Context 各自固定 |
| A06 | `TestAcceptanceA06MultipleVersionsKeepEvidenceButCountOneEvent`：重复引用和同事件两个版本只计一次知识使用/评分，保留两条版本证据 |
| A07 | `TestContentContractsAndActualReferenceDeduplication`：同一评估含 3 个样本，baseline 1 passed、candidate 2 passed、1 not_run；仍需业务方确认样本与标准 |
| A08–A10 | 周期、无来源/已确认零、Token 归一化及大整数用例覆盖；真实来源故障与客户端状态仍待联调 |
| A11–A12 | 组织历史、辅助归属、批次去重/乱序/错误引用、断点与恢复用例覆盖；仍需真实来源对账 |
| A13 | `TestAcceptanceA13IncompleteFirstUseHistoryStaysUnknown`：采集历史不足时活跃仍可观察，激活与稳定使用未知，不生成首次队列 |
| A14 | `TestAcceptanceA14RatingRevisionReplacesOneVoteAndNotesStaySeparate`：补评按事件发生期归属，修订替换一票，旧 Context 不变；note 修改不进入评分 |
| A15 | `TestAcceptanceA15TokenBreakdownsReconcileExactly`：model/日期/团队/actor 精确对账，含超过 2^53 的 Token、unknown 与无测量调用 |
| A16 | `TestAcceptanceA16ComparisonUsesEachPeriodsDenominator`：各期使用各自分母；零基线增长率为 null/no_baseline |

上表验证服务端计算和权限行为，不能替代管理结论、页面交互、真实生产者和设备验收。

| 项目 | 当前证据 | 剩余验收 |
| --- | --- | --- |
| A01–A07 | 有 actor 分解、队列留存、评价与知识/版本/评估实现及集成样本 | 使用业务方提供的数据逐项核对管理结论，不能仅凭通用样本宣布全部产品场景通过 |
| A08–A16 | 有日历、大整数、未知/零、组织历史、导入幂等、引用去重、评分修订、note 隔离及逐模型/多维对账测试 | 补齐实际来源及完整首用历史，与真实账本和业务方边界样例核对 |
| P01–P03、P05 | 有用户/企业/对象/来源/样本及报告权限检查 | 完整协议补齐后复核角色组合和所有拒绝路径；不把入口鉴权等同对象权限全量验收 |
| P04 | 后端 Context/游标绑定；原站企业切换有旧请求保护测试 | 小程序切换企业/周期、缓存与旧请求处理由移动端联调验证 |
| P06–P08、P11–P19 | 有条件写、分享、历史引用来源撤权、ACL 覆盖层、缓存分页再授权、跨 source/企业拒绝、8 路导入并发及任务恢复测试 | 按真实数据源故障/恢复和撤权操作流程联调 |
| P09–P10、P20 | 有未授权空企业、账号状态、code 重放、绑定争用及会话撤销测试 | 真实微信 AppID/code2Session、错 AppID、两台设备及原站实际 2FA 流程 |
| P21–P24 | 有超过 100 项分页、未知/零、历史来源固定、案例版本定位及归档读取测试 | 在小程序真机完成入口、下钻、复制、分享和深链回跳验证 |

上述分组仅用于定位证据；原 acceptance.md 的每个 A/P 条目仍是最终验收依据。

## 容量证据

`TestBICapacityBaseline` 已通过 10,000 条调用、20 批导入和 20 次顺序查询的本地基线。导入 40.587 秒；Context p95 7.735 ms；Overview 服务层读取与序列化 p95 64.408 ms。环境、样本分布、数值断言和复现命令见 [实施记录](./implementation.md#本地容量基线)。未测生产规模、并发吞吐、HTTP 网络或移动设备；PRD 阈值缺失，尚不能判定生产 SLA。

## 外部依赖

1. 原 protocol.md、examples.json、contract-cases.json、interaction-map.md、验证工具及 PRD。
2. 测试 AppID、合法 HTTPS request 域名、测试企业与真实管理账号；凭证只放环境配置。
3. 组织/Eligibility、可信 actor/app/scene、知识/案例/评估来源的接口、样本和同步责任方。
4. 业务事实、内容历史与审计的实际保留政策，以及容量数据和 p95/超时验收目标。

尚未执行生产部署、真实微信设备或外部生产者接入验证。
