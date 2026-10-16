# 项目文档索引

本目录按功能模块保存当前指南、API 契约和验证记录。原始设计基线、退役链路及一次性检查记录集中放在 [历史归档](archive/README.md)，不作为当前操作入口。部署与运维从 [deploy/](../deploy/README.md) 开始，仓库文件的存放位置见 [目录说明](development/REPOSITORY_LAYOUT.md)。

## 按任务阅读

| 任务 | 入口 | 后续文档 |
| --- | --- | --- |
| 配置开发环境、运行测试或同步上游 | [开发与插件](development/README.md) | toolchain、CI、生成代码与本地验证 |
| 接入 Desktop、管理企业或读取总结 | [Desktop](desktop/README.md) | 基础 API、v2 会话、资源、日报和 OpenAPI |
| 管理企业到账、VCC、凭证和对账 | [账目管理](accounting-management/README.md) | 设计、实现、原型和验证记录 |
| 接入小程序 BI 或排查企业授权 | [小程序 BI](mobile-bi/README.md) | 独立身份、接口、数据口径、联调和验收 |
| 配置在线充值或集成支付管理 API | [支付](payments/README.md) | 中英文配置指南和 Admin 集成接口 |
| 配置路由、渠道监控或排查代理归属 | [网关与渠道](gateway/README.md) | 路由、调度、监控安全默认值及上游错误快照 |
| 提交图片任务或调用 Seedance | [图片与视频](media/README.md) | 异步图片、批量图片和视频任务 |
| 阅读部署与运营合规承诺 | [合规文档](legal/README.md) | 中英文正文、运行时引用与版本维护 |
| 查阅原始设计、旧版链路或历史检查 | [历史归档](archive/README.md) | 账目设计基线、BI v1 退役说明和仓库检查记录 |

## 文档类型

| 类型 | 阅读方式 |
| --- | --- |
| 使用与配置 | 从模块 README 进入，按当前接口和配置操作 |
| API 契约 | [Desktop](desktop/README.md#openapi-契约) 与 [BI](mobile-bi/README.md#openapi-契约) 分别索引；文件可被路由测试和工具读取 |
| 实现与权限 | 说明当前模块的边界、数据来源和部署条件；不能仅凭设计稿判断功能已交付 |
| 验证记录 | 保留原验证日期、测试范围和未完成项，不因目录整理而更新为当前验收结果 |
| 历史归档 | 用于追溯来源和决策；原型、快照和退役说明不代替现有页面、接口或扫描结果 |
| 合规正文 | 被应用直接导入；内容与版本按合规文档的维护说明处理 |

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
├── screenshots/               # Documentation image assets
└── archive/                   # Design baselines and historical records
```

图片素材及当前引用状态见 [截图索引](screenshots/README.md)。

## 文档状态与维护

- 功能说明描述使用方式；OpenAPI 描述接口；设计和原型描述约束与交互；验证记录只证明所列环境和时间下的结果。
- Desktop 基础 v1 与 v2 会话文档并存，v2 不替代所有基础接口。BI 当前契约从 [BI 索引](mobile-bi/README.md) 阅读，旧版链路从历史归档追溯。
- 新文档放入对应模块，更新模块索引。新增模块时同步更新本页和 `.gitignore` 的白名单。
- 文档路径调整时，同步修正相对链接、根 README、界面帮助链接，以及读取 OpenAPI 的 route tests。
- API 契约调整时，运行对应路由与 schema 检查；当前 BI 契约由路由与跨端联调检查核对。仅移动文件时保持契约内容不变。
- 部署配置以 `deploy/` 为入口；插件协议以 `backend/pkg/pluginapi/` 为准，避免在 `docs/` 复制另一份实现说明。
