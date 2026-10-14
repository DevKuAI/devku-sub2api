# 小程序 BI 与 Desktop 企业总结

小程序采用独立微信身份，通过企业邀请获得功能权限，直接读取 Desktop 企业源数据。企业后台负责生成总结、配置策略和管理邀请；BI 运维仅展示身份、任务与计费 metadata。H5 与小程序共用页面，预览使用独立的本地 DTO 演示数据。

## 当前范围

- **已实现**：独立 `BIUser`、单次企业邀请、多企业功能授权、内存会话、Desktop 用量／模型／成员／客户端／会话／日报读取、周月总结任务与版本、企业授权管理、运维 metadata。
- **保留设计、未接入**：采用率、稳定使用、留存、部门／岗位、企业应用、业务场景、知识复用、质量评估、收藏反馈及受控分享。生产页面显示「未接入」，没有对应旧接口调用。
- **验证状态**：[验收进度](acceptance-progress.md)。代码实现、联调通过与生产验收分别记录。

## 阅读入口

| 文档 | 内容 |
| --- | --- |
| [协议](protocol.md) | 独立身份、邀请、内存凭证、退出、错误和权限 |
| [数据模型与指标](data-model-and-metrics.md) | Desktop 数据来源、日期、大整数与未知值 |
| [接口设计](desktop-api-design.md) | 三份 OpenAPI 与权限依赖 |
| [路由映射](sub2api-mapping.md) | API、服务、查询及 DTO 边界 |
| [周月总结](period-reports.md) | 自然周期、来源冻结、任务恢复、预算与版本 |
| [页面交互](interaction-map.md) | 四个主入口、按需读取、失败恢复 |
| [运维](operations.md) | 企业接入、身份、任务、审计保留 |
| [配置](configuration.md) | 服务开关、密钥、隐私地址与部署 |
| [联调说明](integration-runbook.md) | PostgreSQL、Go handlers 与实际 TypeScript client |
| [验收](acceptance.md) | 业务、权限、数据、迁移与测试清单 |
| [验收进度](acceptance-progress.md) | 有日期的本地验证证据和设备、生产验收边界 |
| [实现说明](implementation.md) | 已实现链路、已退役模块和客户端实现边界 |
| [两端交接](backend-handoff.md) | 后端与移动端共同遵循的协议和联调约束 |
| [查询边界](query-optimization.md) | 当前数据读取、分页、metadata 投影及容量验证限制 |

[小程序 OpenAPI](openapi.yaml)、[平台运维 OpenAPI](admin.openapi.json)、[企业后台扩展 OpenAPI](desktop.openapi.yaml) 为当前 v2 契约；[独立隐私说明 OpenAPI](privacy-notice.openapi.yaml) 继续有效。原 Desktop 基础接口见 [Desktop API](../desktop/openapi/DESKTOP_API_V1.openapi.yaml)。

旧 v1 的绑定、Context、导入、事实、内容和报告分享已经退役，不兼容旧客户端。migration 257 仅清理明确列出的旧 BI 对象；Desktop、原站账号、计费、配置及独立隐私说明保留。

## 迁移与历史记录

| 文档 | 状态与用途 |
| --- | --- |
| [旧导入与历史链路](ingestion-and-history.md) | 退役说明；当前使用 Desktop 原始数据和企业报告版本 |
| [2026-09-23 旧版审阅](review-2026-09-23.md) | 历史记录；不作为当前 v2 契约、性能或验收依据 |

当前契约与实现以本页列出的 v2 文档为准。验收进度保留原验证日期，不因文档整理而更新测试数量或标记生产通过。

[返回文档索引](../README.md)
