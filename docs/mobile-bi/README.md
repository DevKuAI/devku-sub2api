# BI API 文档索引

当前实现面向 API v1.1.0。可导入的接口文档共覆盖 62 个操作：59 个小程序/采集操作及 3 个配套原站授权管理操作。它们已在本地实现；真实数据源、微信设备和生产容量验收仍按 [验收进度](./acceptance-progress.md) 跟踪。

| 文档 | 用途 |
| --- | --- |
| [主接口 OpenAPI](./openapi.yaml) | 59 个小程序/采集操作，请求响应 Schema、示例、权限、分页和错误契约；`x-implementation-status=implemented-local` |
| [原站授权管理 OpenAPI](./admin.openapi.json) | 3 个管理授权操作，可独立导入；包含管理员认证、revision 条件写、原站响应 envelope 和错误分支 |
| [隐私说明 OpenAPI](./privacy-notice.openapi.yaml) | 公开读取、后台读取和发布 3 个操作；程序内 `/legal/bi-privacy` 页面及 bootstrap 动态配置 |
| [配置参考](./configuration.md) | 9 个服务配置项、环境变量、默认值、校验、部署入口、CLI 参数及固定值 |
| [测试企业与联调操作](./integration-runbook.md) | 测试环境、授权、微信绑定、采集、修订和失败恢复 |
| [实施记录](./implementation.md) | 已实现能力、兼容决策、迁移、容量基线和待确认项 |
| [验收进度](./acceptance-progress.md) | 本地证据与真实环境待验收项 |
| [实现复核](./review-2026-09-23.md) | Standards/Spec 两个方向的复核和修复记录 |
| [查询优化记录](./query-optimization.md) | 历史读取、执行计划、知识状态与应用搜索修复，以及同进程容量对照 |

原始交接说明保留在 [backend-handoff.md](./backend-handoff.md)、[数据模型](./data-model-and-metrics.md)、[采集与历史](./ingestion-and-history.md)、[现有能力映射](./sub2api-mapping.md) 和 [验收清单](./acceptance.md)。这些 Markdown 描述的是交接时状态，实施进度以本目录新增记录为准。

原交接引用的 `protocol.md`、`examples.json`、`contract-cases.json`、`interaction-map.md`、原校验工具/报告和 PRD 尚未取得。主 OpenAPI 已含内嵌示例；这些示例不构成可从空数据库顺序执行的种子脚本。`MetricDefinition.unit` 是否增加 `application` 仍待确认，当前没有把活跃应用数错误标记为 request。

接口定义、配置文档和本地验证已有可交接结果；原协议及业务环境信息尚未全部完备。
