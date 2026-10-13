# 账目管理功能文档（后端交接）

## 1. 业务流程

当前没有在线支付，流程为：

1. 甲方对公转账。
2. 记录实际销售金额、含税口径、税点和实际到账金额，上传到账截图。
3. 关联现有企业记录 `organization_id` 和甲方联系人。
4. 为企业分配 VCC 卡。
5. 记录 VCC 消费、退款和手续费。
6. 记录合同、发票、AI 厂商账单凭证和我方支出。
7. 按企业、VCC 卡和时间范围对账。

支出记录必须关联现有企业记录，不能脱离企业单独记账。

## 2. 到账记录

字段：

- `organization_id`
- 付款企业名称
- 联系人姓名、手机号、邮箱、微信号
- 收款账户
- `sales_amount`：实际销售金额
- `tax_inclusive`：是否含税
- `tax_rate`：税率/税点，比例保存，例如 6% 保存为 `0.06`
- `tax_amount`：税额
- `net_sales_amount`：不含税销售额
- `receivable_amount`：应收含税金额
- `received_amount`：实际到账金额
- 币种
- 到账时间
- 银行流水号/转账单号
- 到账截图
- 备注

金额计算：

- 含税：`tax_amount = sales_amount × tax_rate / (1 + tax_rate)`，`net_sales_amount = sales_amount - tax_amount`。
- 未税：`tax_amount = sales_amount × tax_rate`，`receivable_amount = sales_amount + tax_amount`。
- `received_amount` 单独保存，不能用到账金额代替销售金额。
- 实际到账与应收金额不一致时，必须填写差额原因。

状态：`DRAFT`、`RECEIVED`、`VOID`。已确认记录不能直接修改金额、企业、到账时间和银行流水号，使用冲正或调整记录。

## 3. VCC 卡和消费

### 3.1 VCC 卡

每张卡必须绑定企业，字段包括：卡服务商、卡名称、卡号密文或服务商 token、卡号后四位、币种、额度/分配金额、生效和失效时间、状态、开卡凭证。

完整卡号、CVV、有效期只允许加密存储，普通接口只返回后四位。

### 3.2 VCC 分配

- 一笔到账记录默认分配一张 VCC 卡。
- 分配金额不能大于该到账记录的未分配金额。
- 允许追加分配，但总分配金额不能超过到账金额。
- 换卡时关闭原分配关系，新建关系，不能覆盖历史记录。

### 3.3 VCC 消费

字段包括：企业、VCC 卡、消费日期、金额、币种、商户、服务商交易号、消费类型、状态、原消费记录、截图/账单附件和备注。

消费类型：`CONSUME`、`REFUND`、`FEE`、`ADJUSTMENT`。退款必须关联原消费记录；同一卡的同一服务商交易号不能重复入账。

## 4. 合同、发票和 AI 厂商账单凭证

### 4.1 合同

合同必须关联 `organization_id`，记录合同名称、编号、甲方名称、合同金额、币种、是否含税、税率/税点、签署日期、生效/到期日期、状态和合同文件。

### 4.2 发票

发票记录包括：发票类型、代码、号码、开票日期、开票方和收票方名称/税号、不含税金额、税额、价税合计、币种、状态和发票文件。发票代码+号码不能重复。

### 4.3 AI 厂商账单凭证

AI 厂商费用只上传 Invoice，作为 AI 厂商账单凭证。

凭证记录包括：厂商名称、账单周期、凭证类型（固定为 `INVOICE`）、凭证号码（有则记录）、金额、币种、文件、关联 VCC 消费或我方支出。确认后只能作废后重新上传。

## 5. 我方支出

支出字段：`organization_id`、类别、金额、币种、支出日期、付款账户、供应商、付款状态、合同编号、AI 厂商 Invoice、发票、付款截图和备注。

状态：`DRAFT`、`PENDING`、`PAID`、`VOID`。一笔支出可以分多次付款，每次付款记录金额、日期、账户、交易号和付款凭证；付款总额不能超过支出金额，超额需要管理员权限和原因。

## 6. 甲方组织账号可见范围

组织账号使用现有企业账号认证。服务端从会话取得 `organization_id`，不接受客户端切换企业。组织端只读，不能调用 Admin 账目接口。

组织端使用独立 DTO 和查询，不返回完整 Admin DTO 后再由前端隐藏字段。

| 数据 | 可见 | 不可见 |
| --- | --- | --- |
| 到账 | 本企业销售金额、含税标记、税点、税额、到账金额、日期、状态、已共享截图 | 收款银行账号、内部备注、确认人 |
| VCC | 卡别名、后四位、分配金额、可用余额、消费日期、商户、金额、状态 | 完整卡号、CVV、有效期、服务商、卡成本和内部手续费 |
| 合同/发票 | `customer_visible=true` 的字段和文件 | 未共享文件、内部支出发票 |
| 我方支出 | 不展示 | 供应商、成本、付款和利润 |
| 汇总 | 本企业到账、分配、消费、剩余余额 | 跨企业数据、内部成本和净利润 |

合同、发票和附件增加 `customer_visible`、`shared_at`、`shared_by`。到账截图和 VCC 消费凭证单独控制是否对甲方可见，不能因为关联流水自动公开。

建议组织端接口：

- `GET /api/v1/organization/accounting/summary`
- `GET /api/v1/organization/accounting/transfers`
- `GET /api/v1/organization/accounting/vcc-card`
- `GET /api/v1/organization/accounting/vcc-card/transactions`
- `GET /api/v1/organization/accounting/contracts`
- `GET /api/v1/organization/accounting/invoices`
- `GET /api/v1/organization/accounting/attachments/{id}/download-url`

## 7. 对账口径

```text
企业可用余额 = 已确认到账 - 已分配 VCC
VCC 剩余金额 = 分配金额 - 已确认消费 + 已确认退款 - 已确认卡手续费
未分配金额 = 已确认到账 - 有效 VCC 分配金额
```

对账状态：`UNMATCHED`、`MATCHED`、`DIFFERENCE`、`EXCLUDED`。

支持：到账与银行流水/截图、VCC 消费与服务商账单、我方支出与付款记录/发票/付款截图的匹配。金额差异必须记录原因；排除必须记录原因和操作者。

## 8. 后端接口

基础路径：`/api/v1/admin/accounting`。沿用现有 Admin 认证、分页、错误响应和 `request_id`。

### 到账和 VCC

- `GET /transfers`
- `POST /transfers`
- `GET /transfers/{id}`
- `PATCH /transfers/{id}`（仅草稿）
- `POST /transfers/{id}/confirm`
- `POST /transfers/{id}/void`
- `GET /vcc-cards`
- `POST /vcc-cards`
- `PATCH /vcc-cards/{id}`
- `POST /transfers/{id}/allocations`
- `POST /vcc-cards/{id}/close`
- `GET /vcc-cards/{id}/transactions`
- `POST /vcc-cards/{id}/transactions`
- `POST /vcc-transactions/{id}/confirm`
- `POST /vcc-transactions/{id}/void`

### 合同、发票、支出

- `GET/POST /contracts`
- `GET/PATCH /contracts/{id}`
- `GET/POST /invoices`
- `GET/PATCH /invoices/{id}`
- `POST /invoices/{id}/verify`
- `GET/POST /expenses`
- `GET/PATCH /expenses/{id}`
- `POST /expenses/{id}/payments`
- `POST /expenses/{id}/void`

### 附件、对账和汇总

- `GET /organizations/{organization_id}/summary`
- `POST /attachments/upload-intents`
- `POST /attachments/{id}/complete`
- `GET /attachments/{id}/download-url`
- `DELETE /attachments/{id}`（软删除）
- `GET /reconciliation`
- `POST /reconciliation/match`
- `POST /reconciliation/exclude`
- `POST /reconciliation/adjustments`

所有创建、确认、分配、付款、匹配、调整和作废接口支持 `Idempotency-Key`。重复请求返回第一次结果，不能重复生成金额记录。

附件支持 `jpg`、`jpeg`、`png`、`webp`、`pdf`；保存原文件名、类型、大小、SHA-256、用途、关联对象、上传人和上传时间。下载使用短时效地址；已确认记录的附件只能软删除并保留审计记录。

到账确认、VCC 分配、VCC 消费、支出付款、发票核对、附件下载、对账匹配和导出都写入追加式审计日志，至少包含操作者、对象、动作、请求 ID、时间和变更摘要。

## 9. 权限和验收

权限至少包括：

- `accounting:read`
- `accounting:write`
- `accounting:confirm`
- `accounting:reconcile`
- `accounting:manage`
- `organization:accounting:read`

验收要求：

1. 到账记录能关联企业、联系人、销售金额、税点、到账金额和截图。
2. 到账能分配 VCC，分配金额不能超出未分配金额。
3. VCC 消费、退款、手续费能正确计算余额。
4. AI 厂商费用必须上传 Invoice。
5. 支出、合同、发票、AI 厂商凭证和截图都能按企业查询。
6. 组织账号只能看到本企业和已共享字段。
7. 重复银行流水号或 VCC 服务商交易号不会重复入账。
8. 已确认数据不能直接改金额，调整后保留原记录和审计日志。
