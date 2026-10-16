# 历史归档

本目录保存固定版本的设计参考、已退役链路说明和一次性检查记录。日常操作、配置与当前接口从 [文档索引](../README.md) 进入。

| 分类 | 资料 | 状态与阅读边界 |
| --- | --- | --- |
| 账目设计基线 | [原始设计](accounting-management/design.md)、[HTML 原型](accounting-management/prototype.html) | 来自提交 `4cab773bda883ad6306e26d30c497d032108497e`，内容保持原样；后续企业工作区与业务闭环见 [账目管理](../accounting-management/README.md) |
| BI 退役链路 | [旧导入与历史链路](mobile-bi/ingestion-and-history.md)、[2026-09-23 旧版审阅](mobile-bi/review-2026-09-23.md) | 仅用于追溯 BI v1，当前协议与验收见 [BI 索引](../mobile-bi/README.md) |
| 仓库检查 | [2026-10-16 目录检查](development/2026-10-16-repository-audit.md) | 记录该次清理和当时发现的 Makefile、Vite 入口问题；保留当时验证范围 |
| 安全扫描快照 | [pnpm audit 原始 JSON](development/pnpm-audit.snapshot.json) | 原受版本控制的审计输出，不代表当前漏洞状态；当前扫描方式见 [开发指南](../development/DEV_GUIDE.md#ci-与本地验证) |

归档不表示相关功能被删除，也不表示设计内容已全部实现。文档中的日期、测试数量、未验证项、源提交和原始命令保持各自历史含义。

[返回文档索引](../README.md)
