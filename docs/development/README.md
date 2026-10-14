# 开发与插件文档

开发环境、代码生成和验证从开发指南开始。插件以独立包开发，其运行协议和宿主边界由 Plugin API 定义。

| 文档 | 用途 |
| --- | --- |
| [开发指南](DEV_GUIDE.md) | 仓库与 toolchain、服务配置、CI、本地验证、版本规则和故障排查 |
| [插件开发](PLUGIN_DEVELOPMENT.md) | 独立进程、配置 UI、包清单、签名和宿主能力边界 |
| [Plugin API](../../backend/pkg/pluginapi/README.md) | 协议、包格式及公开开发接口 |
| [CI workflow](../../.github/workflows/backend-ci.yml) | 当前检查命令与工具版本 |
| [Security Scan](../../.github/workflows/security-scan.yml) | Go 漏洞检查与 pnpm audit 例外 |
| [Release workflow](../../.github/workflows/release.yml) | 发布目标、构建矩阵与发布流程 |

[返回文档索引](../README.md)
