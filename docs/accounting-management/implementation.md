# 账目管理实现与验收说明

需求基线为 `origin/docs/accounting-management` 的 `4cab773bda883ad6306e26d30c497d032108497e`。同目录的 `design.md`、`prototype.html` 保留该提交的原始内容。

## 账户入口与权限

管理员和财务角色通过 `/admin/accounting` 进入企业账目列表，再进入 `/admin/accounting/organizations/{organization_id}` 处理单个企业的账目。企业沿用现有 `gateway_user_id` 绑定关系；对应账户显示「财务账目」，进入 `/organization/accounting`。没有账目的已挂载企业仍显示菜单和空状态。绑定企业的管理员在「我的账户」下查看本企业账目。

账户菜单和页面受 Desktop 功能开关控制。组织 API 每次通过 Web 登录身份重新查询企业归属，不采用客户端的 `organization_id`。普通未挂载账户和 Desktop 成员会话不能访问。登录、切换账户、页面重入、窗口聚焦刷新归属，退出后清除缓存。

现有 Admin 兼容 `accounting:read/write/confirm/reconcile/manage`；组织账户只有 `organization:accounting:read`。没有增加权限分配 UI。

## 按企业处理账目

首页展示企业目录、跨企业的按币种汇总和到账趋势，不再把各类业务记录放在企业之外的顶层入口。先选择企业，再在该企业下处理「收款与用卡」「合同与票据」「支出与核对」。企业名称和归属说明持续显示，新增记录自动携带当前企业，表单不再重复选择企业。

- 到账后：从到账详情上传凭证、确认到账，再分配至本企业 VCC；卡片、消费与分配历史均在企业内查看。
- 合同与票据：管理该企业的合同、发票及 AI Invoice，沿用现有确认、共享和关联规则。
- 支出与付款：先登记支出，再从支出详情记录付款；付款列表用于查看和后续处理已有付款。
- 对账与审计：当前企业的筛选、导出、对账、审计和关联选项均携带该企业范围。企业端仍沿用原有只读接口与共享边界。

表单保留原有字段、默认值、校验和保存接口，按金额与时间、交易主体、联系人、税费、关联记录、有效期、敏感信息及备注等业务内容分组。长表单在弹窗正文区域滚动，保存和取消按钮放在底部操作区；窄屏使用账目分类选择框，桌面使用分组导航。

企业工作区支持 `?tab=...` 定位和刷新恢复分类。切换企业会重建工作区，清除上一个企业的列表、表单与关联选项；未找到企业时显示返回列表的提示，不回退为跨企业录入。财务角色可进入上述两个账目页面，不因此获得其他管理页面权限。

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

## 数据与接口

迁移 `256_accounting_management.sql` 只增加账目表及索引，不转换或回填支付、Gateway 余额或 API 计费数据。金额、资金来源及销售/税务金额使用 `NUMERIC(24,6)`；税率使用比例。业务 metadata 使用经过字段白名单校验的 JSONB，销售、税务和发票金额具有自动同步的 numeric generated columns。

基础路径：

- Admin：`/api/v1/admin/accounting`
- 组织只读：`/api/v1/organization/accounting`

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
    "contact_name": "联系人",
    "receiving_account": "收款账户",
    "sales_amount": "58000.00",
    "tax_inclusive": true,
    "tax_rate": "0.06",
    "bank_reference": "银行流水号"
  }
}
```

到账的 `amount` 表示实际到账，销售金额位于 `details.sales_amount`；税额、不含税销售额和应收金额由服务端计算。到账不等于应收时，提交 `details.difference_reason`。

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
