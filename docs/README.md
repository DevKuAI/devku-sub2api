# 项目文档索引

本目录按功能模块保存开发指南、功能说明、API 契约、设计和验证记录。部署与运维从 [deploy/](../deploy/README.md) 开始，变更提案与规格位于 [openspec/](../openspec/)。

## 按任务阅读

| 任务 | 入口 | 后续文档 |
| --- | --- | --- |
| 配置开发环境、运行测试或同步上游 | [开发与插件](development/README.md) | toolchain、CI、生成代码与本地验证 |
| 接入 Desktop、管理企业或读取总结 | [Desktop](desktop/README.md) | 基础 API、v2 会话、资源、日报和 OpenAPI |
| 管理企业到账、VCC、凭证和对账 | [账目管理](accounting-management/README.md) | 设计、实现、原型和验证记录 |
| 接入小程序 BI 或排查企业授权 | [小程序 BI](mobile-bi/README.md) | 独立身份、接口、数据口径、联调和验收 |
| 配置在线充值或集成支付管理 API | [支付](payments/README.md) | 中英文配置指南和 Admin 集成接口 |
| 配置复合分组或渠道监控 | [网关与渠道](gateway/README.md) | 路由、调度和监控安全默认值 |
| 提交图片任务或调用 Seedance | [图片与视频](media/README.md) | 异步图片、批量图片和视频任务 |
| 阅读部署与运营合规承诺 | [中文](legal/admin-compliance.zh.md) / [English](legal/admin-compliance.en.md) | 法律文本与管理员确认边界 |

## 目录结构

```text
docs/
├── README.md                  # Documentation entry point
├── development/               # Development and plugin guides
├── desktop/
│   └── openapi/               # Desktop API import contracts
├── accounting-management/     # Design, implementation and validation
├── mobile-bi/                 # BI contracts, integration and acceptance
├── payments/                  # Payment configuration and integration
├── gateway/                   # Routing and channel monitoring
├── media/                     # Image and video tasks
├── legal/                     # Compliance commitments
└── screenshots/               # Documentation image assets
```

图片素材及当前引用状态见 [截图索引](screenshots/README.md)。

## 文档状态与维护

- 功能说明描述使用方式；OpenAPI 描述接口；设计和原型描述约束与交互；验证记录只证明所列环境和时间下的结果。
- Desktop 基础 v1 与 v2 会话文档并存。BI 的当前契约和已退役链路由 [BI 索引](mobile-bi/README.md) 分别列出，历史审阅记录不代表当前验收。
- 新文档放入对应模块，更新模块索引。新增模块时同步更新本页和 `.gitignore` 的白名单。
- 文档路径调整时，同步修正相对链接、根 README、界面帮助链接，以及读取 OpenAPI 的 route tests。
- API 契约调整时，运行对应路由与 schema 检查；当前 BI 契约由路由与跨端联调检查核对。仅移动文件时保持契约内容不变。
- 部署配置以 `deploy/` 为入口；插件协议以 `backend/pkg/pluginapi/` 为准，避免在 `docs/` 复制另一份实现说明。
