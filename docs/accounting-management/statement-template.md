# 标准账单模板

标准账单模板是渠道账单入账的唯一格式。各平台原始报表先由转换器转成本模板，再导入入账；暂无转换器的平台，可以按本模板手工整理后导入。入账规则见[渠道账单导入](channel-import.md)。

- 空白模板：[channel-statement-template.csv](samples/channel-statement-template.csv)
- SUNRATE 转换样例：[sunrate-72778936-standard.csv](samples/sunrate-72778936-standard.csv)，由账户 `72778936` 的两份收付款余额对账单、卡余额流水、授权明细、入金记录和购汇记录转换，覆盖 2026-08-31 至 2026-10-27 的 36 笔流水
- 资金链路与核对结果：[SUNRATE 资金架构](sunrate-funds-architecture.html)

## 文件格式

- CSV，UTF-8 编码；推荐带 BOM，Excel 可直接打开中文。也接受首个工作表为同样表头的 `.xlsx`。
- 第一行是列代码，列顺序不限。未知列名视为错误，避免拼写错误的列被静默忽略。
- 一行表示渠道账户某个余额中的一笔资金变动。拒付、零金额验卡等不动资金的授权不写入模板。
- 余额按 `platform + account_ref + wallet + currency` 分别计算和校验。一个文件可以包含多个账户、余额类型和币种。
- 单个文件不超过 20 MiB、5 万行。

## 字段

| 列代码 | 必填 | 说明 |
| --- | --- | --- |
| `platform` | 是 | 平台代码，大写字母、数字和下划线，例如 `SUNRATE` |
| `account_ref` | 是 | 平台侧账户号，例如 `72778936` |
| `wallet` | 是 | 账户下的余额类型，大写字母、数字和下划线。SUNRATE 为 `PAYMENT`（收付款余额）和 `CARD`（卡余额）；只有一种余额的平台填 `MAIN` |
| `external_id` | 是 | 平台流水号。在 `platform + account_ref + wallet + currency` 范围内唯一，是去重键 |
| `occurred_at` | 是 | 发生时间，ISO 8601 并带时区，例如 `2026-10-22 18:16:39+08:00`；日期与时间之间可用空格或 `T`。平台只给日期时填当天 `00:00:00` |
| `type` | 是 | 流水类型，见下表 |
| `amount` | 是 | 带符号金额，进入该余额为正、离开为负；不带千分位，小数位不超过币种精度 |
| `currency` | 是 | ISO 4217 三位币种代码，例如 `USD` |
| `balance_after` | 否 | 平台给出的本笔后余额。提供时逐行校验余额连续性；同一文件中同一余额要么全部提供，要么全部留空 |
| `fee_amount` | 否 | 平台从本笔中扣除的手续费，非负数，计入我方成本。`amount` 是扣费后的净额，毛额 = `amount` + `fee_amount` |
| `card_ref` | 消费、退款必填 | 平台侧卡 ID |
| `card_last4` | 否 | 卡号后四位，用于匹配系统中已登记的卡 |
| `card_name` | 否 | 平台侧卡昵称。一个企业一张卡，昵称为企业名称，用于首次自动绑定企业 |
| `merchant` | 否 | 商户名称 |
| `related_id` | 见配对规则 | 关联单号：消费填平台授权交易号，退款填原消费交易号，购汇和划转填配对单号 |
| `original_amount` | 否 | 外币交易的原币金额，正数 |
| `original_currency` | 与 `original_amount` 同时填写 | 外币交易的原币币种 |
| `remark` | 否 | 备注 |

## 流水类型

| `type` | 含义 | `amount` 符号 | 入账口径 |
| --- | --- | --- | --- |
| `FUNDING_IN` | 外部资金转入，例如入金 | 正 | 我方自有资金，不是客户收入 |
| `FUNDING_OUT` | 资金转出到外部，例如提现 | 负 | 我方自有资金 |
| `TRANSFER_OUT` / `TRANSFER_IN` | 同一账户不同余额之间划转，例如收付款余额 → 卡余额 | 负 / 正 | 内部划转，总头寸不变，必须成对 |
| `FX_OUT` / `FX_IN` | 购汇的卖出和买入两端 | 负 / 正 | 币种转换，不产生收入或成本，必须成对 |
| `CONSUME` | 卡消费结算 | 负 | 计入卡所属企业的卡消费 |
| `REFUND` | 卡消费退款 | 正 | 冲减卡所属企业的原消费 |
| `FEE` | 平台单独扣收的手续费 | 负 | 我方成本，不转嫁企业 |
| `ADJUSTMENT` | 平台调账、费用返还等 | 正或负 | 我方资金调整；`remark` 必填 |

## 配对规则

- **购汇**：`FX_OUT` 与 `FX_IN` 的 `related_id` 填同一个购汇交易号，两端币种不同。一个交易号恰好一对；在本批次和已导入流水中都找不到另一端时整批拒绝。
- **划转**：`TRANSFER_OUT` 与 `TRANSFER_IN` 的 `related_id` 互相填对方的 `external_id`。平台两端单号不同、转换时无法确定对应关系时，`related_id` 可以留空，系统按同一账户、同一币种、金额相反、日期相差不超过 1 天自动配对；仍无法配对的划转导入成功，但提示“划转未配对”。

## 校验规则

导入时逐行校验，任一行出错则整批拒绝，错误提示到行号和列名。

1. 必填列缺失、类型不在上表、金额格式或精度不符、时间无时区。
2. `amount` 符号与 `type` 不一致，或金额为 0；`fee_amount` 为负数。
3. 同一文件内去重键重复。
4. `CONSUME`、`REFUND` 缺少 `card_ref`；`ADJUSTMENT` 缺少 `remark`。
5. 提供 `balance_after` 时，同一余额按 `occurred_at` 和文件行序排序后，上一行余额加本行金额必须等于本行余额。
6. 购汇在本批次和已导入流水中都无法成对。

已导入过的去重键不算错误，按重复行跳过，因此重叠导出的报表可以直接重复上传。

## SUNRATE 转换对照

SUNRATE 账户 `72778936` 下有两类余额：收付款余额（Payment Balance，按币种分 HKD、USD）和卡余额（Card Balance，USD，多卡共用）。转换器按文件名识别报表：

| 报表 | 文件名 | 作用 | 生成的 `wallet` |
| --- | --- | --- | --- |
| 收付款余额对账单（每个币种一份） | `SUNRATE_Statement_Payment_Balance_*.pdf` | 收付款余额的资金流水和余额，**入账依据** | `PAYMENT` |
| 卡余额流水 | `SUNRATE_CARD_BALANCES_<账户号>_*.xlsx` | 卡余额的资金流水和余额，**入账依据** | `CARD` |
| 授权明细 | `SUNRATE_AUTHORISATIONS_<账户号>_*.xlsx` | 补充卡昵称、卡号后四位、商户和授权交易号 | — |
| 入金记录 | `SUNRATE_Record_Collection_*.xlsx` | 补充入金毛额、手续费、汇款方和精确时间 | — |
| 购汇记录 | `SUNRATE_Record_Currency_Conversion_*.xlsx` | 补充汇率和精确时间 | — |

**收付款余额对账单**（PDF 表头 `Account ID`、`Currency`；明细列 `Transaction ID`、`Amount`、`Balance`、`Date`、`Description`）：

| `Description` | `type` | 补充 |
| --- | --- | --- |
| `Collection-Credit` | `FUNDING_IN` | `amount` 取对账单净额；按单号从入金记录取 `手续费` 填 `fee_amount`，毛额和汇款方写入 `remark`。入金记录的提交日期与对账单日期相同时，`occurred_at` 取提交时间，否则取对账单日期 |
| `Sell` | `FX_OUT` | `related_id` 为交易号；时间和汇率取自购汇记录 |
| `Buy` | `FX_IN` | 同上 |
| `Transfer to Card Balance` | `TRANSFER_OUT` | 按同日、同金额匹配卡余额流水中的 `Transfer In`，互填 `related_id`，时间取卡余额流水的时间 |
| 其他 | — | 拒绝转换，取得样例后补充映射 |

**卡余额流水**：

| 模板列 | 来源 |
| --- | --- |
| `external_id` | `Serial No.` |
| `occurred_at` | `Transaction Time (GMT+8:00)`，补 `+08:00` |
| `type` | `Transaction Type`：`Transfer In` → `TRANSFER_IN`，`Settlement` → `CONSUME`，`Authorization Fee` → `FEE`；其他类型拒绝转换 |
| `amount`、`currency`、`balance_after` | `Transaction Amount`、`Currency`、`Balance` |
| `card_ref` | `Card ID` |
| `card_last4`、`card_name`、`merchant`、`related_id` | 授权明细中 `Card ID` 相同且 `Settlement Time` 等于本行时间的记录：`Card No.` 后四位、`Card Nickname`、`Merchant Name`、`Transaction ID` |
| `original_amount`、`original_currency` | 授权明细 `Transaction Currency` 与 `Billing Currency` 不同时，取 `Transaction Amount`、`Transaction Currency` |

`SUNRATE_FEES_*.xlsx` 不参与入账：授权费以卡余额流水中的按日汇总金额为准，费用明细留待授权统计使用。

[返回账目管理文档](README.md)
