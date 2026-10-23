# 开发与插件文档

开发环境、代码生成和验证从开发指南开始。插件以独立包开发，其运行协议和宿主边界由 Plugin API 定义。

| 文档 | 用途 |
| --- | --- |
| [开发指南](DEV_GUIDE.md) | 仓库与 toolchain、服务配置、CI、本地验证、版本规则和故障排查 |
| [仓库目录说明](REPOSITORY_LAYOUT.md) | 代码、文档、Agent skills 与本地文件的存放位置 |
| [企业 AI 网关从零建设方案](standalone-platform-architecture-2026-10-23.md) | 定制业务全保留、最小依赖、试点首版、企业多渠道接入与切换门槛 |
| [定制业务与依赖清单](current-feature-preservation-2026-10-23.md) | 130 项定制业务、10 组必要依赖、工程检查、原版排除范围及验收要求 |
| [全仓接口声明快照](../archive/development/current-api-surface-2026-10-23.json) | 固定基线的源码入口索引；非迁移范围，也非运行路由验收结果 |
| [new-api 基础网关研究](new-api-gateway-reference-2026-10-23.md) | 固定源码的渠道、令牌、协议转换、路由、计量及企业化调整依据 |
| [企业管理业务规划](enterprise-organization-plan.md) | 组织归属、数据汇集、成果沉淀、知识库与 SOP 的业务方向 |
| [部门、岗位与员工归属实施方案](enterprise-organization-implementation-plan.md) | 组织模型、迁移、接口、管理页面与验收方案 |
| [组织归属使用与部署](../desktop/DESKTOP_ORGANIZATION_STRUCTURE.md) | 部门、岗位、调岗、成员导入和分阶段数据库收紧 |
| [插件开发](PLUGIN_DEVELOPMENT.md) | 独立进程、配置 UI、包清单、签名和宿主能力边界 |
| [Plugin API](../../backend/pkg/pluginapi/README.md) | 协议、包格式及公开开发接口 |
| [CI workflow](../../.github/workflows/backend-ci.yml) | 当前检查命令与工具版本 |
| [Security Scan](../../.github/workflows/security-scan.yml) | Go 漏洞检查与 pnpm audit 例外 |
| [Release workflow](../../.github/workflows/release.yml) | 发布目标、构建矩阵与发布流程 |

## 历史检查

[2026-10-16 仓库目录检查](../archive/development/2026-10-16-repository-audit.md) 保留该次整理结果和当时发现的入口问题；[pnpm audit 快照](../archive/development/pnpm-audit.snapshot.json) 只保留原始扫描记录。当前验证仍按开发指南运行，完整归档目录见 [历史归档](../archive/README.md)。

[返回文档索引](../README.md)
