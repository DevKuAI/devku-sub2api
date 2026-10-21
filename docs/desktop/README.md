# Desktop 企业与客户端文档

Desktop 文档覆盖企业、成员、客户端认证、模型配置、资源分发、会话上报，以及 Web 后台的对话与总结管理。后端和后台实现不代替 Desktop 客户端、Hook 或对象存储的实际联调。

## 功能说明

| 文档 | 用途与边界 |
| --- | --- |
| [灰度更新与 Release 自动交付](desktop-update-rollout.md) | 安装包自动导入草稿、企业灰度及转全量 |
| [基础 API v1](DESKTOP_API_V1.md) | 企业、成员、模型配置、管理端和更新接口；登录与刷新保留 v1 兼容说明 |
| [v2 会话与对话](DESKTOP_SESSION_CONVERSATION_V2.md) | 滑动会话、Installation ID、对话上报、权限和后台回看 |
| [部门、岗位与员工归属](DESKTOP_ORGANIZATION_STRUCTURE.md) | 组织维护、调岗历史、成员导入与分阶段迁移 |
| [日报与管理总结](DESKTOP_DAILY_REPORTS.md) | 对话归集、生成、执行历史、预算与保留策略 |
| [MCP / Skill 资源](desktop-resources-v1.md) | ZIP 包发布、启停、版本及鉴权下载；Desktop Rust 安装需另行联调 |
| [企业周月总结](../mobile-bi/period-reports.md) | 自然周期、来源冻结、任务恢复和历史版本 |
| [账目管理](../accounting-management/README.md) | 企业资金与凭证，以及关联账户的只读查看 |

## OpenAPI 契约

| 契约 | 范围 |
| --- | --- |
| [Desktop 基础接口](openapi/DESKTOP_API_V1.openapi.yaml) | 基础管理、客户端、更新、用量和日报接口 |
| [v2 会话与对话](openapi/DESKTOP_SESSION_CONVERSATION_V2.openapi.json) | v2 登录、滑动会话、对话写入和后台查询 |
| [资源接口](openapi/desktop-resources-v1.json) | MCP / Skill 发布、管理与分发 |
| [企业 BI 与周月总结扩展](../mobile-bi/desktop.openapi.yaml) | 企业邀请、使用者管理与周月总结 |

接口使用者应按功能组合阅读契约；v2 会话文档补充基础 v1，并非对所有 Desktop 接口的替代。

[返回文档索引](../README.md)
