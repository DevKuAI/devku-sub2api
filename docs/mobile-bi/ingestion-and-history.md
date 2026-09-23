# 数据采集、同步与沉淀接口

本设计先明确产品需要哪些事实，不因现有来源难以收集而删减目标。缺失事实时展示未知，不用推断替代采集。Schema见 [ImportBatch](./openapi.yaml)，来源复用见[映射表](./sub2api-mapping.md)。

## 1. 数据从哪里来

| 数据域 | 拟定事实生产者 | 可信规则 |
| --- | --- | --- |
| 企业/成员/主部门/岗位/可使用范围 | 企业组织系统 + sub2api企业映射 | 组织负责人维护；生效区间保留历史，不由小程序填写 |
| 模型调用与Token | sub2api网关或已授权调用入口 | 用服务端调用ID与API Key历史映射；raw编码版本明确 |
| 人员主动/自动执行、应用/场景 | 发起执行的受信客户端/工作流服务 | 网关call/trace ID关联；未知场景保留；场景自动分类标SceneRecord.classification=inferred，actor_type不靠猜测 |
| 使用评分 | 原执行交互的实际人员 | 一交互当前一票，验证调用归属；与管理者知识note分开 |
| 知识与版本 | 企业现有知识库或维护入口 | 正文、适用范围、维护者、状态、版本、来源ACL；草稿不得标有效 |
| 案例及引用 | 原工作入口与实际执行轨迹 | 应用依赖配置只说明可能引用；必须记录实际knowledge_version_id |
| 版本评估 | 既有评估服务或受控导入 | 固定测试集、标准版本、逐样本结果；不能把API成功当业务通过 |

服务端接入凭证的签发/轮换由运维凭证系统管理，不由小程序API创建。凭证固定 `organization_id`、`source_id`、允许记录kind；请求体不能设置企业ID。目录、用量、知识维护来源可各用不同凭证。企业角色的授权不属于数据导入，不能借组织同步给自己增加管理权限。

## 2. 对外接入流程

1. 从生产者增量游标读取变化，转换成 `ImportRecord`。初次回灌先目录与成员关系，再来源及知识/应用和对应版本，再案例、调用、评价、引用与评估。应用/知识的current_version与版本归属存在循环引用时，必须放同一批：先登记全批ID，再校验关系，最后原子提交；不能按数组顺序要求前一条已入库。
2. `submitImportBatch`：数据批次1–500条（水位心跳允许0条）、UTF-8 JSON最大2MiB；必填 `Idempotency-Key`。服务器先验证凭证、来源、格式并持久化受理记录，返回202和batch_id。
3. 调用 `getImportBatch` 查看 queued→validating→applied/rejected。全批跨对象验证通过后原子写入事实与事务outbox；聚合和索引按revision构建完成后，在同一发布事务切换可读revision、标记applied及推进checkpoint。不能把SQL、搜索引擎和异步队列假定成一个分布式事务；未发布中间态不得被读取，重启可按outbox幂等恢复。
4. 只有 applied 才推进 `getImportCheckpoint` 的 source checkpoint 与 complete_through。rejected不发布业务事实或推进水位；清理未发布暂存修订，逐项返回record_index/code，不返回其他企业对象信息。生产者修复后使用新幂等键提交；同键异体409。
5. 网络重试用相同key；服务按source+entity_kind+entity_id+revision去重，不依赖24小时幂等缓存实现长期去重。旧revision忽略且返回已应用语义，相同revision不同payload拒绝。批次部分已存在、部分新增时仍可原子应用，但任何语义冲突全批拒绝。
6. 接入故障恢复从最后已应用checkpoint继续；不得把“已受理”当“已入库”。每source只允许一个queued/validating批次；已有批次未终结时，不同幂等键返回 `409 IMPORT_BUSY`，相同键同体返回原受理结果。生产者等待applied或rejected后再提交，防止后批游标越过失败前批。

`complete_through=null` 仅用于尚未建立可信水位的初次回灌，此时 `initial_backfill_complete=false`；开始有完整水位后不得清空。`complete_through` 非null时是生产者声明该时刻之前事实已完整同步，并非最大事件时间；不能超过服务接收时间，不能仅因新事件晚到就倒退水位。`initial_backfill_complete` 表示覆盖 `history_start_date` 至水位的已知历史回灌完成；若企业启用早于history_start_date，首次使用仍可能不可确认。正常无新事件也可提交records为空且带complete_through/checkpoint的水位心跳批次，不能靠空用量数组宣布无业务。

## 3. 记录约束

`expected_checkpoint` 是生产者读取到的最后已应用游标（首次为null），`checkpoint` 是本批成功后的非空新游标。受理新批次时在source锁内比较expected与当前游标，不匹配返回 `409 CHECKPOINT_CONFLICT`，不比较字符串大小。同幂等键同体重放先查原结果，不因原批已推进水位而拒绝。幂等域为企业+source+operationId+key，不随接入凭证轮换变化；相同目标checkpoint换新key不得重复推进，返回409并要求读取当前checkpoint。rejected释放source占用且不推进游标，修复数据后从同一expected游标以新key提交。生产者必须包含该游标之前的全部变化，服务端无法从不透明字符串证明上游完整性。

每批的完整水位、history_start_date和初始回灌状态必须跨字段校验。初始回灌完成为true时必须有非null水位；history_start_date不得晚于水位所在上海自然日。回灌完成后不得退回false或将历史起点向后移动来伪装完整；增加更早历史时按新的数据修订记录。空records批次也必须有新checkpoint并遵守这些约束。批次级错误的 `record_index=null`，逐记录错误使用0起始下标。任务可在租约失效后恢复；不可恢复异常置rejected/INTERNAL_ERROR并释放source，不允许永久卡在validating。已发布的ACL拒绝覆盖层属于独立安全状态，后续索引失败不得把权限放开；保留拒绝至修复重放，记录审计。

所有upsert包含 `kind`、正整数revision与类型化payload；不是任意JSON。导入ID在凭证登记的来源命名空间中稳定生成，服务端校验归属；跨来源引用由受信适配器先做ID映射，已存在实体不得更换所属来源。目录记录按时间有效，valid_to>null时必须>valid_from，主归属区间不能重叠。已应用不可变知识/应用版本不允许换正文；内容修订必须新version_id。知识版本的正文、来源映射、valid_from不可改；生命周期valid_to允许以更高revision从null关闭为合法结束时间，并历史化，新context采用新有效期，旧快照不被改写。关闭不等于抹除过期版本的历史引用。当前knowledge记录可切换状态和current_version，但每次变化保留历史。

使用事件以一次实际调用尝试为粒度，`retry_of`可以指向先前尝试；重试产生的Token应计费事实统计，重复投递不重复。human须有可信member_id，automatic/unknown须member_id=null。nullable维度用unknown桶，关联ID非空但不存在时 INVALID_REFERENCE，不私自创建应用或员工。

Token encoding为exclusive_buckets、inclusive_input或unavailable。前两种四个数均有值并通过跨字段约束；unavailable全为null。只有明确测得0才可填"0"。非Token图像/视频调用照样记requests。未来时间超过接收时刻5分钟拒绝；轻微时钟误差标记，当前未完成日不进入完整周期。

引用版本须在发生时可用，超前版本拒绝。迟到引用如果依赖已知事件可以补发新record，不反向修改已发布版本。相同usage+knowledge_version去重；同一usage引用不同版本保留证据，但知识级引用次数按event去重。

rating只允许实际human调用的人员提交，补评按usage发生期归属，rated_at保留真实评价时间。管理小程序的知识note是另一张表，不纳入helpful_rate。

评估样本ID唯一、两边同一问题和标准、source case属于同企业；通过数由样本算，不允许上传汇总分数覆盖样本。跨版本测试集不一致建立不同evaluation，比较页面显示不可比。

## 4. 更新、撤回与权限

`tombstone` 包含实体类型、id、生效时间与原因。只允许对该source拥有的实体写入；跨来源转移所有权需后台显式映射。知识/来源删除保留引用占位，源正文和搜索摘要停止返回，受影响报告实时ACL检查失败。删除普通调用错误数据以新的修订标记撤回，统计产生新data_revision，保留审计，不写负Token抵扣。

来源首次导入时以 `(organization_id, SourceRecord.id, version_id)` 保留不可变正文（SourceRecord.id是内容来源对象，区别于采集凭证的source_id）；同版本内容变更拒绝。导入知识版本/案例时，在同批待提交视图或已发布视图解析source_ids，持久保存当时的来源version_id。知识版本的映射此后不随来源新版本变化，案例每个修订也保留映射；getSource经父关系选定历史正文，仍实时校验来源当前ACL。

来源ACL同知识ACL独立；正文读取必须两层都满足。ACL收紧在该批完成格式、归属和语义校验后，先原子发布拒绝覆盖层，再异步更新索引；即使新数据revision未发布，旧context/报告也必须读取该覆盖层。批次受理202不代表撤权已经生效；生产者只有收到applied或独立的撤权处理确认才能声称完成。无效批次不得造成部分ACL变更。ACL开放不能超越企业ManagerGrant范围。

`organization_readable=true` 仅表示本企业已授权人员可读此对象，不是公网可读。空ACL为拒绝。关联对象失权后，关联标记为通用不可访问，不泄露名字、客户或不存在的链接。事件聚合所需业务事实与可见源正文是两类授权；有总量权限不代表有聊天全文权限。

## 5. 晚到、修正与报告

历史回补和组织纠错产生新的data_revision；旧context保留原快照至30分钟到期，授权撤销例外即时失效。报告冻结原revision及metric_version，不被回填覆盖。用户可生成新报告做修订版；本版不提供就地编辑报告API。

预聚合至少按企业/自然日/事件主团队/actor/application/scene/model保存用量与调用；活跃人数不能从各桶人数直接求和，要按person去重或使用可正确去重的集合。留存保留首用与周活跃事实；数据保留截止造成观察不全时标明history_incomplete。

上线对账同时验证Token、人数、请求、引用、版本和权限。只对Token总数相等不代表采用率或知识复用正确。
