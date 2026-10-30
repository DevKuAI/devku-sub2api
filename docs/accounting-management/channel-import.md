# 渠道账单导入

本文是设计方案，尚未实现。目标是把虚拟卡平台等渠道的账单直接导入账目，替代卡消费的逐条手工录入，并让我方在渠道中的头寸、消费和手续费一目了然。首个接入平台是 SUNRATE，后续平台按同一模板接入。资金链路和 2026-10-28 的核对结果见 [SUNRATE 资金架构](sunrate-funds-architecture.html)。

## 业务口径

2026-10-28 确认：

- SUNRATE 账户 `72778936` 属于 Hong Kong Rich Sum Technology Co. Limited（即香港阜森科技有限公司），账户内资金全部是**我方自有资金**。
- 账户下有两类余额：**收付款余额**（Payment Balance，按币种分 HKD、USD）和**卡余额**（Card Balance，USD，多卡共用）。资金链路为：HKD 入金 → 购汇为 USD → 划转到卡余额 → 企业卡消费。
- 入金的汇款方是我方自有资金来源，**不是客户付款**，不进入企业账。
- **一个企业一张卡**，平台侧卡昵称即企业名称。
- 入金手续费、授权手续费（包括拒付和零金额验卡产生的部分）**由我方承担，记为成本**，不转嫁企业。购汇不收手续费，成本体现在汇率中。
- 导入后**直接入账**，不设预览确认步骤。
- 导入的卡消费挂入该企业**最近一个未关闭的订单**。
- 入金等流水可以补充**凭证**（转账截图或 PDF）作为备份；没有凭证的入金单独提示。

由此形成两层账：

| 层级 | 视角 | 内容 | 核对对象 |
| --- | --- | --- | --- |
| 渠道账 | 我方头寸 | 入金、购汇、余额间划转、卡消费、退款、手续费、调整 | 平台各余额的对账单期末余额 |
| 企业账 | 客户 | 企业卡消费、退款，沿用现有订单、卡和资金分配 | 企业收款 |

入金、购汇和划转都是我方资金在渠道内的移动，不形成客户收入，也不创建到账记录。手续费只进入渠道账。

## 对账需要导出的报表

每次对账从 SUNRATE 导出以下报表，一起拖入导入页。导出区间从上次导出的结束日期往前多留几天即可：重叠部分按平台单号自动跳过，不会重复记账。

| 报表 | 导出时选择 | 格式 | 必需 | 用途 |
| --- | --- | --- | --- | --- |
| 收付款余额对账单 Statement of Payment Balance | Currency 选 **HKD**，区间覆盖本次范围 | PDF | 是 | HKD 入金、卖出购汇及余额 |
| 收付款余额对账单 Statement of Payment Balance | Currency 选 **USD**，同一区间 | PDF | 是 | USD 买入购汇、划转到卡及余额 |
| 卡余额流水 CARD_BALANCES | 同一区间 | xlsx | 是 | 划入、卡消费结算、授权费及余额 |
| 授权明细 AUTHORISATIONS | 同一区间 | xlsx | 是 | 卡昵称、卡号后四位、商户、授权交易号；拒付统计 |
| 入金记录 Record_Collection | 同一区间 | xlsx | 有入金时 | 入金毛额、手续费、汇款方 |
| 购汇记录 Record_Currency_Conversion | 同一区间 | xlsx | 有购汇时 | 汇率和精确时间 |

手续费明细 `SUNRATE_FEES_*.xlsx` 不需要导出。收付款余额以后出现其他币种时，按币种各导出一份对账单。

## 整体流程

```
平台原始报表 ─→ 转换器 ─┐
                       ├─→ 标准账单模板 ─→ 校验 ─→ 渠道流水（去重、配对）─→ 企业卡消费
人工整理的模板 ─────────┘
```

1. 财务在导入页一次上传一组文件。系统按文件名和表头识别平台；能识别的原始报表由对应转换器转成[标准账单模板](statement-template.md)，模板文件直接使用。
2. 按模板规则校验。格式错误、余额不连续或购汇无法成对时整批拒绝，不写入任何数据。
3. 校验通过即入账：新流水写入渠道账，已导入的去重键跳过；购汇和划转完成配对。
4. 卡消费和退款按卡绑定关系计入企业账；无法确定企业或订单的流水照常进入渠道账，标记为待归属，进入待办。
5. 返回导入结果：新增、重复、待归属行数，以及每个余额与平台对账单期末余额的核对结果。

新平台只需新增转换器，入账逻辑和表结构不变。没有转换器之前，可以先手工整理模板导入。

## 数据模型

新增 6 张表，不改现有记录表结构。渠道相关表不带 `organization_id`，因为渠道账属于我方。

**`accounting_channel_accounts` 渠道账户**

| 字段 | 说明 |
| --- | --- |
| `id`、`platform`、`account_ref` | `(platform, account_ref)` 唯一；首次导入未知账户时自动创建 |
| `name`、`remark` | 显示名称，SUNRATE 取对账单 `Account Name` |
| `created_at`、`created_by` | |

账户下的余额由流水的 `wallet + currency` 区分，不单独建表。

**`accounting_channel_cards` 渠道卡**

| 字段 | 说明 |
| --- | --- |
| `id`、`channel_account_id`、`card_ref` | `(channel_account_id, card_ref)` 唯一；首次出现时自动创建 |
| `card_last4`、`card_name` | 取最近一次导入的值 |
| `organization_id`、`vcc_card_id` | 绑定的企业和系统卡，未绑定时为空 |
| `bound_at`、`bound_by`、`bind_method` | 绑定方式：`AUTO_CARD`、`AUTO_NAME`、`MANUAL` |

**`accounting_channel_imports` 导入批次**

| 字段 | 说明 |
| --- | --- |
| `id`、`converter` | 使用的转换器，模板直导为 `TEMPLATE` |
| `files` | 文件名、sha256、附件 ID 列表；原始文件存为附件 |
| `reported_balances` | 平台在报表中给出的期末余额，按 `wallet + currency` 记录金额和日期，例如对账单的 Closing Balance |
| `total_rows`、`new_rows`、`duplicate_rows`、`pending_rows` | 结果计数 |
| `status` | `APPLIED` 或 `REVOKED` |
| `created_by`、`created_at`、`revoked_by`、`revoked_at`、`revoke_reason` | |

**`accounting_channel_lines` 渠道流水**

| 字段 | 说明 |
| --- | --- |
| `id`、`channel_account_id`、`import_id` | |
| 模板字段 | `wallet`、`external_id`、`occurred_at`、`type`、`amount NUMERIC(24,6)`、`currency`、`balance_after`、`fee_amount`、`card_ref`、`card_last4`、`card_name`、`merchant`、`related_id`、`original_amount`、`original_currency`、`remark` |
| `source_row` | 原始报表行 JSONB，用于追溯转换结果 |
| `pair_line_id` | 购汇或划转配对的另一端流水 |
| `channel_card_id` | 关联的渠道卡 |
| `transaction_id` | 关联的企业卡消费 `accounting_vcc_transactions.id` |
| `booking` | `INTERNAL`（仅渠道账）、`BOOKED`（已新建企业卡消费）、`LINKED`（关联到手工录入的消费）、`PENDING_CARD`（待绑定卡）、`PENDING_ORDER`（待归属订单） |
| `status` | `ACTIVE` 或 `REVOKED`；`(channel_account_id, wallet, currency, external_id)` 在 `ACTIVE` 范围内唯一 |

**`accounting_channel_attachments` 渠道凭证**

| 字段 | 说明 |
| --- | --- |
| `id`、`channel_account_id`、`wallet`、`currency`、`external_id` | 凭证挂在平台单号上，而不是某一次导入的流水行上，因此批次撤销后重新导入，凭证仍然保留 |
| `filename`、`content_type`、`size_bytes`、`sha256`、`object_key` | 仅允许 jpg、png、webp、pdf，类型按文件内容判断；单个不超过 20 MiB，同一单号最多 20 个 |
| `created_by`、`created_at`、`deleted_by`、`deleted_at` | 删除为软删除 |

现有 `accounting_attachments` 要求归属某个企业，渠道账不属于企业，所以单独建表；文件同样存放在账目私有存储的 `accounting/channel/proofs/` 下。

**`accounting_channel_authorizations` 授权明细**（第三期）

保存平台授权记录，包括拒付和零金额验卡，只用于统计，不入账。字段取授权号、卡、状态、金额、商户、拒绝原因和时间。

## 入账规则

### 渠道账

所有类型的流水都写入渠道流水。每个余额（账户 + `wallet` + 币种）的系统余额 = 该余额首笔流水的期初余额 + 有效流水金额合计。

| 类型 | 处理 |
| --- | --- |
| `FUNDING_IN`、`FUNDING_OUT` | 我方资金进出渠道。`fee_amount` 计入手续费成本 |
| `FX_OUT`、`FX_IN` | 按 `related_id` 配对，互填 `pair_line_id`；汇率 = 卖出金额 ÷ 买入金额，用于展示，不产生损益 |
| `TRANSFER_OUT`、`TRANSFER_IN` | 先按 `related_id` 配对；没有时按同一账户、同一币种、金额相反、日期相差不超过 1 天、时间最近的未配对流水配对 |
| `FEE` | 手续费成本 |
| `ADJUSTMENT` | 我方资金调整 |
| `CONSUME`、`REFUND` | 渠道账记录资金变动；第二期起同时计入企业账 |

以上流水的 `booking` 都是 `INTERNAL`，`CONSUME`、`REFUND` 在第二期按下文规则变更。手续费成本 = `FEE` 金额合计 + `fee_amount` 合计，按币种分别统计。

**头寸**：每个账户按 `wallet + currency` 列出余额，再按币种汇总账户总头寸。不同币种不相加；页面可按最近一次购汇汇率给出折算参考，并标明是参考值。

### 卡绑定

`CONSUME` 和 `REFUND` 需要先确定渠道卡属于哪个企业的哪张系统卡。依次尝试：

1. 渠道卡已绑定：直接使用。
2. **按卡匹配**：系统中 `provider` 为该平台、`last_four` 等于 `card_last4` 的有效卡恰好一张，绑定该卡及其企业。
3. **按名称匹配**：`card_name` 等于某个企业的名称或代码，且该企业恰好有一张该平台的有效卡，绑定该卡。
4. 以上都不满足：流水标记 `PENDING_CARD`。财务在渠道页为该卡选择企业和系统卡后，该卡的待归属流水自动补入企业账。

系统登记新卡仍要求完整卡号，导入不会自动创建系统卡。平台侧卡昵称为空时只能按卡号后四位或手工绑定，例如样例中的 `****0956`（导出时昵称为空，2026-10-29 已在平台补设为“银通源”）。

### 企业卡消费

已绑定卡的 `CONSUME`、`REFUND` 按以下顺序处理：

1. **关联手工记录**：同一系统卡上已有有效消费，其 `provider_transaction_id` 等于 `related_id`（无 `related_id` 时取 `external_id`），则直接关联，`booking` 为 `LINKED`。
2. **按金额日期关联**：同一系统卡上有未关联渠道流水、交易号为空、金额相同、日期相差不超过 1 天的已确认消费，且候选恰好一笔，则关联并标记“按金额日期匹配”，供人工复核。
3. **新建消费**：通过订单条目的现有逻辑创建卡消费，`provider_transaction_id` 取 `related_id`，没有时取 `external_id`；`merchant`、`occurred_at`、金额绝对值照录，`booking` 为 `BOOKED`。同币种收款自动出资、AI Invoice 自动分摊等订单规则保持不变。
4. `REFUND` 通过 `related_id` 找到原消费；找不到时标记 `PENDING_ORDER`，由财务处理。

新建消费挂入该卡所属企业最近一个未关闭的订单。企业没有未关闭订单时标记 `PENDING_ORDER`；财务为该企业新建或重新打开订单后，待归属流水自动补入。

### 撤销

导入批次可以整批撤销，撤销需要管理员填写原因：

- 只能撤销该渠道账户最近一次未撤销的批次，避免中间断档。
- 撤销时，该批次 `BOOKED` 的企业卡消费按订单条目删除规则作废；`LINKED` 只解除关联，手工记录保留；配对关系一并解除。
- 涉及已结账期的流水不能撤销。
- 批次与流水改为 `REVOKED` 后保留，可追溯；同一去重键可以重新导入。

## 核对与提示

| 检查 | 结果 |
| --- | --- |
| 文件内余额连续性 | 不连续则整批拒绝 |
| 购汇成对 | 无法成对则整批拒绝 |
| 平台期末余额 | 每个余额的系统余额与批次 `reported_balances` 比较，显示一致或差额 |
| 跨批次衔接 | 新批次首笔的期初余额与系统中前一笔流水的 `balance_after` 不一致时，导入成功但提示“流水断档”，显示断档区间 |
| 划转未配对 | 导入成功，列出未配对的划转 |
| 负余额 | 平台余额为负的区间单独提示，例如样例中卡余额 2026-10-10 至 2026-10-15 |
| 待归属 | `PENDING_CARD`、`PENDING_ORDER` 计入财务待办 |
| 按金额日期匹配 | 列出供复核 |
| 入金缺凭证 | 列出没有任何凭证的 `FUNDING_IN`，可一键筛选 |

## 界面

新增入口“渠道”，与订单、卡片、收款账户并列。页面都使用独立路由；卡绑定在渠道详情页的列表行内完成。

**渠道列表** `/admin/accounting/channels`：每个渠道账户一张卡片，列出各余额（如收付款 HKD、收付款 USD、卡余额 USD）的系统余额和平台对账单余额、差额，以及按币种汇总的总头寸、待处理事项和最近导入时间。

**渠道详情** `/admin/accounting/channels/:id`：

- 资金链路：入金 → 购汇 → 划转 → 卡消费，每段显示期间金额，样式参照资金架构图。
- 按月汇总：入金、购汇（含平均汇率）、划转、消费、手续费成本、调整，各币种分列。
- 按卡汇总：卡、绑定企业、消费笔数和金额；第三期加入拒付次数，以及拒付和验卡产生的手续费。
- 流水列表：可按余额、月份、类型、卡、`booking` 筛选，待归属行提供绑定入口，购汇和划转显示配对的另一端。
- 凭证：每行可以就地展开上传区，直接粘贴截图（⌘V / Ctrl+V），也可以拖入或选择文件；没有凭证的入金标记“缺凭证”，删除凭证需确认。
- 导入批次：结果计数、平台期末余额核对、原始文件下载、撤销。

**导入** `/admin/accounting/channels/import`：拖入一组文件，提交即入账，然后显示导入结果。页面列出本平台需要导出的报表清单，提供空白模板下载，以及“只转换不导入”，用于下载转换后的模板文件进行核对。

以 SUNRATE 样例为例，渠道列表显示：收付款 HKD 0.00、收付款 USD 2,172.87、卡余额 USD 1,026.50，均与平台一致；USD 总头寸 3,199.37；`****0956` 待绑定 1 笔。详情页 2026-08 至 2026-10 的汇总为：入金 HKD 34,608.63（手续费 HKD 34.64），购汇 HKD 34,608.63 → USD 4,403.62，划转到卡 USD 2,230.75，卡消费 USD 1,200.00，授权费 USD 4.25。

## 接口

沿用 `/api/v1/admin/accounting` 的认证、角色、幂等和审计规则。写操作要求 `Idempotency-Key`。

| 路径 | 方法 | 用途 |
| --- | --- | --- |
| `/channel-imports` | POST | multipart 上传一组文件，可指定 `converter`，校验通过即入账 |
| `/channel-imports/convert` | POST | 只转换，返回标准模板 CSV |
| `/channel-imports`、`/channel-imports/{id}` | GET | 批次列表和详情 |
| `/channel-imports/{id}/revoke` | POST | 撤销批次，仅管理员，需原因 |
| `/channel-accounts`、`/channel-accounts/{id}` | GET/PATCH | 渠道账户列表、详情，修改名称和备注 |
| `/channel-accounts/{id}/summary` | GET | 各余额核对、头寸、按月和按卡汇总 |
| `/channel-accounts/{id}/lines` | GET | 渠道流水，分页筛选 |
| `/channel-cards/{id}` | PATCH | 绑定或改绑企业和系统卡，补入待归属流水 |
| `/channel-template` | GET | 下载空白模板 |
| `/channel-lines/{id}/proofs` | POST | multipart 上传一个凭证文件，挂到该流水的平台单号上 |
| `/channel-proofs/{id}/download-url` | GET | 凭证的短时下载地址 |
| `/channel-proofs/{id}` | DELETE | 删除凭证 |

## 实现要点

- 后端新增 `backend/internal/accounting/channel` 包。转换器实现统一接口，按平台注册：

  ```go
  type Converter interface {
  	Name() string
  	Detect(files []SourceFile) bool
  	Convert(files []SourceFile) (Result, error) // 标准流水 + 平台期末余额
  }
  ```

- xlsx 用 `github.com/xuri/excelize/v2` 解析，CSV 用 `encoding/csv`，PDF 对账单用纯 Go 的文本提取库按行解析表格；新增依赖需通过 govulncheck。前端现有的 `xlsx@0.18.5` 有已知安全问题，不用于解析上传文件。
- 一次导入在一个事务内完成，并按渠道账户加锁，防止并发导入破坏余额校验。
- 金额沿用 `shopspring/decimal` 和币种精度规则。
- 原始文件存为附件，附件格式需要为该用途放开 `.xlsx`、`.csv`；`.pdf` 已允许。
- 企业卡消费复用订单条目的创建、作废逻辑，不另写一套资金分配。

## 分期

1. **渠道账**：除授权明细外的 5 张表、模板导入、SUNRATE 转换器（两份对账单 PDF 与 4 类 xlsx）、购汇与划转配对、平台期末余额核对、入金凭证、渠道列表和详情、批次撤销。此阶段 `CONSUME` 和 `REFUND` 只进渠道账，不进企业账。
2. **企业账**：卡绑定、企业卡消费的新建与关联、待归属待办。
3. **授权统计与扩展**：授权明细导入、拒付统计，以及其他平台转换器。

## 待确认

1. 收付款余额对账单能否导出 Excel 或 CSV。能的话转换器优先读表格；只有 PDF 时按文本解析，平台改版式需要同步调整。
2. SUNRATE 退款、提现、调账在报表中的类型名称，需要取得实际样例后补入转换映射。
3. 是否需要为已手工录入的历史消费补录交易号，以提高自动关联的准确率。

[返回账目管理文档](README.md)
