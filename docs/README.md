# 项目文档索引

本目录集中保存开发指南、功能设计、API 契约和验证记录。部署配置和操作文档位于 [deploy/](../deploy/README.md)，变更提案与规格位于 [openspec/](../openspec/)。

## 开发与账目管理

| 文档 | 内容 |
| --- | --- |
| [开发指南](DEV_GUIDE.md) | 开发环境、工作流、常见问题与本地验证命令 |
| [账目管理](accounting-management/README.md) | 企业到账、VCC、凭证、支出、对账与账户只读查看 |
| [插件开发](PLUGIN_DEVELOPMENT.md) | 插件协议、配置界面与打包要求 |

## Desktop 与企业功能

| 文档 | 内容 |
| --- | --- |
| [Desktop API v1](DESKTOP_API_V1.md) | 企业登录、资源、配置与管理接口 |
| [Desktop API OpenAPI](DESKTOP_API_V1.openapi.yaml) | API 导入契约 |
| [Desktop 会话与对话上报 v2](DESKTOP_SESSION_CONVERSATION_V2.md) | 会话、对话与服务端行为 |
| [对话上报 OpenAPI](DESKTOP_SESSION_CONVERSATION_V2.openapi.json) | v2 接口契约 |
| [Desktop 日报](DESKTOP_DAILY_REPORTS.md) | 企业日报与分析功能 |
| [Desktop 资源](desktop-resources-v1.md) | MCP/Skill 包与资源分发 |
| [资源 OpenAPI](openapi/desktop-resources-v1.json) | 资源接口契约 |
| [移动端 BI](mobile-bi/README.md) | BI 数据模型、接口、接入与验收文档 |

## 支付、渠道与图片任务

| 文档 | 内容 |
| --- | --- |
| [支付说明（中文）](PAYMENT_CN.md) / [English](PAYMENT.md) | 在线支付功能与配置 |
| [Admin 支付集成 API](ADMIN_PAYMENT_INTEGRATION_API.md) | 管理端支付接口 |
| [复合分组](COMPOSITE_GROUPS.md) | 分组与调度行为 |
| [渠道监控安全默认值](channel-monitor-v2-safe-defaults.md) | 监控配置与安全默认值 |
| [异步图片任务](ASYNC_IMAGE_TASKS.md) | 异步任务设计与接口 |
| [批量图片 MVP](BATCH_IMAGE_MVP.md) | 批量图片功能设计 |
| [Seedance API](seedance-api.md) | 视频生成接口说明 |

管理员合规文档位于 [中文说明](legal/admin-compliance.zh.md)和 [English](legal/admin-compliance.en.md)。

## 根目录组织约定

| 路径 | 用途 |
| --- | --- |
| `README*.md`、`LICENSE`、`CLA.md` | 项目入口、许可证与贡献者协议 |
| `Makefile`、`Dockerfile*`、`.goreleaser*.yaml` | 构建与发布入口 |
| `backend/`、`frontend/` | 后端与前端代码 |
| `deploy/` | 部署配置、脚本与运维文档 |
| `docs/` | 开发指南、功能文档、API 契约与验证记录 |
| `assets/` | README 使用的图片资源 |
| `openspec/` | 变更提案与功能规格 |
| `skills/` | 项目集成 skills |
| `tools/` | 仓库维护工具 |
| `.github/`、`.agents/` | CI 配置与项目级 Agent skills |

新增功能文档按模块存放到 `docs/<module>/`。`docs/` 使用 `.gitignore` 白名单，新增文档时同步添加对应文件或模块的精确例外。
