# 上游 v0.2.21 同步记录

本次同步采用上游精确标签 `v0.2.21`，fork 源码版本为 `0.2.21.0`。本次仅进行本地合并和验证，不推送、不创建 fork 标签或 GitHub Release。

## 来源

| 项目 | 值 |
| --- | --- |
| 上游仓库 | `Wei-Shaw/sub2api` |
| 上游标签 | `refs/tags/upstream/v0.2.21` |
| 上游 tag object | `43f15294c477bec427d586ddfc3f918ce1284c98` |
| 上游 peeled commit | `10ba8f9b4dc2e6023d4a9879ab77b21841babd10` |
| Fork 仓库 | `DevKuAI/devku-sub2api` |
| 同步前 main / origin/main | `48a0148a0837c1dfcf6d43078a5f21dcb0d1b27e` |
| 合并基点 | `dab829a4e9b7d1cc307189aea8b0319eb29e29d0` |
| Fork 基线版本 | `v0.2.21.0`，未创建标签 |

合并前已清点相对目标的 239 个 fork 独有提交及 1098 个差异路径。上游带来 82 个提交、200 个路径变更，其中 45 个路径与 fork 定制重叠。使用 `git merge-tree --write-tree --messages` 预演后进行三方合并，共处理 14 个冲突文件。没有纳入 `upstream/main` 在该标签之后的 2 个提交；合并结果包含 fork 定制，不与上游标签源码相同。

## 保留范围

- 保留 Desktop 企业组织、部门、岗位、成员、托管 API Key、客户端模型配置、资源、更新、会话及报表。
- 保留 BI 独立身份、微信接入、运营管理、周期报表和原生联调入口。
- 保留账目模块，包括渠道账单导入、卡消费唯一入口、合同、订单、对账与凭证。
- 保留个人 API Key 创建数量限制、托管 Key 身份、请求正文记录及查询、按张计费和订阅账号只读额度查询。
- 保留 DevKu 蓝色主题、双语 README、更新与安装地址、镜像仓库及 fork 发布 workflow。

Desktop、BI、账目业务目录及发布配置未因本次同步发生变更。管理员代登录测试的 stub 需要适配上游新增的 Refresh Token 消费方法，生产代登录接口保持原有行为。

## 集成与冲突处理

- 新增 Claude Fable / Mythos 5.2 模型、推理配置、签名保留及兜底价格；显式模型映射和白名单仍需管理员按需调整。
- 用量日志同时保留 fork 的 `request_body` 和上游新增的 `cache_creation_30m_tokens`、`cache_creation_12h_tokens`。同步更新单条、批量及 best-effort 写入、查询扫描和测试参数位置，避免字段错位。
- 扩展上游缓存 TTL 集成测试，在上述三种写入路径验证缓存分桶、正文可用标记和正文原文同时往返成功。
- 保留迁移 `249_add_usage_logs_api_key_cost_index_notx.sql` 和 `250_openai_cache_write_ttl.sql`。迁移按完整文件名排序和记录，与 fork 已有同数字前缀文件共存；没有改写既有迁移。仅去掉新增 TTL 迁移末尾多余空行，以通过 `git diff --check`。
- WebSocket 采用上游 `IsFirstTokenEvent` 回调，替代 fork 的等价 `MessageStartsTTFT` 接口。保留并适配原有语义输出／空 delta 回归测试，同时接入逐轮计时和 `codex.*` 事件覆盖。
- 分配测量测试采用上游的独立子进程实现，继续隔离其他测试启动的后台 goroutine，不放宽零分配断言。
- API Key 列表保留 fork 的创建限额加载，接入上游用量独立加载；统计聚合不再阻塞列表显示。
- Refresh Token 轮转采用上游原子消费方法；补齐 fork 代登录测试 stub 的对应方法，保留身份、会话和审计断言。
- 重新执行 Ent/Wire 生成，生成结果与自动合并后的生成文件一致，没有额外生成漂移。
- 双语 README 同步新增功能、图片输出格式与计费边界；保留 [Codex 原生 Images 状态及代理超时说明](../OPENAI_IMAGES_CODEX.md)，并加入媒体文档入口。请求体预算、显式配置优先级及部署注意事项见 [边缘安全说明](../../deploy/EDGE_SECURITY.md)。

## 宣传内容核对

相对同步前 main 没有文件删除，没有新增 `README_JA.md`、推广图片、赞助或推荐链接；新增 URL 仅为测试 fixture 和官方技术文档。DevKu 更新、安装、镜像和仓库配置保持不变，保留中立上游署名。

## 验证

验证环境使用 Go `1.27.3`、以 Go `1.27.3` 构建的 golangci-lint `2.14.0`、Node.js `24.21.0`、pnpm `9.15.9` 和 Python `3.12.16`。Go 和 pnpm 通过 mise 显式选择；Release helpers 使用临时虚拟环境中的 `PyYAML 6.0.3`。后端测试移除外部 `OPENAI_API_KEY`；集成测试设置 `DESKTOP_REQUIRE_WEB_HTTP=1`，使用本地 Docker 测试容器。

| 检查 | 结果 |
| --- | --- |
| CI shell job 全部命令 | 通过 |
| Compose simple mode、Desktop conversation proxy | 通过 |
| Release helpers、镜像脚本语法、audit checker 自测 | 通过；分别为 12 个和 4 个 Python 测试 |
| `env -u OPENAI_API_KEY make -C backend test-unit` | 全量复跑通过 |
| `env -u OPENAI_API_KEY make -C backend test-integration` | 全量复跑通过；启用 Web HTTP 联调 |
| BI Native workflow 的两条 Go test 命令 | 均通过；使用 `-count=1` 重新执行 |
| `golangci-lint run --timeout=30m ./...`，在 `backend/` 执行 | 最终版本全量通过，`0 issues` |
| `govulncheck ./...`，在 `backend/` 执行 | 通过；代码调用路径命中 0 个漏洞 |
| pnpm frozen install、`lint:check`、`typecheck` | 通过 |
| 前端完整 `test:run` | 458 个测试文件、3706 个测试通过 |
| 前端 production build | 通过；保留 Vite 大于 500 kB 的 chunk 提示 |
| pnpm production audit 例外检查 | 通过；使用官方 npm registry |
| Ent/Wire 生成及 diff 审阅 | 通过，无额外生成漂移 |
| 后端 build | 通过 |
| 缓存 TTL 与请求正文往返集成回归 | 三种写入路径通过；真实 PostgreSQL / Redis 容器，未跳过 |
| 版本脚本、未解决冲突及 diff 空白检查 | 通过；新增文档本地链接目标均存在 |

单元测试首轮发现代登录测试 stub 未适配新的 Token 消费方法，修复后单独测试及全量单元测试均通过。Release helpers 初次缺少 PyYAML，安装 workflow 指定依赖后复跑通过。lint 复跑曾遇到本地 Go 可执行路径和 GOROOT 混用，显式固定 Go `1.27.3` 路径、GOROOT 和 `GOTOOLCHAIN=local` 后，最终全量检查通过；没有修改源码来绕过工具问题。

集成测试首轮被 Docker 下载 PostgreSQL 镜像的重试阻塞。通过主机从官方 registry 下载同一 `postgres:18.1-alpine3.23` ARM64 镜像，校验 manifest、config、9 个压缩层摘要及解压后的 layer diff ID 后导入 Docker；停止本次卡住的下载和测试进程后，完整集成、BI Native 和专门的三路径回归均通过。没有替换成其他 PostgreSQL 版本，也没有修改 Docker mirror 配置或源码检查规则。官方平台 manifest 为 `sha256:70c585d0e816fc96a4a3a0c938170db6cbe2af35f28bbd6997c5291c3ec64239`，config 为 `sha256:91f1036a9148b028c43669532b2f1c7eed3c8b63a7e92abea98b70388b27dccb`。

pnpm audit 原始退出码为 `1`，两个 high advisory 均为仓库已有且仍有效的 `xlsx` 例外，最终例外检查通过。扫描同时报告 moderate / low advisory，不能解释为依赖零漏洞。govulncheck 还报告未被当前调用路径使用的 package / module advisory，不能把调用路径命中 0 个漏洞解释为依赖完全没有 advisory。

日志、完整 fork 提交／路径清点及双方 diff 保存在本机 `/tmp/sub2api-v021-checks/`，不作为远程 CI 或生产验收证据。

本地未验证浏览器交互、生产启动或真实上游账号调用。本次不触发远程 CI、Release、制品上传或镜像发布。

[返回开发索引](README.md)
