# 账目管理实现与验收说明

需求基线为 `origin/docs/accounting-management` 的 `4cab773bda883ad6306e26d30c497d032108497e`。[原始设计](../archive/accounting-management/design.md) 与 [HTML 原型](../archive/accounting-management/prototype.html) 已归档，保留该提交的原始内容。

## 账户入口与权限

管理员和财务角色通过 `/admin/accounting` 进入企业账目列表，再进入 `/admin/accounting/organizations/{organization_id}` 处理单个企业的账目。企业沿用现有 `gateway_user_id` 绑定关系；对应账户显示「财务账目」，进入 `/organization/accounting`。没有账目的已挂载企业仍显示菜单和空状态。绑定企业的管理员在「我的账户」下查看本企业账目。

账户菜单和页面受 Desktop 功能开关控制。组织 API 每次通过 Web 登录身份重新查询企业归属，不采用客户端的 `organization_id`。普通未挂载账户和 Desktop 成员会话不能访问。登录、切换账户、页面重入、窗口聚焦刷新归属，退出后清除缓存。

现有 Admin 兼容 `accounting:read/write/confirm/reconcile/manage`；组织账户只有 `organization:accounting:read`。没有增加权限分配 UI。

## 订单

一笔订单对应一次企业业务：收到企业付款 → 给企业开发票 → 用 VCC 卡为企业采买 AI 账号 → AI 厂商 Invoice 回来。订单是财务的主入口：

- **新建订单**时可同时登记第一笔收款，也可以先建订单、收款到账后再补。
- **订单页**集中展示四步进度（收款、开票、卡消费、AI Invoice）、按币种汇总的收入/已开票/卡消费/AI 成本，以及订单内的全部记录。每块都能直接登记新记录（保存后回到订单页），收款、发票和 AI Invoice 还可以"关联已有记录"。
- **自动完成**：有已到账收款、已核对销项发票、已确认卡消费，且所有卡消费都已被已确认的 AI Invoice 分摊覆盖时，订单显示"已完成"。缺项（如客户不要发票）可填原因手动结束，之后能重新打开。
- 订单内的发票只能是销项发票；进项发票、内部换汇资金和卡充值不能归入订单。一条记录最多属于一个订单。

订单内的自动处理：

- **卡消费资金**：确认订单内的卡消费后，如果卡上资金不够，会自动从本订单同币种的已到账收款中补足分配；这类分配是消费已发生后的记账，不再生成"服务商待确认"事项。收款晚于消费确认时，同样会在收款确认后补足。
- **AI 成本分摊**：订单内的 AI Invoice 确认后，自动按金额分摊到本订单同币种的已确认消费（先到先分），受原有成本上限约束；剩余部分仍在待办里。

### VCC 卡

- 卡片和卡交易（消费、退款、手续费、调整）只能在订单里登记，后端强制；退款自动归入原消费所在订单，即使该订单已结束。
- 新增卡必须填写完整卡号。系统保存卡号的带密钥指纹（密钥由 `TOTP_ENCRYPTION_KEY` 派生），同一张卡只能登记一次，在其他企业再次登记会被拒绝；指纹功能上线前登记的卡按"服务商 + 后四位"兜底比对。
- **卡充值**：收款和卡不同币种（如人民币收款、美元卡）时，卡消费不会有同币种客户资金覆盖，卡余额为负。在卡片或订单页登记"卡充值"（我方自有资金）后，卡余额 = 充值/分配 − 消费，可与服务商余额核对。卡充值不计入客户收入，不出现在到账列表，企业端不可见，不生成服务商待确认事项；暂不支持作废，纠错请登记"调整"类卡交易。

## 按企业处理账目

首页是企业目录：各币种的可用资金、VCC 余额汇总，以及每个企业的可用资金、VCC 余额、已到账和未对账数量；点击企业进入其账目。「收款账户」入口维护我方统一收款账户。

企业账目页分为六个分区，分区内用分段切换具体类型：

| 分区 | 内容 |
| --- | --- |
| 概览 | 各币种可用资金、VCC 余额、已到账、已消费；进行中的订单；财务待办（按类型筛选，点击直达记录） |
| 订单 | 全部订单及进度，按状态筛选，新建订单 |
| 收款 | 到账、应收、换汇 |
| 用卡 | VCC 卡片、卡交易 |
| 合同票据 | 合同、发票、AI Invoice、成本分摊 |
| 支出 | 支出、付款记录 |
| 核对结账 | 对账、服务商核对、月结、审计日志 |

`?tab=` 使用具体类型（如 `transfers`、`periods`），旧链接 `?tab=workbench` 对应概览。列表筛选（状态、币种、日期、搜索）修改后立即生效；点击行进入记录详情。

### 记录详情与操作

每条记录有独立详情页 `/admin/accounting/organizations/{organization_id}/{kind}/{id}`：页头显示名称、状态和主金额，下方依次为格式化字段（税率显示为百分比和计税方式，敏感字段、内部 ID 不显示）、资金分配、收款核销、关联记录、附件和处理记录。

页头只显示当前状态下可执行的操作，主操作突出显示，例如草稿「确认入账」、有可分配余额的到账「分配到卡」、待付款支出「登记付款」、有效卡片「登记卡交易」。

- 只需确认的操作（确认入账、停用卡片、标记合同到期）使用确认弹窗，不离开详情页。
- 需要填写信息的操作（作废、分配、付款、金额调整、对账匹配/排除、核销、成本分摊、换汇资金分配、服务商确认、月结与重新开账）进入独立页面，完成后返回来源页。
- 付款和服务商确认可以在操作页直接上传凭证，无需先回到详情页上传。

### 表单

新增和编辑统一使用 `/admin/accounting/organizations/{organization_id}/{kind}/new` 和 `.../{id}/edit`，企业由路由确定，表单不再选择企业。字段按类型定义（`features/accounting/schema.ts`），只显示与当前选择相关的字段：

- 到账：收款账户按币种自动选择（多个时必须选择）；计税方式与税率合并为一个控件，实时预览税额、应收和到账差额，有差额时才要求填写原因。
- 卡交易：卡片决定币种；「调整方向」仅在调整类交易出现，「原消费」仅在退款（必填）或入账调整时出现；勾选 AI 费用后才显示 AI Invoice。
- 支出：类别为 AI 厂商费用时才显示 AI Invoice。
- 合同、发票：业务时间分别取签署日期、开票日期；发票价税合计由不含税金额和税额自动计算。
- 换汇：汇率按两个金额自动计算，可手动修改。

从详情页发起的新增（如从卡片登记交易、从到账发起换汇）会带上来源记录，保存后返回来源页。未保存离开时会提示确认。财务角色可进入上述账目页面，不因此获得其他管理页面权限。

企业端 `/organization/accounting` 为只读视图：概览、到账、VCC 卡片、卡交易、合同、发票、对账单，记录详情以只读弹窗展示公开字段和已共享附件。

企业端只返回已确认的记录：到账为已到账，VCC 卡片为有效或已停用，卡交易为已确认，合同为已签署或已到期，发票为已核对。草稿和已作废记录不出现在列表、附件和对账单中，即使其附件已设为共享也不能下载。

## 配置

账目附件复用现有私有 `resource_bucket` 和 Desktop S3 连接配置，不单独配置 bucket。文件存放在独立的 `accounting/{organization_id}/{attachment_id}/` 文件夹：

```yaml
desktop_update_storage:
  endpoint: "https://your-s3-endpoint"
  region: "your-region"
  resource_bucket: "private-resources"
  access_key_id: "configured-outside-version-control"
  secret_access_key: "configured-outside-version-control"
  force_path_style: false
```

对应环境变量沿用 `DESKTOP_UPDATE_STORAGE_RESOURCE_BUCKET`，其他连接变量沿用 `DESKTOP_UPDATE_STORAGE_*`。账目文件与 MCP/Skill 文件通过不同文件夹隔离；下载仍使用鉴权后的短时效地址。`resource_bucket` 保持私有访问。未配置存储时可管理基础记录，上传和依赖凭证的确认返回 `ACCOUNTING_STORAGE_UNAVAILABLE`。

完整卡号、CVV、卡片有效期及服务商 token 使用 AES-256-GCM。录入敏感信息需要配置固定的 `TOTP_ENCRYPTION_KEY`；备份和恢复时保留该密钥。所有读取接口均不返回这些字段的明文或密文。卡片有效期只作为加密字段录入；生效/失效日期为独立业务字段。

新增业务闭环和账期规则见 [企业账务闭环](workflows.md)。

## 数据与接口

迁移 `256_accounting_management.sql` 只增加账目表及索引，不转换或回填支付、Gateway 余额或 API 计费数据。金额、资金来源及销售/税务金额使用 `NUMERIC(24,6)`；税率使用比例。业务 metadata 使用经过字段白名单校验的 JSONB，销售、税务和发票金额具有自动同步的 numeric generated columns。

基础路径：

- Admin：`/api/v1/admin/accounting`
- 组织只读：`/api/v1/organization/accounting`

我方收款账户通过 Admin 下的 `GET/POST /receiving-accounts` 和 `PATCH /receiving-accounts/{id}` 统一维护。账户没有 `organization_id`，客户企业只是到账记录的付款方。前端按到账币种筛选统一账户列表：只有一个匹配账户时自动选中，存在多个时必须由操作人明确选择。

业务列表沿用 `items/total/page/page_size/pages`。金额和税率是十进制字符串，业务时间为 RFC3339；metadata 日期使用 `YYYY-MM-DD`。通用记录具有 `id`、`organization_id`、`parent_id`、`status`、`currency`、`amount`、`occurred_at`、`details`、`customer_visible`、`version`。

到账创建示例：

```http
POST /api/v1/admin/accounting/transfers
Idempotency-Key: a-unique-key-for-this-operation
Content-Type: application/json
```

```json
{
  "organization_id": "org_existing_public_id",
  "currency": "CNY",
  "amount": "58000.00",
  "occurred_at": "2026-10-12T10:00:00+08:00",
  "details": {
    "payer_name": "付款企业名称",
    "contact_id": "organization-contact-uuid",
    "receiving_account_id": "our-receiving-account-uuid",
    "sales_amount": "58000.00",
    "tax_inclusive": true,
    "tax_rate": "0.06",
    "bank_reference": "银行流水号"
  }
}
```

到账的 `amount` 表示实际到账，销售金额位于 `details.sales_amount`；税额、不含税销售额和应收金额由服务端计算。到账不等于应收时，提交 `details.difference_reason`。服务端根据 `receiving_account_id` 校验账户存在且币种一致，并把开户名称、银行、支行和账号写入到账详情作为历史快照；客户端不能直接提交自由文本替代统一账户。

其他实体路径为 `vcc-cards`、`vcc-transactions`、`contracts`、`invoices`、`vendor-invoices`、`expenses`、`expense-payments`。卡交易的 `parent_id` 指向卡片；付款通过 `/expenses/{id}/payments` 创建。草稿 PATCH 携带最新 `version`，避免覆盖并发修改。

付款需要 `details.proof_attachment_id`，指向该支出的已完成附件。超额付款必须设置 `details.override=true` 并填写 `details.reason`。AI 费用通过 `ai_vendor=true` 或支出类别 `AI_VENDOR` 标识，确认消费或付款前必须关联已确认 AI Invoice。

辅助接口包括 `summary`、`trend`、`organizations`、`audit-logs`、`export?kind=...`、分配历史、共享设置及对账。发票类型为 `SALES/PURCHASE`；AI Invoice 凭证类型固定为 `INVOICE`。组织端仅返回已共享客户发票和合同，内部发票和 AI Invoice 不公开。

所有创建、修改、确认、分配、付款、共享、匹配、排除、调整、作废及附件操作都要求 `Idempotency-Key`。同一操作和 key 重放首次结果；请求内容不同返回冲突。资金变更、操作结果和追加审计在同一事务中提交。key 不自动过期，后续独立操作使用新 key。客户端失败重试保留 key，成功后清理操作 key。

## 资金与对账口径

不同币种分别统计，不自动换汇。分配、卡交易、付款及对账金额比较必须同币种；AI Invoice 可以保留原币种，只做凭证关联。

到账、分配和消费总额按筛选期间统计；企业可用余额、卡片余额是当前资金状态。趋势按 Asia/Shanghai 自然日统计。组织消费汇总不包含内部手续费明细，卡余额包含实际费用影响。

分配通过企业事务锁防止并发超额。消费和费用按分配时间、ID 使用 FIFO 资金来源；新增分配先覆盖该卡已记录的透支。真实账单允许负余额，并显示差异。

关闭卡片保留分配关系，只释放实际未消费余额。关闭后退款先抵消原消费未覆盖部分，剩余资金释放到原到账；其他费用仍形成缺口时只释放实际正余额。释放资金再次分配后，退款冲正必须先处理新的占用。

已确认记录禁止直接修改受保护字段。作废保留原记录、资金来源，追加冲正记录与审计；金额调整保存独立调整记录。有有效退款的原消费、有未释放分配的到账、已有付款的支出不能直接作废。

对账通过 `reconciliation/match`、`exclude`、`adjustments` 手工操作；参考金额不同必须填写原因，排除也必须填写原因。卡片纠错录入 `ADJUSTMENT` 交易，使用 `DEBIT/CREDIT`，credit 关联原交易。对账状态与入账状态分开，排除对账不改变金额。

## 附件流程

1. 创建业务记录。
2. `POST /attachments/upload-intents`，提交关联对象、用途、文件名、类型、字节大小和 SHA-256。
3. `PUT /attachments/{id}/content` 上传原始文件。
4. `POST /attachments/{id}/complete` 完成后才可用于确认或付款。

允许 JPG、JPEG、PNG、WebP、PDF，单文件不超过 20 MiB，intent 有效期 10 分钟。服务端校验扩展名、实际类型、大小和 SHA-256，再写入 S3。文件不可覆盖。

共享使用 PATCH `/{kind}/{id}/sharing` 或 `/attachments/{id}/sharing`，提交 `customer_visible`；文件与合同/发票共享开关独立，默认关闭。AI Invoice 确认后不能新增或替换凭证，需要作废重建。

下载接口签发 5 分钟有效的私有地址，签发前检查归属、对象可见性、软删除状态并写审计。删除仅软删除，保留文件和历史审计。

## 验证

验证结果记录在 `validation.md`。常规命令：

```sh
make -C backend test-unit
make -C backend test-integration
(cd backend && golangci-lint run ./... --timeout=30m)
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend exec vitest run --maxWorkers=4 --minWorkers=1
pnpm --dir frontend run build
make -C backend build
git diff --check
```

集成测试要求 Docker，必须确认 PostgreSQL、Redis 测试容器实际启动。S3 适配器使用隔离 HTTP 服务验证，账目集成测试不使用生产 bucket。

可选浏览器 fixture：`SUB2API_ACCOUNTING_BROWSER_QA=1 go test -tags=integration ./internal/repository -run '^TestAccountingBrowserFixture$' -timeout=30m -count=1 -v`，从 backend 目录运行。服务地址写入 `/tmp/sub2api-accounting-qa.json`，前端代理连接该地址。仅供隔离验收，测试身份不代表生产认证。验收结束创建 `/tmp/sub2api-accounting-qa-complete` 让测试清理自身数据库容器。
