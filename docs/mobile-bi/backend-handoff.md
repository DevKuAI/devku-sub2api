# 后端开工交接 · API v1.1

复核日期：2026-09-22。结论：可以按本版契约开始后端实现。59个操作均为拟新增接口，不代表服务已经实现或联调通过。当前小程序仍为演示；本次仅修正文档和离线契约校验，没有改动后端、连接生产或发布。

## 实现入口

1. [OpenAPI 1.1.0](./openapi.yaml)：方法、路径、请求、响应、权限和错误类型的机器契约。
2. [协议](./protocol.md)：微信身份绑定、会话、范围交集、并发、分页和分享语义。
3. [数据模型与指标](./data-model-and-metrics.md)、[采集协议](./ingestion-and-history.md)：表关系、归一化、历史、数据发布和水位。
4. [现有后端能力映射](./sub2api-mapping.md)：哪些代码可适配，哪些是新能力；“有源码”不代表生产已部署。
5. [示例](./examples.json)、[契约边界样例](./contract-cases.json)、[联调验收](./acceptance.md)：分别用于接入参考、离线Schema校验及后端集成测试。

v1.0尚未实施，本次直接修正为1.1.0，路径仍为 `/api/bi/v1`。若已经依据旧文件生成DTO或客户端，需要重新生成；不能继续使用旧的必填字段和不可空整数假设。示例是独立操作片段，关联ID需由测试种子预先建立；它们不是可以从空数据库直接依次重放的业务脚本。

## 本轮修正

| 问题 | v1.1处理 | 实现影响 |
| --- | --- | --- |
| 成功示例含ok:false | Ack固定ok:true，错误用非2xx | 确认绑定、退出、收藏成功语义一致 |
| 缺数据却只能返回整数 | 相关统计计数可为null；Overview.assets无知识权限时为null | DTO须保留null，不转换为0；getAssetAnalysis要求knowledge:read |
| 成员应用、模型、反馈应用数组无界 | 3个现有GET补limit/cursor及next_cursor/has_more；未知应用以null组表示 | 汇总不分页，明细分页；默认20、最多100 |
| 筛选项缺少联动上下文 | getFilterOptions增加可选context_id；listScenes增加knowledge_id | 不在小程序下载全部数据后过滤 |
| 案例只有版本ID，无法构造版本路径 | CaseDetail.knowledge_versions返回knowledge_id及version_id | 仅输出DTO变化；导入仍使用标准版本ID |
| 来源读取缺少父关系参数 | getSource必填parent_kind/parent_id，校验父对象与来源双层权限 | 历史来源正文按父关系固定的version_id读取 |
| 阅读权限可列出分享链接 | listReportShares改为reports:share | 分享解析同时检查创建者和接收者当前权限 |
| 导入游标可被并发或旧批覆盖 | ImportBatch新增必填expected_checkpoint，每source最多一个在途批次 | 新键并发409；同键重试幂等；失败不能推进checkpoint |
| 回灌未完成仍被迫声明完整水位 | complete_through允许初次回灌为null | initial_backfill_complete=true必须有有效水位 |
| 异步/错误边界不全 | 补批次级错误、500错误码、通用响应头、分页和任务状态约束 | 批次错误record_index可为null；日志用request_id定位 |

## 开工顺序与交付边界

### 1. 身份和企业授权

实现公开bootstrap、微信code交换、绑定审批/兑换、refresh/logout、me、企业列表及绑定撤销。前端调用Taro.login，后端调用微信code2Session；现有网页微信OAuth不可替代此流程。

最低存储关系：微信绑定唯一键 `(appid, openid)`、带期限和状态的绑定挑战、可撤销会话/刷新凭证、企业ManagerGrant及授权版本。公开管理者ID与sub2api账号映射保留在服务端，不能将员工手机号或模型API Key当作管理身份。

原管理账号网页需要新增确认绑定和撤销入口。复用原登录、验证码及2FA；绑定审批使用Sub2apiBearer，小程序使用MobileBearer，采集使用ConnectorBearer，三个入口必须校验不同audience。首个交付闭环是“真实微信 → 本人管理账号确认 → 有权企业 → 退出/撤权后不可读”，不是只返回openid。

### 2. 事实层、快照和总览

建立组织映射、成员/主部门/岗位历史、Eligibility、应用/场景、调用事实、标准Token及导入批次。接入首个测试企业的已授权数据后，提供data-status、analysis-contexts、overview、adoption、tokens、trend和团队/成员/应用/场景查询。

首次数据导入的目录、应用、版本循环引用可在同一批原子校验。数据发布用事实/outbox → 构建读取视图 → 发布revision/推进checkpoint，避免查询读到半批数据。ACL拒绝覆盖层独立于旧context的数据快照，撤权不能等待旧快照过期。

测试数据至少覆盖人员交互、自动执行、未知归因、失败调用、缓存Token、离职/调岗、历史不完整和没有调用的完整日期。用网关调用日志对账，不能将rolling30d当自然月，也不能将sub2api group当部门。

### 3. 知识、报告和跨会话保存

实现知识/版本/案例/来源/评估接入、带权限检索、个人收藏/补充反馈、报告快照、分享创建/解析/撤销。来源正文按版本保留，当前ACL对历史正文和报告同样生效。

应用/场景归因、知识引用和评估结果必须来自实际生产者；缺少来源应标未知，不为填满UI编造数据。这些功能属于当前首发范围，不能将阶段2完成标成整个产品上线完成。

### 4. 联调与发布准备

执行[验收清单](./acceptance.md)的数据、权限、并发及微信设备用例。提供受控测试企业和审核操作说明；配置真实AppID、合法HTTPS request域名与隐私信息。接口完成与备案、认证、代码审核分别记录状态。

## 错误响应

所有错误遵循Error对象；错误正文request_id与响应头相同，不返回令牌或内部堆栈。枚举允许的代码不代表每个操作都能出现所有代码，应使用该操作列出的HTTP状态。

| HTTP | code | 约定 |
| --- | --- | --- |
| 400 | INVALID_ARGUMENT | 字段、微信code、非法组合或业务参数错误 |
| 401 | UNAUTHENTICATED | 会话无效，最多一次刷新/重新登录 |
| 403 | FORBIDDEN、CONTEXT_REVOKED | 企业/功能无权或上下文撤权 |
| 404 | NOT_FOUND | 对象不存在、跨企业、不可见、分享过期/撤销/创建者失权；统一返回 |
| 409 | CONFLICT、IDEMPOTENCY_CONFLICT、BINDING_CONFLICT、CHECKPOINT_CONFLICT、IMPORT_BUSY | 状态、幂等、绑定或导入冲突，不能无条件覆盖重试 |
| 410 | CONTEXT_EXPIRED、BINDING_EXPIRED | 上下文/分页快照或绑定挑战过期 |
| 412 | PRECONDITION_FAILED | note版本冲突，保留本地草稿 |
| 413 | PAYLOAD_TOO_LARGE | 请求体超限 |
| 428 | PRECONDITION_REQUIRED | 缺少创建/修改/删除所需的条件头 |
| 429 | RATE_LIMITED | 按Retry-After等待 |
| 500 | INTERNAL_ERROR | 内部异常，输出诊断ID |
| 503 | DATA_UNAVAILABLE、UPSTREAM_UNAVAILABLE | 查询或依赖暂时失败，按可重试标记处理 |

绑定尚未审批为202/PendingBinding，不作为Error；异步导入已受理后发生的数据错误放在ImportJob.errors。导入失败任务的record_index=null表示批次级问题，数字为0起始记录下标。

## 实施时必须落实的信息

这些信息不妨碍接口和数据库开发，但缺失时不能声称真实联调或上线完成：

| 信息 | 需要落实的内容 |
| --- | --- |
| 微信及服务环境 | 实际AppID、服务端保管的AppSecret、测试/生产HTTPS域名；凭证不进入文档和小程序 |
| 企业管理关系 | 测试企业、真实管理账号与组织映射、首批Owner授权；其他角色由现有企业授权流程显式配置 |
| 数据来源 | 组织历史/Eligibility维护方，调用与应用场景归因生产者，知识/案例/评估来源、同步频率及授权范围 |
| 运行部署 | BI作为sub2api模块还是独立服务、任务执行器、读取视图发布方式；路由和DTO不随部署方式改变 |
| 数据处理 | 实际保留期限、来源撤回、解绑与原账号注销/个人数据清除流程及责任人；不能将解绑当作删除企业业务数据 |

本文件不规定新的账户注册或通用后台产品；授权维护复用已有企业后台。原文中的24个月保留及性能数字是待落实的设计目标，不是已完成的容量测试或生产配置。

## 验证命令

在devku-mobile目录执行：

```bash
python3 -m venv /tmp/devku-contract-check
/tmp/devku-contract-check/bin/pip install -r docs/api/tools/requirements.txt
/tmp/devku-contract-check/bin/python docs/api/tools/validate_contract.py
```

结果写入[validation-report.json](./validation-report.json)。此检查覆盖文档契约、示例、权限声明、参数与Schema边界；不会访问后端，也不能证明权限中间件、数据库事务或微信真机行为正确。
