# 仓库目录检查记录

检查日期：2026-10-16。范围覆盖根目录、各应用的主要子目录、文档与工具入口、构建和 CI 路径引用，以及本地生成产物。受版本控制文件与本轮新增路径一并检查；依赖目录和本地配置只核对位置、大小及 Git 忽略状态。本记录是目录与引用检查结果，业务测试和部署验收仍以各自验证记录为准。

## 根目录检查

| 目录 | 检查结果与处理 |
| --- | --- |
| `backend/` | 保留服务源码、Ent、迁移、公开 Plugin API、运行资源与服务端工具；独立的代理归属说明移入 `docs/gateway/` |
| `frontend/` | 保留 Vue 应用、功能模块、同级测试和工具配置；历史审计 JSON 移入开发文档的 `reports/`；配置自动发现问题见后文 |
| `deploy/` | 保留公开安装入口、Compose、Dockerfile、示例配置与部署说明；Caddy 回归脚本归入 `tests/`，同步 CI 路径 |
| `docs/` | 功能模块已有索引；补充仓库说明与检查记录；按用户决定移除 OpenSpec；移除空的本地 `docs/openapi/` 目录，实际契约仍在 `desktop/openapi/` 和 `mobile-bi/` |
| `tools/` | 两个脚本被开发指南、CI 或后端联调测试读取，保留现有路径并新增工具说明 |
| `assets/` | README Logo 有引用；合作方目录内 27 张图片未找到引用，用户确认不需要上游赞助素材后已移除；两份同内容 Logo 的用途已说明 |
| `.github/` | GitHub 安全政策、workflow、审计例外和发布工具归属明确，发布工具路径由 Release workflow 和测试读取 |
| `.agents/` | `skills/` 保留上游同步与发布 Skill；可选的管理 API Skill 已按用户授权移除 |
| `.codex/` | 本地 Hook 配置已被 Git 忽略，保持现有运行位置 |
| `.git/` | Git 元数据由 Git 管理，目录整理通过工作区路径操作完成 |

根目录的 `README.md`、`README_CN.md`、`LICENSE`、`CLA.md` 和 `AGENTS.md` 保留在原位。`.gitignore`、`.gitattributes`、`.dockerignore`、`Makefile`、两个 Dockerfile 和两份 GoReleaser 配置是工具入口或被 workflow 显式引用的配置，保留规范路径。

## 应用子目录检查

| 位置 | 归属与路径约束 |
| --- | --- |
| `backend/cmd/` | 服务入口、资源校验、日志清理、JWT 和利润预览工具，按可执行程序分包 |
| `backend/internal/` | 业务实现、Provider、路由、服务、仓储、配置和测试工具；目录调整需要同步 Go import 与生成代码 |
| `backend/ent/` | Ent schema 与生成代码，跟随 `make -C backend generate` 维护 |
| `backend/migrations/` | SQL 由 `migrations.go` 的 `go:embed *.sql` 读取；迁移文件名和位置具有运行含义 |
| `backend/pkg/pluginapi/` | 面向独立插件的公开协议及 SDK，保留独立公开包边界 |
| `backend/resources/` | 定价 JSON 是后端测试、容器和二进制发布包的运行依赖，保留配置中的相对路径 |
| `backend/scripts/` | 版本解析、版本校验及部署后 SQL 工具有引用，保持原位；SQL 工具的维护窗口限制继续适用 |
| `frontend/src/` | `views`、`router`、`components`、`composables`、`api`、`stores`、`utils`、`types` 与 `i18n` 各有职责 |
| `frontend/src/features/` | 账目、渠道监控 v2 与提示词审计按功能聚合；现有共享组件继续通过 alias 和 import 使用 |
| `frontend/public/` 与 `frontend/src/assets/` | 前者发布静态路径，后者参与 Vite 模块打包，资源的消费方式不同 |
| `deploy/tests/fixtures/` | 部署回归使用的替代 `curl` 和 `container` 命令，不是生产可执行文件 |
| `.github/release-tools/` | 与发布 workflow 一同复制到 runner 临时目录，独立依赖、测试和说明均保留 |

## 已执行的整理

| 原位置 | 新位置 | 引用处理 |
| --- | --- | --- |
| `skills/sub2api-admin/` | 已移除，共 4 个文件 | 可选管理员 API CLI 未被应用、构建、CI 或发布流程调用，按用户授权移除，并清理文档入口与专用 Git 白名单 |
| `openspec/` | 已移除 | 项目不采用 OpenSpec 开发模式；配置、提案、规格、任务、历史证据及临时新增的索引一并移除，清理文档入口和专用 Git 白名单 |
| `backend/dev-docs/upstream-error-proxy-attribution.md` | `docs/gateway/upstream-error-proxy-attribution.md` | 纳入网关文档索引，正文保持原样 |
| `deploy/test-caddyfile-cache.sh` | `deploy/tests/caddyfile-cache-test.sh` | 调整脚本的仓库根目录定位，更新 CI 调用路径 |
| `frontend/audit.json` | `docs/development/reports/pnpm-audit.snapshot.json` | 原始 JSON 保留为历史快照；CI 仍生成新的 `frontend/audit.json`，本地输出加入忽略规则 |
| `assets/partners/` | 已移除，共 27 张图片 | 无仓库消费引用；用户明确说明 fork 不需要上游赞助素材 |

新增素材与维护工具说明，补充 Python 缓存忽略规则。移除的 `backend/dev-docs/` 和 `docs/openapi/` 均为空目录；保留的迁移文件维持原始内容，Caddy 测试脚本仅修改路径定位。

`sub2api-admin` 本身是手动调用管理员 API 的辅助工具。此次移除针对仓库中的 Skill 与 CLI 文件，管理员 API、后台页面和同名 WebSocket subprotocol 保持各自原有用途。

## 保留的入口问题

以下问题已核实路径或工具规则，尚未修改执行行为，后续可以单独修复并运行对应检查。

| 入口 | 证据 | 后续处理 |
| --- | --- | --- |
| `deploy/Makefile` | `make -C deploy -n build` 提示找不到 `./scripts/resolve-version.sh`；计划运行的 `./cmd/server` 也不在 `deploy/` 下 | 明确该入口是否继续提供；保留时应委托 `backend/Makefile` 或显式进入 `backend/`，避免维护两份后端构建流程 |
| `backend/Makefile` 的 `test-e2e` | 展开的命令为 `./scripts/e2e-test.sh`，该文件不存在；`test-e2e-local` 已提供实际 Go 测试命令 | 统一 E2E 入口，并在服务与依赖齐备后实际运行验证 |
| Vite 的本地配置入口 | 本地存在被忽略的 `frontend/vite.config.js`；当前 Vite 的 `DEFAULT_CONFIG_FILES` 中 `.js` 排在 `.ts` 之前，npm 脚本未显式指定配置 | 固定 `--config vite.config.ts` 并核对 TypeScript 的配置文件输出位置，随后执行前端 CI 检查 |

上述 Makefile 只运行了 dry run，没有编译、测试或执行清理命令。Vite 结论来自当前安装版本的配置文件优先级与本地文件状态。

## 素材清理

扫描 README、业务代码和原有文档等消费位置中的路径和文件名，未找到以下 27 张图片的引用；素材清单不计作使用位置。用户明确说明 fork 不需要上游赞助素材，已删除这些图片及空目录。以下清单仅保留清理记录，合计移除 2,619,068 字节。

```text
assets/partners/logos/
├── RoxyBrowser.png
├── aigocode.png
├── aimzoon.jpg
├── apikey-fun.png
├── bestproxy.png
├── bmoplus.jpg
├── cctk.jpg
├── ctok.png
├── duckip.png
├── etok.png
├── fastaitoken.jpg
├── fennoai.jpg
├── haoai.png
├── lanox.jpg
├── nagora.png
├── openmodel.jpg
├── pateway.png
├── pincc-logo.png
├── poixe.png
├── pptoken.png
├── proxy4free.png
├── qiniu.jpg
├── rapidproxy.jpg
├── runapi.png
├── silkapi.png
├── swiftprox.png
└── veilx.png
```

另有一张保留的未引用截图，见 [截图索引](../../screenshots/README.md)。`assets/logo.svg` 与 `frontend/public/logo.svg` 的 SHA-256 相同，但分别服务于 README 与网页，保留两处发布路径。

## 本地产物

检查时，`frontend/node_modules/` 约 740 MiB、`backend/server` 约 150 MiB、`backend/bin/` 约 115 MiB，嵌入的前端输出约 7 MiB。这些路径已被 Git 忽略，大小仅记录本次本地状态。`.DS_Store`、TypeScript 输出、Python 缓存和本地 Hook 配置也按忽略规则保留。

历史审计快照不参与当前安全状态判断；本地二进制、依赖和配置文件没有作为本次目录整理的删除对象。

## 本次验证

| 检查 | 结果与边界 |
| --- | --- |
| CI shell job | workflow 中的 9 条检查全部通过；Apple container 使用测试替代命令验证生命周期 |
| Caddy 脚本 | `sh -n` 通过，从仓库外的工作目录运行也通过 |
| Compose simple mode | 环境静态检查通过 |
| Desktop 对话代理限制 | 固定长度与 chunked 请求的 32 MiB 边界共 4 个用例通过 |
| BI 静态契约 | 4 份规范校验通过；默认 Python 缺少 jsonschema，使用 uv 隔离环境补齐依赖后执行；未进行 HTTP 联调 |
| Markdown lint | 本次新增或调整的 7 份入口与说明文档通过；长行规则按现有表格风格关闭 |
| 文件与链接 | 3 个保留的迁移文件逐一核对内容及权限，Caddy 脚本仅有预期路径变更；移除 Skill 和 OpenSpec 后重新检查文档相对链接 |
| Git 忽略与差异 | 新路径可纳入版本控制，新增输出与缓存规则生效，`git diff --check` 通过 |

本次没有修改 Go 或 Vue 业务源码，未重跑应用的全量 unit、integration、后端 lint 和前端构建。Makefile 的实际构建与 E2E 执行仍未验证，不能将本记录用作这些入口已修复的证明。

[返回开发索引](../../development/README.md) · [仓库目录说明](../../development/REPOSITORY_LAYOUT.md)
