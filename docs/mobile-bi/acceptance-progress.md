# 实现与验收进度

更新时间：2026-10-13。状态针对本地工作区，不代表提交、发布或生产可用。

| 项目 | 状态与证据 |
| --- | --- |
| 独立微信身份、企业邀请、功能授权 | 已实现；真实 PostgreSQL integration 通过，覆盖无原站账号、一企业多人、多企业不同权限、重复加入不扩权、撤权、过期、撤销、并发领取和 refresh replay |
| 实际 TypeScript client → Go HTTP → PostgreSQL | 已验证独立登录、邀请、企业选择、用量、总结读取与复制、隔离和退出；外部微信兑换替换为测试实现 |
| 自然周／月及总结来源 | 已实现；calendar、日总结来源 canary、部分／无来源、预算阻断和冻结来源 unit 通过 |
| 周月任务、历史版本与保留 | PostgreSQL integration 验证去重、checkpoint retry、regenerate 新版本、来源变化、租约恢复、计费、正文清理后执行留存及活动任务保护 |
| 精确用量、分页与历史 | PostgreSQL integration 验证跨 32-bit 聚合、缓存桶、121 个模型完整分页、删除成员历史、无权限用量隐藏与不可观测值；客户端测试验证超出 JavaScript safe integer 的字符串显示 |
| 清理升级与新库 | 完整 migration 在新隔离库通过；旧 BI 授权和外键数据的升级测试通过，Desktop／用户／用量／会话／日报／执行／配置样本保持原值 |
| 后端 unit／integration | 全量命令通过；最后修改另跑 BI、Desktop handlers／routes、报告与保留定向回归通过 |
| 后端 build／lint | `make build` 通过；`golangci-lint run ./...` 为 0 issues |
| 后台 typecheck／tests／lint:check／build | 通过；403 个测试文件、3,134 项测试通过。含邀请依赖、迟到响应、step-up 重试及运维正文 canary |
| 小程序 typecheck／tests／weapp／H5 build | `npm run check` 通过，28 项测试；新增 H5 本地 DTO、对账、完整分页、总结版本／来源、权限及北京时间／状态显示测试。此前 Babel 和 DOM.Iterable 修复保留 |
| OpenAPI 与文档 | 四份 OpenAPI 通过 validator；移动和平台 BI 契约通过路由测试；32 个相对文档链接可解析 |
| H5 演示 | 320px／390px 无横向溢出；长企业名称、四入口、成员加载更多／下钻、总结版本／来源、企业切换／权限不足和退出已检查。H5 与小程序共用新版页面组件，数据独立演示 |

## 尚未验证与交付边界

- 真实微信兑换、微信开发者工具、真机、生产模型、容量和生产接入未验证。本地测试替身与编译不代替这些验收。
- H5 与小程序共用页面；H5 本地 DTO 演示与实际 TypeScript HTTP 联调分别验证。H5 不请求微信或生产服务，不作为真实企业数据或微信运行证据。
- 测试使用隔离数据库。migration 已准备并验证；用户现有运行库尚未执行清理，生产没有改动。
- 企业负责人继续原站归属权限，管理自己的邀请与使用者，读取日／周／月总结。平台管理员维护企业配置、生成控制及全局身份；小程序只读。
- 采用、留存、部门岗位、企业应用、业务场景、知识、评估、收藏反馈和受控分享仅保留设计，生产标为未接入，不调用旧接口。
- 未自动 commit、push、部署、发布或发送邀请。新 CI workflow 代码已更新，远端 CI 未运行。

构建仍会输出现有 Browserslist／bundle size 提示；小程序工具链还提示父目录 Expo tsconfig 不可解析。本轮目标 Taro 工程的 typecheck 和两种 build 均以退出码 0 完成。
