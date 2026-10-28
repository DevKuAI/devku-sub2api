# 渠道账单导入

本文是设计方案，尚未实现。目标是把虚拟卡平台等渠道的账单直接导入账目，替代卡消费的逐条手工录入，并让渠道资金池的余额、消费和手续费一目了然。首个接入平台是 SUNRATE，后续平台按同一模板接入。

## 业务口径

2026-10-28 确认：

- 渠道账户是**我方资金池**。SUNRATE 账户 `72778936`（香港阜森科技有限公司）由多张卡共用余额，卡分给企业使用。
- **一个企业一张卡**，平台侧卡昵称即企业名称。
- 平台手续费，包括拒付和零金额验卡产生的授权费，**由我方承担，记为成本**，不转嫁企业。
- 导入后**直接入账**，不设预览确认步骤。
- 导入的卡消费挂入该企业**最近一个未关闭的订单**。

由此形成两层账：

| 层级 | 视角 | 内容 | 核对对象 |
| --- | --- | --- | --- |
| 渠道账 | 我方 | 渠道账户的充值、转出、卡消费、手续费、调整 | 平台余额 |
| 企业账 | 客户 | 企业卡消费、退款，沿用现有订单、卡和资金分配 | 企业收款 |

渠道充值是我方资金在银行与渠道之间的移动，不形成客户收入，也不创建到账记录。手续费只进入渠道账，不进入企业账。

## 整体流程

```
平台原始报表 ─→ 转换器 ─┐
                       ├─→ 标准账单模板 ─→ 校验 ─→ 渠道流水（去重）─→ 企业卡消费
人工整理的模板 ─────────┘
```

1. 财务在导入页一次上传一组文件。系统按文件名和表头识别平台；能识别的原始报表由对应转换器转成[标准账单模板](statement-template.md)，模板文件直接使用。
2. 按模板规则校验。格式或余额连续性有误时整批拒绝，不写入任何数据。
3. 校验通过即入账：新流水写入渠道账，已导入的 `external_id` 跳过。
4. 卡消费和退款按卡绑定关系计入企业账；无法确定企业或订单的流水照常进入渠道账，标记为待归属，进入待办。
5. 返回导入结果：新增、重复、待归属行数，以及余额核对结果。

新平台只需新增转换器，入账逻辑和表结构不变。没有转换器之前，可以先手工整理模板导入。

## 数据模型

新增 5 张表，不改现有记录表结构。渠道相关表不带 `organization_id`，因为渠道账属于我方。

**`accounting_channel_accounts` 渠道账户**

| 字段 | 说明 |
| --- | --- |
| `id`、`platform`、`account_ref` | `(platform, account_ref)` 唯一；首次导入未知账户时自动创建 |
| `name`、`remark` | 显示名称，SUNRATE 取余额报表 `Account Name` |
| `created_at`、`created_by` | |

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
| `total_rows`、`new_rows`、`duplicate_rows`、`pending_rows` | 结果计数 |
| `status` | `APPLIED` 或 `REVOKED` |
| `created_by`、`created_at`、`revoked_by`、`revoked_at`、`revoke_reason` | |

**`accounting_channel_lines` 渠道流水**

| 字段 | 说明 |
| --- | --- |
| `id`、`channel_account_id`、`import_id` | |
| 模板字段 | `external_id`、`occurred_at`、`type`、`amount NUMERIC(24,6)`、`currency`、`balance_after`、`card_ref`、`card_last4`、`card_name`、`merchant`、`related_id`、`original_amount`、`original_currency`、`remark` |
| `source_row` | 原始报表行 JSONB，用于追溯转换结果 |
| `channel_card_id` | 关联的渠道卡 |
| `transaction_id` | 关联的企业卡消费 `accounting_vcc_transactions.id` |
| `booking` | `INTERNAL`（仅渠道账）、`BOOKED`（已新建企业卡消费）、`LINKED`（关联到手工录入的消费）、`PENDING_CARD`（待绑定卡）、`PENDING_ORDER`（待归属订单） |
| `status` | `ACTIVE` 或 `REVOKED`；`(channel_account_id, external_id)` 在 `ACTIVE` 范围内唯一 |

**`accounting_channel_authorizations` 授权明细**（第三期）

保存平台授权记录，包括拒付和零金额验卡，只用于统计，不入账。字段取授权号、卡、状态、金额、商户、拒绝原因和时间。

## 入账规则

### 渠道账

所有类型的流水都写入渠道流水。渠道账户余额 = 首笔流水的期初余额 + 有效流水金额合计，按币种分别计算。

`FUNDING_IN`、`FUNDING_OUT`、`FEE`、`ADJUSTMENT` 只进入渠道账，`booking` 为 `INTERNAL`。手续费汇总即渠道成本。

### 卡绑定

`CONSUME` 和 `REFUND` 需要先确定渠道卡属于哪个企业的哪张系统卡。依次尝试：

1. 渠道卡已绑定：直接使用。
2. **按卡匹配**：系统中 `provider` 为该平台、`last_four` 等于 `card_last4` 的有效卡恰好一张，绑定该卡及其企业。
3. **按名称匹配**：`card_name` 等于某个企业的名称或代码，且该企业恰好有一张该平台的有效卡，绑定该卡。
4. 以上都不满足：流水标记 `PENDING_CARD`。财务在渠道页为该卡选择企业和系统卡后，该卡的待归属流水自动补入企业账。

系统登记新卡仍要求完整卡号，导入不会自动创建系统卡。平台侧卡昵称为空时只能按卡号后四位或手工绑定，例如样例中的 `****0956`。

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
- 撤销时，该批次 `BOOKED` 的企业卡消费按订单条目删除规则作废；`LINKED` 只解除关联，手工记录保留。
- 涉及已结账期的流水不能撤销。
- 批次与流水改为 `REVOKED` 后保留，可追溯；同一 `external_id` 可以重新导入。

## 核对与提示

| 检查 | 结果 |
| --- | --- |
| 文件内余额连续性 | 不连续则整批拒绝 |
| 跨批次衔接 | 新批次首笔的期初余额与系统中前一笔流水的 `balance_after` 不一致时，导入成功但提示“流水断档”，显示断档区间 |
| 平台余额与系统账面 | 最新 `balance_after` 与系统计算余额比较，显示差额 |
| 负余额 | 平台余额为负的区间单独提示，例如样例中 2026-10-10 至 2026-10-15 |
| 待归属 | `PENDING_CARD`、`PENDING_ORDER` 计入财务待办 |
| 按金额日期匹配 | 列出供复核 |

## 界面

新增入口“渠道”，与订单、卡片、收款账户并列。页面都使用独立路由；卡绑定在渠道详情页的列表行内完成。

**渠道列表** `/admin/accounting/channels`：每个渠道账户一张卡片，显示平台余额、系统账面、差额、待归属笔数和最近导入时间。

**渠道详情** `/admin/accounting/channels/:id`：

- 按月汇总：充值、转出、消费、手续费、调整，各币种分列。
- 按卡汇总：卡、绑定企业、消费笔数和金额；第三期加入拒付次数，以及拒付和验卡产生的手续费。
- 流水列表：可按月份、类型、卡、`booking` 筛选，待归属行提供绑定入口。
- 导入批次：结果计数、原始文件下载、撤销。

**导入** `/admin/accounting/channels/import`：拖入一个或多个文件，提交即入账，然后显示导入结果。页面提供空白模板下载，以及“只转换不导入”，用于下载转换后的模板文件进行核对。

以 SUNRATE 样例为例，渠道列表会显示：平台余额 USD 1026.50，系统账面 USD 1026.50，差额 0，`****0956` 待绑定 1 笔。详情页 9 月至 10 月汇总为：充值 2230.75，消费 1200.00，手续费 4.25。

## 接口

沿用 `/api/v1/admin/accounting` 的认证、角色、幂等和审计规则。写操作要求 `Idempotency-Key`。

| 路径 | 方法 | 用途 |
| --- | --- | --- |
| `/channel-imports` | POST | multipart 上传一组文件，可指定 `converter`，校验通过即入账 |
| `/channel-imports/convert` | POST | 只转换，返回标准模板 CSV |
| `/channel-imports`、`/channel-imports/{id}` | GET | 批次列表和详情 |
| `/channel-imports/{id}/revoke` | POST | 撤销批次，仅管理员，需原因 |
| `/channel-accounts`、`/channel-accounts/{id}` | GET/PATCH | 渠道账户列表、详情，修改名称和备注 |
| `/channel-accounts/{id}/summary` | GET | 余额核对、按月和按卡汇总 |
| `/channel-accounts/{id}/lines` | GET | 渠道流水，分页筛选 |
| `/channel-cards/{id}` | PATCH | 绑定或改绑企业和系统卡，补入待归属流水 |
| `/channel-template` | GET | 下载空白模板 |

## 实现要点

- 后端新增 `backend/internal/accounting/channel` 包。转换器实现统一接口，按平台注册：

  ```go
  type Converter interface {
  	Name() string
  	Detect(files []SourceFile) bool
  	Convert(files []SourceFile) ([]Line, error)
  }
  ```

- xlsx 在后端用 excelize 解析，CSV 用 `encoding/csv`。前端现有的 `xlsx@0.18.5` 有已知安全问题，不用于解析上传文件。
- 一次导入在一个事务内完成，并按渠道账户加锁，防止并发导入破坏余额校验。
- 金额沿用 `shopspring/decimal` 和币种精度规则。
- 原始文件存为附件，附件格式需要为该用途放开 `.xlsx` 和 `.csv`。
- 企业卡消费复用订单条目的创建、作废逻辑，不另写一套资金分配。

## 分期

1. **渠道账**：5 张表中的前 4 张、模板导入、SUNRATE 转换器、渠道列表和详情、余额核对、批次撤销。此阶段 `CONSUME` 和 `REFUND` 只进渠道账，不进企业账。
2. **企业账**：卡绑定、企业卡消费的新建与关联、待归属待办。
3. **授权统计与扩展**：授权明细导入、拒付统计，以及其他平台转换器。

## 待确认

1. SUNRATE 退款、转出、调账在余额报表中的 `Transaction Type` 名称，需要取得实际样例后补入转换映射。
2. 是否需要为已手工录入的历史消费补录交易号，以提高自动关联的准确率。
3. 渠道账户出现多币种时，各币种是否需要单独的渠道账户名称或核对规则。

[返回账目管理文档](README.md)
