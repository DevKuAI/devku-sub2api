# 标准账单模板

标准账单模板是渠道账单入账的唯一格式。各平台原始报表先由转换器转成本模板，再导入入账；暂无转换器的平台，可以按本模板手工整理后导入。入账规则见[渠道账单导入](channel-import.md)。

- 空白模板：[channel-statement-template.csv](samples/channel-statement-template.csv)
- SUNRATE 转换样例：[sunrate-72778936-standard.csv](samples/sunrate-72778936-standard.csv)，由 `CARD_BALANCES` 加 `AUTHORISATIONS` 报表转换，覆盖 2026-09-02 至 2026-10-27 的 23 笔流水

## 文件格式

- CSV，UTF-8 编码；推荐带 BOM，Excel 可直接打开中文。也接受首个工作表为同样表头的 `.xlsx`。
- 第一行是列代码，列顺序不限。未知列名视为错误，避免拼写错误的列被静默忽略。
- 一行表示渠道账户中的一笔资金变动。拒付、零金额验卡等不动资金的授权不写入模板。
- 一个文件可以包含多个渠道账户和币种，余额按 `platform + account_ref + currency` 分别校验。
- 单个文件不超过 20 MiB、5 万行。

## 字段

| 列代码 | 必填 | 说明 |
| --- | --- | --- |
| `platform` | 是 | 平台代码，大写字母、数字和下划线，例如 `SUNRATE` |
| `account_ref` | 是 | 平台侧账户号，例如 `72778936` |
| `external_id` | 是 | 平台流水号。同一渠道账户内唯一，是去重键 |
| `occurred_at` | 是 | 发生时间，ISO 8601 并带时区，例如 `2026-10-22 18:16:39+08:00`；日期与时间之间可用空格或 `T` |
| `type` | 是 | 流水类型，见下表 |
| `amount` | 是 | 带符号金额，进入账户为正、离开账户为负；不带千分位，小数位不超过币种精度 |
| `currency` | 是 | ISO 4217 三位币种代码，例如 `USD` |
| `balance_after` | 否 | 平台给出的本笔后余额。提供时逐行校验余额连续性；同一文件同一账户币种要么全部提供，要么全部留空 |
| `card_ref` | 消费、退款必填 | 平台侧卡 ID |
| `card_last4` | 否 | 卡号后四位，用于匹配系统中已登记的卡 |
| `card_name` | 否 | 平台侧卡昵称。一个企业一张卡，昵称为企业名称，用于首次自动绑定企业 |
| `merchant` | 否 | 商户名称 |
| `related_id` | 否 | 关联交易号：消费填平台授权交易号，退款填原消费的交易号 |
| `original_amount` | 否 | 外币交易的原币金额，正数 |
| `original_currency` | 与 `original_amount` 同时填写 | 外币交易的原币币种 |
| `remark` | 否 | 备注 |

## 流水类型

| `type` | 含义 | `amount` 符号 | 入账口径 |
| --- | --- | --- | --- |
| `FUNDING_IN` | 向渠道账户充值 | 正 | 我方资金池转入，不是客户收入 |
| `FUNDING_OUT` | 从渠道账户转出 | 负 | 我方资金池转出 |
| `CONSUME` | 卡消费结算 | 负 | 计入卡所属企业的卡消费 |
| `REFUND` | 卡消费退款 | 正 | 冲减卡所属企业的原消费 |
| `FEE` | 平台收取的手续费 | 负 | 我方成本，不转嫁企业 |
| `ADJUSTMENT` | 平台调账、费用返还等 | 正或负 | 我方资金池调整；`remark` 必填 |

## 校验规则

导入时逐行校验，任一行出错则整批拒绝，错误提示到行号和列名。

1. 必填列缺失、类型不在上表、金额格式或精度不符、时间无时区。
2. `amount` 符号与 `type` 不一致，或金额为 0。
3. 同一文件内 `external_id` 重复。
4. `CONSUME`、`REFUND` 缺少 `card_ref`。
5. 提供 `balance_after` 时，按 `occurred_at` 和文件行序排序后，上一行余额加本行金额必须等于本行余额。

已导入过的 `external_id` 不算错误，按重复行跳过，因此重叠导出的报表可以直接重复上传。

## SUNRATE 转换对照

转换器读取 `SUNRATE_CARD_BALANCES_<账户号>_<时间>.xlsx` 作为资金流水，`account_ref` 取自文件名中的账户号；同批上传的 `SUNRATE_AUTHORISATIONS_*.xlsx` 只用于补充卡、商户和授权交易号。`SUNRATE_FEES_*.xlsx` 不参与入账，手续费以余额流水中的按日汇总金额为准。

| 模板列 | SUNRATE 来源 |
| --- | --- |
| `external_id` | 余额流水 `Serial No.` |
| `occurred_at` | 余额流水 `Transaction Time (GMT+8:00)`，补 `+08:00` |
| `type` | 余额流水 `Transaction Type`：`Transfer In` → `FUNDING_IN`，`Settlement` → `CONSUME`，`Authorization Fee` → `FEE`；其他类型拒绝转换，待取得样例后补充映射 |
| `amount`、`currency`、`balance_after` | 余额流水 `Transaction Amount`、`Currency`、`Balance` |
| `card_ref` | 余额流水 `Card ID` |
| `card_last4`、`card_name`、`merchant`、`related_id` | 授权明细中 `Card ID` 相同且 `Settlement Time` 等于余额流水时间的记录：`Card No.` 后四位、`Card Nickname`、`Merchant Name`、`Transaction ID` |
| `original_amount`、`original_currency` | 授权明细 `Transaction Currency` 与 `Billing Currency` 不同时，取 `Transaction Amount`、`Transaction Currency` |

[返回账目管理文档](README.md)
