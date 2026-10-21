# Sub2API 开发指南

本文说明 DevKu fork 的开发环境、验证命令、代码生成和版本规则。所有 shell 示例从仓库根目录执行；进入子目录的命令使用独立子 shell，避免改变后续命令的工作目录。部署与运行配置见 [deploy/](../../deploy/README.md)。

## 仓库与工具版本

| 项目 | 当前要求 | 配置来源 |
| --- | --- | --- |
| Fork | `DevKuAI/devku-sub2api` | `origin` remote |
| 上游 | `Wei-Shaw/sub2api` | `upstream` remote；Go module/import 路径继续使用此名称 |
| Go | `1.27.3` | [go.mod](../../backend/go.mod) 与 CI 的版本断言 |
| Node.js / pnpm | Node.js `20` / pnpm `9` | [CI](../../.github/workflows/backend-ci.yml) 与 [Release](../../.github/workflows/release.yml) |
| golangci-lint | `2.14.0` | CI lint job |
| PostgreSQL / Redis | 当前 Compose 为 PostgreSQL `18` / Redis `8` | [docker-compose.yml](../../deploy/docker-compose.yml) |
| 后端 / 前端 | Gin、Ent、SQL Repository / Vue、TypeScript、Vite、Pinia | `backend/` 与 `frontend/` |

工具版本更新时，以仓库配置为准。macOS 已由 mise 管理的环境继续使用 mise，避免另装重复的 Node 或全局包管理器。pnpm 9 用于当前 lockfile；不要因本地版本不兼容而重写 lockfile。

安装 lint 工具：

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
```

## 开发与构建

### 前置条件

1. 配置可连接的 PostgreSQL 和 Redis，数据库账号、密码与地址以本地配置为准。
2. 使用上述 Go、Node.js 和 pnpm 版本。
3. 安装前端依赖：

```bash
pnpm --dir frontend install --frozen-lockfile
```

全新环境先执行 CLI setup，再启动后端：

```bash
(cd backend && go run ./cmd/server --setup)
(cd backend && go run ./cmd/server)
```

在另一个终端启动前端：

```bash
pnpm --dir frontend run dev
```

Vite 默认在端口 `3000` 提供页面，并代理到 `localhost:8080`。后端地址不同时设置 `VITE_DEV_PROXY_TARGET`。`go run` 不提供后端自动热重载。

### 编译

```bash
# Build the backend only.
make -C backend build

# Build the frontend assets.
pnpm --dir frontend run build

# Embed the built frontend in the backend binary.
(cd backend && go build -tags embed -ldflags="-X main.Version=$(./scripts/resolve-version.sh)" -o bin/server ./cmd/server)
```

前端输出到 `backend/internal/web/dist/`。不带 `-tags embed` 的二进制不提供 Web UI；构建输出被 `.gitignore` 排除。

## CI 与本地验证

| Workflow | 触发与职责 |
| --- | --- |
| [CI](../../.github/workflows/backend-ci.yml) | push / pull_request；部署脚本、Go unit/integration、前端关键测试、lint 和 Release helpers |
| [Security Scan](../../.github/workflows/security-scan.yml) | push / pull_request / 每周定时；govulncheck、pnpm audit 与例外检查 |
| [Release](../../.github/workflows/release.yml) | `v*` tag / workflow_dispatch；校验 fork 版本、选择构建矩阵、构建和发布；支持 dry run |

常规验证：

```bash
env -u OPENAI_API_KEY make -C backend test-unit
env -u OPENAI_API_KEY make -C backend test-integration
(cd backend && golangci-lint run --timeout=30m ./...)
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run test:run
pnpm --dir frontend run build
git diff --check
```

- Integration tests 需要 Docker 和实际启动的测试容器；跳过不能计为通过。
- `make test-frontend` 运行 lint、typecheck 和关键 Vitest 子集；`test:run` 运行完整套件。
- `lint:check` 不自动修改文件；`lint` 带 `--fix`，执行后检查 diff。
- 有效的外部 `OPENAI_API_KEY` 会改变部分测试路径；常规 CI 对齐验证使用上面的 `env -u` 命令。
- 部署脚本、Desktop proxy 和 Release helpers 的命令直接见 CI workflow，避免在文档中维护另一份清单。

安全检查与 CI 保持一致：

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
(cd backend && govulncheck ./...)
pnpm --dir frontend audit --prod --audit-level=high --json > /tmp/sub2api-pnpm-audit.json || true
python3 tools/check_pnpm_audit_exceptions.py --audit /tmp/sub2api-pnpm-audit.json --exceptions .github/audit-exceptions.yml
```

pnpm audit 的退出码由例外检查继续判定；例外必须与 advisory、包名和有效期匹配。不能通过忽略最终检查来隐藏失败。

## 生成代码与接口契约

修改 Ent schema、Wire provider 或其消费者后执行：

```bash
make -C backend generate
git diff -- backend/ent backend/cmd/server/wire_gen.go
```

先解决源文件，再重新生成；不要独立手改生成代码。新增 Go interface 方法时，同时更新相关 stub/mock。Ent 在当前 Go toolchain 下把部分 `json.RawMessage` 字段生成为 `jsontext.Value` 属于已有生成行为，仍需审阅 diff。

Desktop OpenAPI 见 [契约索引](../desktop/README.md#openapi-契约)，由对应 route tests 读取。BI OpenAPI 位于 [mobile-bi/](../mobile-bi/README.md)，当前 BI 通过路由契约和实际 TypeScript HTTP 联调核对，不沿用已退役的内嵌 schema。修改路由、DTO 或契约时，运行对应的现有测试。

## Git 与 fork 版本

`origin` 是 DevKu fork，`upstream` 是上游仓库。检查当前工作区和 remote 后再选择同步目标：

```bash
git status --short --branch -uall
git remote -v
git fetch --prune origin
git fetch --prune --no-tags upstream
```

指定上游版本时同步精确 tag，不替换为更新的 `upstream/main`。具体流程见 [sync-upstream-release](../../.agents/skills/sync-upstream-release/SKILL.md)，保留 Desktop、BI、账目、API key 限额、品牌及 fork 发布配置。

- 上游使用三段版本，如 `v0.2.16`；fork 基线为 `v0.2.16.0`。
- 同一上游基线的本地迭代递增第四段；采用下一上游版本时，第四段重置为 `0`。
- 源码构建读取 `backend/cmd/server/VERSION`；release tag 构建优先使用 tag。修改版本文件不等于发布。
- 四段版本不是标准 SemVer。发布前由 `backend/scripts/release-version.sh --fork` 校验；Release workflow 的具体 GoReleaser 调用以当前配置为准。
- 每个发布创建新的 tag，保留已有版本和下载文件。更新检测和回滚按数字比较第四段，历史三段版本按修订号 `0` 处理。
- 同步、local commit、push 和发布分别执行；本地同步不会自动 push、创建 tag 或发布 Release。

只计算基线版本，不修改 refs：

```bash
sh backend/scripts/release-version.sh --upstream v0.2.16
```

## 常见问题

| 现象 | 检查与处理 |
| --- | --- |
| frozen install 失败或 lockfile 格式不兼容 | 核对 pnpm 9；依赖调整时审阅并提交对应 lockfile 改动 |
| Go 版本断言或容器构建失败 | 核对 `go.mod`、三个 workflow 和 `Dockerfile`、`deploy/Dockerfile`、`backend/Dockerfile` 的 Go 镜像版本 |
| interface 编译报 `missing method` | 搜索对应 stub/mock，补齐新增方法并跑相关测试 |
| 修改 schema 或 provider 后行为未变 | 执行 `make -C backend generate`，审阅生成文件 |
| `localhost` 连接优先尝试 IPv6 | 确认服务监听地址；仅监听 IPv4 时使用 `127.0.0.1` |
| Windows 没有 make | 在 `backend/` 中使用 `go test -tags=unit ./...` 和 `go test -tags=integration ./...` |
| 批量编辑后模型无法调度 | 核对账号平台、模型映射、分组 allowlist 和实际错误；按平台处理配置，避免跨平台覆盖 |

[返回开发索引](README.md) · [返回文档索引](../README.md)
