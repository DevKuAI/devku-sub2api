# 账目管理验证记录

验证日期：2026-10-12。工作分支：`codex/accounting-management`。需求基线：`4cab773bda883ad6306e26d30c497d032108497e`。保留原有本地提交，没有 commit、push 或部署。

## 本地自动验证

| 检查 | 结果 |
| --- | --- |
| `make -C backend test-unit` | 通过，全量 unit tests |
| `GOFLAGS=-p=1 make -C backend test-integration` | 通过，全量 integration tests；串行执行 package 控制容器资源占用 |
| `golangci-lint run ./... --timeout=30m` | 通过，0 issues |
| `make -C backend build` | 通过 |
| `pnpm --dir frontend exec vitest run --maxWorkers=4 --minWorkers=1` | 通过，403 个文件、3142 项测试 |
| `pnpm --dir frontend run lint:check` | 通过 |
| `pnpm --dir frontend run typecheck` | 通过，build 同时执行 vue-tsc |
| `pnpm --dir frontend run build` | 通过，包含 locale key completeness 检查 |
| Desktop storage Docker Compose 环境检查 | 通过 |
| `git diff --check` | 通过 |

PostgreSQL 18.1 和 Redis 8.4 测试容器实际启动并执行测试，没有将 Docker 缺失导致的跳过计为通过。人工浏览器 fixture 默认 opt-in，不属于自动集成测试的必跑案例；本次另外启动并完成，最终以 PASS 退出并清理自身数据。

新增回归覆盖税额与币种精度、金额无浮点丢失、并发超额分配、交易号去重、幂等重放与冲突、消费/退款/费用、真实透支、旧卡关闭后退款、退款释放与后续费用占用、资金来源再分配、付款凭证、分次付款和超额授权、AI Invoice、冲正依赖、追加审计失败回滚、合同及发票共享、软删除重试、上传 intent 过期、附件大小/类型/哈希、企业隔离、账号重绑立即撤销旧权限，以及绑定卡片身份不能覆盖历史。

S3 adapter 使用隔离 HTTP 服务验证私有 bucket、对象路径和 300 秒签名有效期；账目业务集成测试使用受控 Storage adapter，不访问生产存储。

## 浏览器验收

使用 ego-browser，前端连接隔离 PostgreSQL 测试环境。身份由测试 fixture 提供，生产 JWT 认证仍使用项目既有实现。

- Admin 可管理全部企业；绑定企业的 Admin 在「我的账户」下显示「财务账目」。
- 普通绑定账户显示「财务账目」，只读页面没有企业切换器、录入、确认、作废、支出或对账管理操作。
- 到账详情不包含收款账号、内部备注或未共享附件；金额和税额与真实数据库结果一致。
- 管理员通过真实页面填写并创建到账草稿，随后列表显示记录，未出现错误提示。
- 桌面、390 px 移动端及暗色模式完成视觉检查；页面没有横向溢出，金额保持完整单行，移动端使用 DataTable 卡片布局。
- 关闭浏览器 TaskSpace，结束人工 fixture 并停止本次启动的 Vite 服务。

截图保存在本次本机临时目录：`/tmp/sub2api-accounting-admin.png`、`/tmp/sub2api-accounting-receipt-detail.png`、`/tmp/sub2api-accounting-customer-detail.png`、`/tmp/sub2api-accounting-mobile.png`、`/tmp/sub2api-accounting-dark.png`。

## 验证边界

未运行远端 CI、未连接生产 S3 bucket、未执行生产迁移或部署。浏览器验证覆盖菜单、只读隔离、查看和草稿创建；退款、付款、冲正、共享及并发边界由 API/数据库集成测试验证，没有声称逐一完成所有浏览器操作。

首次人工 fixture 在代码迭代期间超时，之后重新启动并完成最终验收。早期 Vue 事件格式化和共享路由冲突已修复，修复后相关检查通过。

Vite build 仍提示项目既有的大 chunk 及 Browserslist 数据更新建议；没有为绕过警告更改构建阈值或扩大本次修改范围。

## 2026-10-13：复用资源 bucket

按补充要求，账目附件改为复用 `DESKTOP_UPDATE_STORAGE_RESOURCE_BUCKET`，对象仍存放在 `accounting/{organization_id}/{attachment_id}/` 文件夹。移除单独的 bucket 字段、默认值和部署配置，配置说明已同步。

针对 S3 adapter 的测试验证了共享资源 bucket 中的账目路径、短时效签名下载以及资源 bucket 缺失时不回退至安装包 bucket。相关 repository/accounting tests、Desktop storage 环境配置测试、Docker Compose 环境检查、后端 build、相关 package 的 golangci-lint（0 issues）和 `git diff --check` 均通过。本次不改动前端与账目计算规则。
