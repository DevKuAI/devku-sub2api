# 上游 v0.2.20 同步记录

本次同步采用上游精确标签 `v0.2.20`，fork 源码版本为 `0.2.20.0`。本次只完成本地合并和验证，不推送、不创建 fork 标签或 GitHub Release。

## 来源

| 项目 | 值 |
| --- | --- |
| 上游仓库 | `Wei-Shaw/sub2api` |
| 上游标签 | `refs/tags/upstream/v0.2.20` |
| 上游 tag object | `ec41309811cd30af310a0e4b9119b976ce65c9be` |
| 上游 peeled commit | `dab829a4e9b7d1cc307189aea8b0319eb29e29d0` |
| Fork 仓库 | `DevKuAI/devku-sub2api` |
| 同步前 main / origin/main | `6878d0aa578f977f436a14c0e2967d337eccde96` |
| 合并基点 | `dece26153621959ccd7635814f4b2d08304e681a` |
| Fork 基线版本 | `v0.2.20.0`，未创建标签 |

合并前已清点相对目标的 237 个 fork 独有提交及其路径。使用三方合并保留双方历史；未把更新的 `upstream/main` 纳入本次同步。合并结果包含 fork 定制，不与上游标签源码相同。

## 保留范围

- 保留 Desktop 企业组织、部门、岗位、成员、托管 API Key、客户端模型配置、资源、更新、会话及报表。
- 保留 BI 独立身份、微信接入、运营管理、周期报表及原生联调入口。
- 保留账目模块，包括渠道账单导入、卡消费唯一入口、合同、订单、对账与凭证。上述业务目录未因本次同步发生改动。
- 保留个人 API Key 创建数量限制、托管 Key 身份、按张计费字段及订阅账号的只读额度查询。
- 保留 DevKu 蓝色主题、双语 README、更新和安装地址、镜像仓库及 fork 发布 workflow。

## 集成与冲突处理

- 接入上游管理 MCP、OAuth 授权和迁移 `249_mcp_oauth.sql`，以及管理员 Key 权限、邮箱精确黑名单和网关修复。MCP 默认关闭，部署和协议限制见 [管理 MCP](../mcp.md)。
- 保留 fork 的用户路由配置参数；MCP 内部管理路由使用空配置参数，保持上游工具目录范围。
- 合并 Wire 清理步骤，同时保留 Desktop/BI 和新增的 DigestSessionStore、BalanceNotifyService 清理。重新执行 Ent/Wire 生成；Ent 相对同步前无变化。
- API Key 认证快照版本提升到 `28`，同时包含 fork 的托管身份、按张计费字段和上游的隐私配置字段。
- WebSocket 每轮首 Token 计时采用上游的轮次修复，并保留 fork 的 `MessageStartsTTFT` 回调。
- HTTP 错误同时保留 fork 的 `retryAfter` 和上游的 `retry_after_seconds`，继续支持账目请求及用量导出的重试逻辑。
- Desktop 图片补充目录的断言加入两个 GPT Image 2.6 模型，模型数由 5 更新为 7，继续校验 OAuth/API Key 类型、分组隔离和上游错误。
- 流式字符串状态复制测试及 benchmark 补齐 `content_block_start`，兼容 fork 对孤立 delta 的校验；保留原有复制隔离断言。
- 注册邮箱黑名单编辑改为 `/admin/settings/registration-email-blacklist` 独立页面，保留未保存提示；保存或取消后返回设置页的安全标签。禁用命中用户仍使用危险操作确认。
- 双语 README 同步说明 GPT Image 2.6、原生 Images 端点、回退限制、参数和价格兜底，并新增 MCP 文档入口。

## 宣传内容核对

保留中立上游署名。经用户授权，排除 `README_JA.md` 及 `assets/partners/logos/` 下的 `aigocode.png`、`aigocode-dark.png`、`marucode.jpg`、`massivetokens.jpg`，未重新引入推广段落或孤立宣传资产。相对同步前 main 无文件删除。

## 验证

验证环境使用 Go `1.27.3`、以 Go `1.27.3` 构建的 golangci-lint `2.14.0`、Node.js `24.21.0` 和 pnpm `9.15.9`。Release helpers 和 audit checker 另使用 Python `3.12.16` 按 CI 复核。后端测试移除外部 `OPENAI_API_KEY`；集成测试使用 Docker 并设置 `DESKTOP_REQUIRE_WEB_HTTP=1`。

| 检查 | 结果 |
| --- | --- |
| CI shell job 全部命令 | 通过 |
| Compose simple mode、Desktop conversation proxy 检查 | 通过 |
| Release helpers 单元测试及镜像脚本语法 | 通过；Python 3.12 下 12 个测试通过 |
| audit checker 自测 | 通过；Python 3.12 下 4 个测试通过 |
| `env -u OPENAI_API_KEY make -C backend test-unit` | 全量通过 |
| `env -u OPENAI_API_KEY make -C backend test-integration` | 全量通过；启用 Web HTTP 联调 |
| BI Native workflow 的两条 Go test 命令 | 均通过；使用 `-count=1` 重新执行 |
| `golangci-lint run --timeout=30m ./...`，在 `backend/` 执行 | 全量通过，`0 issues` |
| `govulncheck ./...`，在 `backend/` 执行 | 通过；代码调用路径命中 0 个漏洞 |
| pnpm frozen install | 通过；使用 pnpm 9，最终前端 manifest 和 lockfile 相对 main 无变化 |
| 前端 `lint:check` | 通过 |
| 前端 `typecheck` | 通过 |
| 前端完整 `test:run` | 458 个测试文件、3685 个测试通过 |
| 前端 production build | 通过；保留 Vite 的大于 500 kB chunk 提示 |
| pnpm production audit 例外检查 | 通过；使用官方 npm registry |
| Ent/Wire 生成及 diff 审阅 | 通过；Ent 无变化，Wire 保留双方依赖及清理步骤 |
| fork 版本校验及 release-version 脚本测试 | 通过；更新服务测试也已通过 |
| 流式状态复制测试及 64-byte delta benchmark | 通过；benchmark 使用 `-benchtime=1x` 作功能核对 |
| `git diff --check` 及 staged diff 检查 | 通过，无未解决冲突 |
| 双语 README、索引及本记录的本地链接 | 目标文件均存在 |

本机 npm 镜像没有 audit 接口，因此 audit 改用 `https://registry.npmjs.org` 重跑，没有修改依赖或例外规则。govulncheck 仍报告未被当前代码调用的 package/module advisory，不能把调用路径命中 0 个漏洞解释为依赖完全没有 advisory。

验证日志保存在本机 `/tmp/sub2api-v020-checks/`。同步前后的完整 fork 提交及路径清点也保留在本机临时记录中；不把这些日志作为远程 CI 或生产验收证据。

本地未验证 Windows 实际运行、生产启动或真实上游账号调用。本次不触发远程 CI、Release、制品上传或镜像发布。

[返回开发索引](README.md)
