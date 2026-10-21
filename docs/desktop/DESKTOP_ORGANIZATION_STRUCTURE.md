# 企业部门、岗位与员工归属

本功能在现有 Desktop 企业内管理单层部门、岗位和员工归属。平台管理员进入企业详情的「成员 → 组织架构」；企业负责人使用「我的企业」中的同一入口。创建、编辑、调岗和导入均使用独立路由页面。

## 归属与状态

- 一个岗位只属于一个部门，创建后不能直接修改所属部门；不同部门可以使用相同岗位名称。
- 一个未删除员工只能有一个当前岗位，同一岗位可以有多名员工。停用员工仍保留岗位。
- 新增员工必须选择启用部门下的启用岗位。部门是选择条件，服务端由岗位确定实际部门。
- 岗位有未删除员工时不能停用，包含已停用员工。部门停用前需转移员工，并停用下属岗位。
- 部门和岗位保留历史引用，当前不提供删除接口。停用后的名称仍保留，重新使用时恢复原对象。
- 企业停用期间组织结构只读。岗位归属不改变 Model Token、预算、登录身份或模型分组权限。

调岗通过单独页面完成。服务端比较归属版本，在同一事务内关闭旧记录、建立新记录和更新当前岗位；旧页面的过时提交返回冲突。姓名、手机号、备注和预算编辑不会产生调岗记录。

## 存量员工与历史

升级不会根据姓名、备注、对话内容或 Gateway 分组推断岗位，也不会自动创建默认部门。历史员工可以暂时显示「待分配」，正常 AI 使用不因待分配状态停止；新建员工不能缺少岗位。

管理员或企业负责人建立真实部门和岗位后，对照确认名单为存量员工分配岗位。首次确认从本次操作开始记录，`source` 为 `migration_confirmed`；不声称该岗位在迁移前已存在或已生效。

员工列表、筛选和 CSV 导出显示当前归属。导出的用量可以有统一截止时间，但当前归属不随 `as_of` 冻结。旧会话、日报、周月总结和成员用量不回写；本期没有按历史部门重新归集用量或生成部门日报。

## 导入

新模板在姓名、手机号和备注之外增加部门、岗位列。系统按规范化名称精确匹配本企业启用的部门与岗位；未知、停用或不匹配项按行报错，不自动创建组织结构。

旧模板不含归属时，在导入页面明确选择整批部门与岗位。提交前显示每一行的最终归属。导入沿用逐行结果和幂等重试；已经成功的行不重复执行。

## API

管理员前缀为 `/api/v1/admin/desktop/organizations/{organization_id}`；企业负责人前缀为 `/api/v1/desktop/organization`。后者由 Web 身份确定企业，不接受客户端更换企业范围。

两类入口均提供部门、岗位的查询、创建、详情和编辑，员工详情、岗位变更历史，以及以下操作：

| 接口 | 用途 |
| --- | --- |
| `GET /organization-structure/status` | 查询待分配人数、历史不一致人数和最终约束状态 |
| `POST /members` | 创建员工，`position_id` 必填；沿用成员创建的幂等 Key |
| `GET /members` | 支持 `department_id`、`position_id` 和 `unassigned=true` 筛选 |
| `PUT /members/{member_id}/position` | 传入 `position_id`、`expected_revision`、可选 `reason`，并提供 `Idempotency-Key` |

部门和岗位创建同样要求幂等 Key。编辑需要对象的 `expected_revision`，不能通过岗位编辑修改部门，也不能通过普通成员编辑偷偷修改岗位。读取和重放均检查当前企业访问范围，企业停用后不能重放写入操作。

创建员工新增必填字段属于管理 API 的兼容性变化。升级时同步更新 Web 与直接调用成员创建 API 的脚本；旧调用缺少岗位时返回 `422`，不提供永久空岗位兼容分支。Desktop 客户端登录和会话协议、BI capabilities 保持现有范围。

完整字段、状态与错误响应见 [Desktop OpenAPI](openapi/DESKTOP_API_V1.openapi.yaml)。

## 实际 Web client 联调

后端 integration 的 `TestDesktopStructureHTTPActualTypeScriptClient` 启动与生产共享路由注册的 Go HTTP 服务，运行 [真实 TypeScript client 用例](../../frontend/tests/desktop-structure/client.http.ts)。请求经过原有 Axios client、拦截器、JSON DTO 和真实 PostgreSQL；Web 身份由测试夹具提供，JWT 认证继续由既有认证回归覆盖。

用例包含部门与岗位创建、员工创建及预算、成员归属、幂等重放、过时版本冲突、更名后的历史名称保留，以及结构化错误。CI 的后端与 Native Integration job 安装 Node.js 20／pnpm 9 前端依赖，并设置 `DESKTOP_REQUIRE_WEB_HTTP=1`，依赖缺失即失败。

本地从仓库根目录执行：

```bash
pnpm --dir frontend install --frozen-lockfile
(cd backend && DESKTOP_REQUIRE_WEB_HTTP=1 go test -tags=integration ./internal/repository -run TestDesktopStructureHTTPActualTypeScriptClient -count=1)
```

该用例使用独立 Vitest config，不混入常规前端 unit 测试，也不会打入生产包。

## 分阶段部署与数据库收紧

首次发布只自动执行 [267 扩展 migration](../../backend/migrations/267_desktop_organization_structure.sql)，新增表、可空岗位字段、复合外键和唯一索引。不能把最终约束脚本直接移入自动 migration 目录：现有员工需要先确认真实归属。

上线前列出成员创建的全部写入方。在计划窗口内同步切换新后端、Web 和脚本，确认没有旧实例继续创建无岗位员工。员工正常 AI 请求不属于成员创建写入窗口。

所有企业的未删除员工补齐后，检查当前岗位、历史版本及开放历史一致。单个企业的 `status=complete` 不等于全库最终约束已经启用；`constraint_enabled` 明确返回数据库状态。

有数据库维护权限的部署人员在完成上述检查后，单独执行以下脚本。连接串使用待维护数据库的实际值；执行前确认目标环境。

```bash
psql '<PostgreSQL connection string>' \
  --set ON_ERROR_STOP=1 \
  --single-transaction \
  --file backend/migrations/deferred/desktop_member_position_required.sql
```

[最终约束脚本](../../backend/migrations/deferred/desktop_member_position_required.sql) 不在启动时嵌入的 `*.sql` 范围内。脚本先在企业表取得 EXCLUSIVE 锁，让已有企业写事务完成，再锁定成员等关联表；新的组织写入及对话写入会短暂等待，普通查询可继续。该顺序与应用先锁企业的事务一致，避免与调岗关闭历史再更新成员的过程形成锁循环。脚本在发现任一未分配或历史不一致员工时失败并回滚；通过后添加并验证 `desktop_members_position_required`。执行成功后重新读取状态，`constraint_enabled` 应为 `true`。

全新数据库没有历史映射工作，也应在正式接入成员前执行同一收紧步骤。迁移前已软删除且无可靠归属的员工保持历史未知，不为其建立占位岗位。

最终约束启用后，旧二进制的空岗位插入会失败。回退使用兼容新 schema 的修复版本；不能直接回退旧写入程序，也不能通过清空映射或删除约束隐藏失败。

## 验证边界

数据库约束、调岗并发、事务回滚、HTTP 权限、幂等和字段投影由真实 PostgreSQL integration 验证；页面、导入、返回与未保存状态由 Vitest 及浏览器检查验证。本地页面测试数据不代表生产企业资料。

本次开发没有修改生产数据库、执行真实员工映射、启用生产最终约束、提交或发布。具体运行结果在实施方案的验证记录中列出。
