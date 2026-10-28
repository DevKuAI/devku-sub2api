# 账目管理文档

账目管理沿用 Desktop 企业与账户挂载关系，覆盖到账、VCC 分配和消费、合同、发票、AI Invoice、支出、对账，以及企业账户的只读查看。

| 文档 | 内容 |
| --- | --- |
| [业务术语](GLOSSARY.md) | 到账、应收、核销、成本分摊与月结等术语定义 |
| [企业账务闭环](workflows.md) | 订单页日常录入，以及保留在接口中的应收核销、换汇、成本分摊、服务商确认及月结 |
| [实现与配置](implementation.md) | 接口、数据规则、账户菜单及资源 bucket 配置 |
| [渠道账单导入](channel-import.md) | 设计方案（未实现）：渠道资金池、平台账单导入、卡消费自动入账与余额核对 |
| [标准账单模板](statement-template.md) | 渠道账单的统一入账格式、校验规则及 SUNRATE 转换对照 |
| [验证记录](validation.md) | 2026-10-12 与 2026-10-13 的本地测试、浏览器验收及验证边界 |

日常操作从订单开始，先阅读企业账务闭环的“订单：日常入口”，接口接入与存储配置以实现说明为准。账目附件复用 `DESKTOP_UPDATE_STORAGE_RESOURCE_BUCKET`，通过 `accounting/` 文件夹与其他资源隔离。

## 设计基线

[原始设计](../archive/accounting-management/design.md) 和 [HTML 原型](../archive/accounting-management/prototype.html) 已归档，保留 `origin/docs/accounting-management` 的 `4cab773bda883ad6306e26d30c497d032108497e` 原始内容。原型用于追溯设计，不是当前管理页面；后续企业工作区与账务闭环按上面的使用和实现文档阅读。

[返回文档索引](../README.md)
