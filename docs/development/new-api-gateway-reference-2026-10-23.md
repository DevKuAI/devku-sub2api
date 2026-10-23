# new-api 基础网关参考与企业化设计差异

日期：2026-10-23。本文为 [企业 AI 网关从零建设方案](standalone-platform-architecture-2026-10-23.md) 的源码研究附件；目标项目的技术栈仍未限定，首版实施范围以主方案为准。

参考范围是 new-api 的基础网关能力和实现机制。新平台完整保留当前定制业务，仅承接相关原版依赖，部署在亚太；美西原版 sub2api 与 DeepSeek 等服务均作为独立渠道。一个企业可以绑定多个渠道，每个企业—渠道绑定使用一份活动上游凭据，员工只持有新平台凭据。本文不构成选用 new-api 作为代码底座或增加一层 new-api 服务的决定。

迁移范围见 [定制业务与依赖清单](current-feature-preservation-2026-10-23.md)：130 个定制业务条目、必要依赖及工程检查。new-api 的额外能力与 sub2api 无关的原版能力都不自动成为需求。原版充值、套餐、退款、返利、完整个人门户和渠道运营后台不迁移；定制企业财务与供应商订阅账号扩展仍完整保留。

## 1. 研究基线与证据范围

| 项目 | 核实结果 |
| --- | --- |
| 官方仓库 | `QuantumNous/new-api` |
| 源码基线 | 固定提交 `7fb6e57a5ff671582131c74e00bdfaa867d4e985`，提交时间为 `2026-10-23 00:23:08 +08:00`；下文源码链接均指向该提交 |
| Release 元数据 | 2026-10-23 查询 GitHub latest Release API 得到 `v1.0.0-rc.46`，`prerelease=false`、`draft=false`；发布时间为 `2026-10-22 00:25:54 +08:00`。标签名称和 API 状态分别记录，不据 `rc` 字符串改写发布状态 |
| 版本关系 | 分析对象为上述 main 提交，未将 main 的全部能力归入 Release 标签 |
| 许可 | 固定提交的 `LICENSE` 为 GNU AGPL v3；第 13 条包含修改版本通过网络提供服务时的对应源码要求；`NOTICE` 还列出署名和原项目链接保留条款。后续代码复用或 fork 选型应单独核对适用许可义务 |
| 验证方式 | 官方仓库只读 checkout、固定 SHA 源码交叉阅读、GitHub Release API 回读；没有启动 new-api，没有调用收费模型，没有完成真实协议联调或两机房网络测试 |

来源：[固定提交][S01]、[Release API][S02]、[LICENSE][S03]、[NOTICE][S44]。

## 2. 可以作为功能基线的能力

下表「源码行为」表示固定提交中可确认的实现路径，不代表每种渠道、模型、部署配置均已实测支持。

| 能力 | new-api 源码行为 | 新平台采用方式 |
| --- | --- | --- |
| 渠道管理 | `Channel` 保存类型、Base URL、Key、模型、Group、优先级、权重、状态、自动禁用、模型映射、参数与 Header 覆盖；另有多 Key 状态与轮询信息 [S04] | 参考渠道配置与运维能力；将企业凭据从 Channel 移到 Binding，首期不采用共享多 Key 池 |
| 平台 Token | `Token` 关联 `UserId`，包含有效期、状态、额度、模型限制、IP 白名单、Group 和 Auto Group 配置；鉴权检查用户状态、IP 与可用 Group [S05][S06][S07] | 保留凭据生命周期与约束，归属改为本地企业及员工／服务主体；模型、渠道范围取授权交集 |
| 模型映射 | 将客户端模型沿映射链解析为上游模型，检测循环；路由模型与上游模型分别保存 [S08] | 平台 `LogicalModel` 映射到 Binding 下的 `ChannelModel`，保存映射版本和实际执行快照 |
| 模型可达索引 | `Ability` 按 Group、Model、ChannelId 建索引，并保存启用状态、优先级和权重；由 Channel 的模型列表与 Group 列表展开 [S09][S35] | 参考预计算候选集合；索引必须包含企业绑定和能力版本，不能以此替代企业授权 |
| 优先级与权重 | 先筛选可用候选，按优先级降序选择，同一优先级内随机加权；重试索引影响优先级选择 [S10][S11] | 企业路由首期采用主备优先级；仅在已批准的等价候选中使用权重 |
| 自动选择与跨组重试 | Token 的 `auto` Group 可解析有序候选 Group，并受用户可用范围约束；跨组重试可配置 [S12][S13] | 显式的企业 RoutePolicy 和候选 Binding 列表；员工不能自行扩展企业渠道范围 |
| 协议 Adapter | Interface 分别处理 Chat、Responses、Claude、Gemini、Embedding、Image、Audio 等转换与响应；异步任务有独立 TaskAdaptor [S14] | 参考按操作适配和共用传输层，保留「声明能力、准备、执行」接口与可选管理能力，不强制所有渠道实现同一组管理 API |
| 聚合型与直连渠道 | 注册表分别注册 DeepSeek、sub2api、new-api；sub2api Adapter 复用 new-api Adapter [S15][S16] | 直接佐证 sub2api 可以是普通渠道类型；DeepSeek 从亚太直连，其调用路径不经过美西 sub2api |
| 重试和渠道健康 | 集中记录重试决策，按错误、状态码、重试次数、渠道固定约束与会话模式决定重试；支持自动禁用、测试恢复 [S17][S18][S19][S20] | 借鉴可解释策略与健康状态；补充执行阶段、结果未知、预算和企业 Binding 范围 |
| 预扣与结算 | 请求估价后预扣；BillingSession 按实际消耗计算差额并结算或退款；Token 预留有原子检查路径 [S21][S22][S23] | 采用预占、结算、补差思想；实现持久化企业多维预算状态与可恢复补偿 |
| 用量与日志 | 日志保存用户、Token、渠道、模型、Group、平台 Request ID、上游 Request ID；区分本地计量、上游计量和估算来源 [S24][S25] | 增加企业、员工、用途、Binding、Attempt、凭据代次、价格版本及证据可信度，区分业务账和渠道成本 |
| 请求限流 | ModelRequestRateLimit 按用户计数，配置独立 Group 限流时按用户＋Group 计数；分别统计总请求和成功请求，具有 Redis／内存路径 [S26] | 企业、员工、Token、Binding 分层限流；多实例使用一致计数，单独实现并发和 Token 速率等已确认需求 |
| 运营管理 | Channel API 提供 CRUD、测试、余额刷新、优先级、状态、标签、模型同步；Token、日志、统计、导出有独立 API 和权限控制 [S27][S28] | 参考功能范围，采用平台运营者与企业管理员分权；所有创建、编辑、绑定及策略配置使用独立路由页面 |

## 3. 必须保留的领域差异

### 3.1 Group 是资源与计价分组，不能充当企业

new-api 的 Group 参与可用渠道／模型筛选、Token 可选范围、自动 Group 顺序，以及用户 Group 到执行 Group 的价格倍率。鉴权中确实有 Group 权限校验，不能把它描述为「没有权限控制」。但 `Ability` 主键没有企业或企业绑定，Channel 的 Key 直接属于渠道，Token 则属于用户；这些结构不表达本项目要求的「企业—渠道—专属凭据」关系。[S04][S05][S06][S09][S13]

本项目应采用以下关系：

```text
Organization
  ├─ Member / ServicePrincipal → ClientCredential
  └─ OrganizationChannelBinding → Channel
          ├─ active CredentialVersion
          ├─ authorized ChannelModels
          └─ quota / rate / routing constraints
```

`Channel` 表示接入端点和 Adapter，`Binding` 表示企业有权使用该渠道的凭据及约束。员工增加或停用不创建渠道用户，也不增删上游 Key。业务数据、缓存键、路由候选、异步任务和查询条件均以可信企业 ID 约束；客户端传入的 Group、渠道 ID 或 Header 不能改变企业身份。

### 3.2 Ability 是路由索引，完整能力合同需另建

new-api 已有请求路径、Task Plugin 和 Responses WebSocket 等过滤器，不能笼统说它只匹配模型名。固定提交中，普通渠道的 `request_path` 过滤直接通过，Advanced Custom 才按路径配置进一步判断；Responses WebSocket 则另查开关和渠道类型。这套约束与「每模型逐项验证 tools、输入类型、状态引用、计量定义」仍是不同层级。[S29]

新平台的能力判断至少应覆盖操作、传输、输入输出、tools、structured output、reasoning、状态对象、计量和管理接口，分别标记 `declared / verified / unsupported / unknown`。新增模型或读取到 `/models` 不自动使其进入生产路由。

源码也明确存在能力边界：DeepSeek Adapter 的 Gemini、Audio、Image 转换返回 `not implemented`；路由中 `/files` 和部分 Fine-tuning 路径指向 `RelayNotImplemented`。因此参考基础网关不能转写为「全部 OpenAI API 都可兼容」。这些事实仅描述该 Adapter／路由的实现，不推断供应商产品本身是否支持相关功能。[S30][S31]

RelayKit 将协议数据建模、转换与宿主的 HTTP、路由、鉴权和计费分开，转换结果带 `Quality`、`Steps`、`Diagnostics`，这条模块边界适合作为设计参考。源码还提供 `allow / safe / strict` 损失策略：默认 `allow` 允许有损转换并返回诊断，`safe` 拒绝改变工具执行语义的请求转换，`strict` 进一步拒绝其他损失；工具转换实际调用该策略检查。新平台应把关键语义损失作为拒绝条件，保留诊断用于审计，不能将默认允许转换视作无损兼容。采用相同模块边界不等于已经选定 Go 或 RelayKit 依赖。[S38][S39][S42][S43]

### 3.3 权重语义应由新平台统一定义

固定提交的内存路径按配置权重选择，在全部权重为零时改为等权；数据库路径给每个权重加 `10` 再选。两条路径的有效分配并非完全相同，不应把实现中的具体概率直接作为新平台产品合同。[S10][S11]

后续引入跨渠道主备与权重时，`priority` 决定主备层级，`weight` 只在同一层级、同一能力合同且企业明确允许的候选内生效。主方案将权重定义为正整数、默认 1，停用通过状态表达；缓存与数据库采用同一选择语义。试点首版采用固定平台模型映射，sub2api 内部主备和调度由原版运行，只验证其对定制会话和计量的影响；路由记录用于说明模型、渠道和配置版本。

### 3.4 会话亲和不能替代对象归属

new-api 的亲和规则可以从可信 Context、Header 或请求 JSON 取值，再按配置加入规则名、模型、Group 组成缓存键，具有 TTL 和严格会话模式。[S32][S36]

新平台应区分两种用途：无状态请求的亲和可以帮助稳定渠道与缓存；有状态对象必须由 `provider_objects` 持久化绑定企业、所有者、Binding、凭据代次与远端 ID。亲和缓存失效不能导致 `previous_response_id`、文件或异步任务被转发到其他企业或渠道。亲和键必须包含可信企业范围，不能仅采用客户端提供的会话字符串。

## 4. 预算、计量与重试的改造要求

### 4.1 预扣机制可以借鉴，企业预算不能只复制 BillingSession

new-api 的预扣并非对每个请求无条件执行：免费模型跳过预扣；当 `ForcePreConsume=false`、信任额度阈值有效且大于零、Token 无限额或 Token 额度大于阈值、钱包用户余额也大于阈值时，可以将有效预扣额设为零。订阅来源明确不走这条信任旁路。钱包路径创建 Session 时仍先检查用户余额大于零且不少于预估预扣额，因此「跳过实际预扣」不能表述为「没有余额准入检查」。[S21][S34][S40]

BillingSession 使用进程内锁与状态标记，资金来源和 Token 分两步结算；资金已提交而 Token 更新失败时记录错误并防止误退款。Refund 通过后台 goroutine 执行，失败记录日志。Token 原子预留解决单维额度的并发检查，但不等于企业、员工、用途、Binding 的持久化联合预算事务。[S22][S23]

新平台需要的合同如下：

- 所有收费候选先完成企业与适用子预算的原子预占；硬预算默认不使用信任余额旁路。
- `reservation_id`、`request_id`、`attempt_id` 与结算幂等键持久化；进程重启后可恢复结算和退款。
- 重试前重新检查候选价格和可用预占；替代渠道更贵时补充预占失败则停止，不绕过企业限额。
- 未确认执行或费用的 Attempt 保留 `unknown / pending_reconciliation`；上游已经执行的费用不能因为客户端失败而自动归零。
- 企业业务费用与渠道供应商成本分别记账。响应 usage、本地估算、sub2api 用量及供应商账单作为证据合并到同一费用事件，不能重复扣账。

### 4.2 集中重试策略需要增加执行阶段门槛

new-api 的集中 `DecideRelayRetry` 检查严格会话、渠道固定约束、Channel Error、SkipRetry、次数、系统排除与配置状态码；系统排除包含 `504`、`524` 和 `BadResponseBody`。该函数本身不检查「上游请求是否可能执行」「客户端是否已收到业务输出」。部分协议 Handler 具有自己的流式终止和错误处理，因此本次静态检查不能据此断言所有已输出流都会被重试。[S17][S18]

本项目应把重试条件做成必经门槛：

| Attempt 状态 | 默认行为 |
| --- | --- |
| 请求尚未发送，或有可靠证据表明未执行 | 在企业授权、能力、预算和次数均允许时换候选 |
| 已发送但结果未知 | 记录未知执行状态，等待可用查询或对账；不透明重新生成 |
| 已向客户端输出业务内容 | 返回协议允许的中断结果；不拼接另一个渠道的新生成 |
| 已创建上游状态对象／任务 | 保持原 Binding；恢复或查询使用同一对象归属 |
| 明确的模型／参数不支持 | 返回明确错误；仅在发布策略允许、且没有执行时选择满足同一合同的候选 |

主备策略和安全重试是两件事：配置了备用渠道不表示所有失败都能自动切换。亚太到美西链路中断尤其需要保留未知状态，避免重复生成和重复渠道成本。

### 4.3 健康状态按影响范围隔离

new-api 可根据错误类型、配置状态码及关键字禁用渠道；自动测试还可以根据耗时阈值禁用，并在测试成功后恢复自动禁用渠道，手动禁用渠道被排除。[S19][S20][S37]

新平台应按失败原因更新最小范围：企业 Key 失效隔离该 Binding；某模型不支持隔离对应 ChannelModel；端点整体不可达才影响 Channel；员工预算不足不影响渠道健康。恢复探测需要明确测试模型、凭据、频率和成本上限，恢复后再逐步放流。亚太观测到的跨区耗时不能直接解释为供应商计算性能。

### 4.4 限流范围必须落到企业主体

本次核对的 ModelRequestRateLimit 使用用户或用户＋Group 计数，不能把名称理解为已经提供企业、成员、Binding 的全部限流组合。[S26]

新平台准入顺序为企业 → 员工／服务主体 → ClientCredential → Binding；分别记录拒绝原因。RPM、Token 速率、并发数和长期金额预算使用各自的计量单位，不能互相替代。多实例计数应保持一致；不要以每个进程的独立内存计数承诺全局硬限制。

## 5. 功能参考与迭代时机

下表说明实施顺序。MVP 用于内部验证或受控试点；分阶段不缩减定制业务。正式切换验收覆盖定制条目、适用依赖和已启用的新网关能力，不要求复制 sub2api 或 new-api 的全部功能。

| 工作范围 | 内容 | 实施时机 |
| --- | --- | --- |
| 企业 AI 基本流程 | 企业员工、Channel／Binding、平台 Token、固定模型映射、sub2api 原生接入、用量与管理页面 | 首版；数据关系支持多绑定，真实 DeepSeek 集成后续加入 |
| 基础正确性 | 企业授权、凭据隔离、必要限流、企业预算预占、幂等结算、未知状态和持久化恢复 | 与首版同时完成，不能用「简化设计」省略 |
| 定制调用依赖 | Desktop／报告所需协议、模型、价格口径、员工权限、计量和相关账号操作 | 按具体定制条目承接；渠道内部引擎和完整运营后台继续由原版管理 |
| 新增网关能力 | 新 Adapter、当前不存在的协议、跨渠道主备／权重和高级配置 | 有明确接入需求后增加，不豁免已有相似能力 |
| 新增运营自动化 | 平台远端开户联动、额外自动探测和批量发布编排 | 根据明确需求实施；不以此引入原版完整账号运营产品 |
| 定制业务重建 | 部门岗位、Desktop、对话、报告、BI、企业财务、账号扩展、提示词、代登录与其他定制 | 按主方案逐模块交付和迁移；必要身份和权限同时承接 |

new-api 的多 Key 轮询、额外请求覆盖和模型部署仅作参考。企业—渠道一份活动凭据的约束保持；原版账号池、供应商运维和插件管理留在渠道，不列为新平台必迁任务。尚未启用的协议明确拒绝；若某项定制客户端合同依赖该协议，则在该客户端切换前实现并验收相应路径。

## 6. 管理页面与 API 的落点

new-api 的运营 API 与界面覆盖连接、模型、路由映射、请求响应设置、状态、测试和日志；可参考它的信息组织与运维动作。其渠道创建／编辑使用 `ChannelMutateDrawer`，该容器形式不适用于本项目现行表单规范。[S27][S28][S33][S41]

本项目按照现有表单规范使用独立路由页面。建议页面为「渠道」「企业渠道绑定」「模型目录」「企业模型路由」「访问凭据」「预算与用量」「请求与 Attempt」「对账任务」。平台运营者维护渠道端点及 Adapter；企业管理员在获准范围内管理 Binding、用途与预算；员工仅查看自己的凭据和用量。

建议管理 API 以领域资源组织，例如 `channels`、`organizations/{id}/channel-bindings`、`logical-models`、`organizations/{id}/model-routes`、`client-credentials`、`ai-requests`、`budget-accounts`、`reconciliation-jobs`。这些是新项目设计资源名，不能直接当作 new-api 或 sub2api 已存在的接口。渠道测试、凭据轮换、模型同步和策略发布作为显式操作保存审计与任务状态。

## 7. 必须进入验收的场景

这些场景按对应能力的启用阶段执行；首版只实施固定平台路由时，不提前建设跨渠道自动主备。企业隔离和已启用的预算／计量必须在首版验证；原版功能仅验收其对定制流程的必要影响。

1. 企业 A 与 B 绑定同一个渠道实例，分别持有上游凭据；伪造企业 ID、Group、渠道 ID 和会话字符串不能访问对方的模型、对象、日志或预算。
2. 企业 A 同时绑定 sub2api 和 DeepSeek，普通员工与报告服务使用不同平台 Token；归属可区分，渠道 Key 仍按 Binding 共享。
3. 模型名称相同但 tools、状态引用或传输能力不同，路由正确排除不满足请求合同的渠道。
4. 同优先级权重、零权重、缓存重建和多实例路由遵循同一公开规则；主备切换不越过企业授权和预算。
5. 发送前连接失败、发送后超时、首个业务事件之后断流、取消请求、WebSocket 中断均留下正确 Attempt 与费用状态。
6. 预占后进程重启、结算部分失败、重复结算、补偿失败及账单后到时，企业账与渠道成本可恢复和对账。
7. 一个企业的无效 Key 不自动停用其他企业对同一 Channel 的正常绑定；手动停用不被自动探测恢复。
8. 亚太入口到美西 sub2api、亚太直连 DeepSeek 分别测量首响应、首 Token、完整耗时、SSE／WebSocket 连续性及大请求行为。
9. 同企业共享渠道 Key 后，定制员工累计／窗口额度、用量及已启用对象权限仍独立；响应全部丢失或进程重启后可恢复归属，不能用永久 unattributed 代替定制员工统计。
10. 原版 Backend 禁止普通用户自助时，定制订阅账号查看者仍通过本地身份、授权与 Adapter 完成绑定账号查询和额度操作；不要求重建原版支付或个人门户。

本轮未执行上述运行验收。文档交付验证仅包含 Markdown 结构、相对链接、固定源码引用与行号的本地校验及 `git diff --check`；未运行应用 test、lint、build，不能据本文认定网关已可上线。

## 8. 固定源码索引

正文中的 `Sxx` 链接均指向下面的来源。重点入口为 [Channel][S04]、[Token][S05]、[路由选择][S10]、[Adapter][S14]、[重试决策][S17]、[BillingSession][S22] 和 [管理路由][S27]；Release API 是本次读取的动态元数据来源，其结果已在第 1 节固定记录。

[S01]: https://github.com/QuantumNous/new-api/commit/7fb6e57a5ff671582131c74e00bdfaa867d4e985
[S02]: https://api.github.com/repos/QuantumNous/new-api/releases/latest
[S03]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/LICENSE#L540-L550
[S04]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/model/channel.go#L23-L71
[S05]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/model/token.go#L14-L35
[S06]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/middleware/auth.go#L591-L645
[S07]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/model/token.go#L222-L253
[S08]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/relay/helper/model_mapped.go#L14-L68
[S09]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/model/ability.go#L18-L25
[S10]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/model/channel_cache.go#L117-L216
[S11]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/model/ability.go#L108-L172
[S12]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/service/channel_select.go#L114-L203
[S13]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/service/group.go#L14-L134
[S14]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/relay/channel/adapter.go#L18-L82
[S15]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/relay/relay_adaptor.go#L50-L128
[S16]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/relay/channel/sub2api/adaptor.go#L1-L17
[S17]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/service/relay_error.go#L18-L60
[S18]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/setting/operation_setting/status_code_ranges.go#L17-L38
[S19]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/service/channel.go#L28-L93
[S20]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/controller/channel-test.go#L933-L969
[S21]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/relay/request_billing.go#L22-L81
[S22]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/service/billing_session.go#L29-L150
[S23]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/model/quota_reserve.go#L200-L239
[S24]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/model/log.go#L57-L101
[S25]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/service/billing_usage.go#L8-L67
[S26]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/middleware/model-rate-limit.go#L172-L208
[S27]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/router/channel-router.go#L40-L102
[S28]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/router/api-router.go#L282-L359
[S29]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/model/channel_constraint.go#L91-L139
[S30]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/relay/channel/deepseek/adaptor.go#L27-L62
[S31]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/router/relay-router.go#L168-L180
[S32]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/service/channel_affinity.go#L290-L350
[S33]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/web/src/features/channels/components/drawers/channel-configuration.tsx#L95-L109
[S34]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/service/billing_session.go#L203-L361
[S35]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/model/ability.go#L225-L246
[S36]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/service/channel_affinity.go#L590-L625
[S37]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/controller/channel-test.go#L1120-L1134
[S38]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/relaykit/README.md#L1-L48
[S39]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/relaykit/types/conversion.go#L24-L74
[S40]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/service/billing_session.go#L396-L414
[S41]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/web/src/features/channels/components/channels-dialogs.tsx#L31-L58
[S42]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/relaykit/relayconvert/internal/toolconv/encode.go#L17-L49
[S43]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/relaykit/relayconvert/request_registry.go#L24-L45
[S44]: https://github.com/QuantumNous/new-api/blob/7fb6e57a5ff671582131c74e00bdfaa867d4e985/NOTICE#L6-L25
