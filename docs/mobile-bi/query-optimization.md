# BI 查询逻辑优化记录

本次优化基于 `c797deb3e`，覆盖分析历史读取、周期查询执行计划、知识状态查询和应用搜索。API 路径、DTO、指标分母、Token 归一化及当前 ACL 校验规则保持原契约。

## 历史读取与一致性

分析请求保留当前周期与对比窗口的逐次调用明细。更早的人员调用按成员、上海自然日、团队、应用、场景和 outcome 选取最早事件，供激活、稳定使用和首次队列留存计算；更早的非人员调用不载入应用内存。

发生主 Membership 起止变化的日期保留该人员全部事件，因此日内岗位或部门变化仍使用准确事件时间判断。两个首次调用时间相同时，以事件 ID 作稳定次序。数据库先按冻结 data revision 解析最新事件版本，再处理周期及历史活动，避免日期修订或撤回后的旧事件重新出现。

这项优化减少数据库返回的历史明细和 Go 侧遍历量。源事实继续完整保存，数据库仍需解析历史修订；内容目录、评级和引用元数据仍按快照读取，不能据此宣称已解决任意规模的历史数据查询。

## 周期查询执行计划

原周期读取先用 `IN` 查候选事件。在刚导入 10,000 条调用、PostgreSQL 统计信息尚未更新的样本中，执行计划出现约 5,014 万次无效事件连接比较。完整读取约 45 ms，而周期读取超过 1 秒的诊断预算；带计时的 EXPLAIN 执行约 8.9 秒。

周期读取现在固定候选 ID 集合，再按 ID 经索引查找最新已发布版本。相同诊断样本的周期读取约 63 ms，无效事件连接比较为 0。该修改也作用于内容接口需要的周期事实读取。

回归测试检查实际事件连接工作量，排除小型 revision 目录的线性查找，不使用固定毫秒阈值。测试分别覆盖更新统计信息前后，并已用临时旧查询覆盖确认仍能检测原缺陷。诊断覆盖和日志保留在临时调试目录，生产代码不含调试输出。

## 知识状态与搜索

- 知识状态历史在单个请求内加载一次，并通过生效时间查找引用时状态。资产统计与引用明细共用判断；缓存不跨请求或 Context，不替代当前 ACL 检查。
- 应用搜索在目录和期末 Eligibility 无法确认关联时，按需读取本期事实。当前适用团队变化或 Eligibility 结束后，本期实际使用过的应用仍可出现在有权范围内的结果中。

## 本地对照结果

环境为 macOS arm64、Go 1.27.0、独立 PostgreSQL 18.1 测试容器。每组使用同一数据集分别执行原完整读取和当前实现，共 20 次顺序查询；历史组交替执行两条路径。下表为服务层总览读取耗时，不包含 HTTP 网络或移动端。

| 样本 | 原完整读取 | 优化后 |
| --- | --- | --- |
| 新增 10,000 条历史调用：载入事实条数 | 10,015 | 15 |
| 历史调用样本：总览 p50 | 40.864 ms | 28.497 ms |
| 历史调用样本：总览 p95 | 41.861 ms | 29.258 ms |
| 新增 10,000 条近期调用：总览 p50 | 63.613 ms | 69.333 ms |
| 近期调用样本：总览 p95 | 68.238 ms | 74.677 ms |

历史样本的事实条数减少 99.85%，总览 p95 降低约 30.1%。近期样本保留全部明细，p95 增加约 6.4 ms（9.4%），反映历史处理的额外开销。事实条数不等于整个进程的 RSS，以上样本也不构成生产 SLA。

复现性能对照：

```bash
cd backend
env -u OPENAI_API_KEY GOTOOLCHAIN=go1.27.0 go test \
  -tags='integration bi_capacity' ./internal/bi \
  -run '^TestBI(CapacityBaseline|HistoricalReadBaseline)$' -count=1 -v
```

语义回归包括 `TestAnalysisHistoryCompactionPreservesMetricsAndDirectoryChanges`、`TestRetentionUsesStableEventIDForSimultaneousFirstCalls`、`TestKnowledgeStatusHistoryLoadsOncePerFrameAndPreservesTimeOrder`、`TestPeriodUsageQueryAvoidsQuadraticWorkAfterBulkImport` 和 `TestApplicationSearchKeepsPeriodUsageAfterEligibilityEnds`。全仓验证结果记录在 [验收进度](./acceptance-progress.md)。
