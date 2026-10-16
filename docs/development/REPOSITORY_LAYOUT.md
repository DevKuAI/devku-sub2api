# 仓库目录说明

仓库根目录保留应用代码入口、构建与发布配置、README 和许可证。按用途查找文件时，从下表开始。

| 位置 | 内容 |
| --- | --- |
| `backend/` | Go 服务、数据库迁移、服务端工具和嵌入的 Web UI 构建产物 |
| `frontend/` | Vue 应用、前端配置与测试 |
| `deploy/` | 安装脚本、容器与代理配置、部署文档和部署测试 |
| `docs/` | 按功能模块组织的指南、API 契约、设计与验证记录 |
| `.agents/skills/` | 项目开发与发布 Agent skills，包含参考文档和元数据 |
| `.github/` | CI、安全扫描、发布 workflow 与发布辅助工具 |
| `tools/` | 仓库维护和契约检查工具，见 [工具说明](../../tools/README.md) |
| `assets/` | README 使用的图片与品牌素材，见 [素材说明](../../assets/README.md) |

## 应用代码与测试

后端入口位于 `backend/cmd/`；业务实现、路由、服务和仓储位于 `backend/internal/`。`backend/pkg/pluginapi/` 是独立插件的公开接口。`backend/ent/` 的生成代码、`backend/migrations/` 的嵌入迁移及 `backend/resources/` 的定价回退数据均由编译、测试或容器打包读取，按各自现有流程维护。

前端页面和路由位于 `frontend/src/views/`、`frontend/src/router/`，组件、组合式函数和 API 分别位于 `components/`、`composables/`、`api/`。`features/` 保存账目、渠道监控和提示词审计的功能模块。前端测试与模块放在同级的 `__tests__/` 中；后端 Go 测试与包内实现相邻。

`deploy/tests/` 保存部署脚本和静态配置回归测试，`fixtures/` 保存测试使用的替代命令。与发布 workflow 紧密耦合的脚本及其测试位于 `.github/release-tools/`，通用维护工具位于 `tools/`。

## Agent skills

[sync-upstream-release](../../.agents/skills/sync-upstream-release/SKILL.md) 保存上游同步与 fork 发布流程。

Skill 的参考文档和脚本与 `SKILL.md` 保存在同一目录中。使用或分发 Skill 时，保留完整目录。

## 功能文档

功能说明、API 契约和持续维护的验证记录按对应模块保存在 `docs/` 中，由 [文档索引](../README.md) 提供入口。固定提交的原始设计、退役链路和一次性检查记录集中放入 [历史归档](../archive/README.md)。历史验证记录只说明当时的结果，当前行为以实际实现与验收结果为准。

## 本地文件与生成产物

`.codex/`、`docs-local/` 和 `plugins/` 属于已被 Git 忽略的本地配置、资料或独立插件工作区。`node_modules/`、`backend/bin/`、`backend/server` 和 `backend/internal/web/dist/` 属于依赖或生成产物；Web UI 目录中的 `.keep` 是受版本控制的占位文件。

`frontend/vite.config.js`、`frontend/vite.config.d.ts`、`*.tsbuildinfo` 和 Python 缓存为本地产物。Vite 自动选择配置的行为及检查时发现的 Makefile 入口问题见 [2026-10-16 目录检查记录](../archive/development/2026-10-16-repository-audit.md)。

原先受版本控制的 `frontend/audit.json` 已移至 [历史审计快照](../archive/development/pnpm-audit.snapshot.json)，仅保留原始记录，不代表当前依赖安全状态。安全扫描仍在 CI 中生成 `frontend/audit.json` 并即时校验；本地生成的同名文件由 Git 忽略。

新增文档时，放入对应模块并更新 [文档索引](../README.md)。调整路径时，检查 `.gitignore` 白名单、文档链接、构建配置和脚本引用，避免文件移动后无法纳入版本控制或被工具找到。

[返回开发索引](README.md) · [返回文档索引](../README.md)
