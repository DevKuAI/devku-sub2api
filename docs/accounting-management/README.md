# 账目管理文档

账目管理沿用 Desktop 企业与账户挂载关系，覆盖到账、VCC 分配和消费、合同、发票、AI Invoice、支出、对账，以及企业账户的只读查看。

| 文档 | 内容 |
| --- | --- |
| [业务术语](GLOSSARY.md) | 到账、应收、核销、成本分摊与月结等术语定义 |
| [功能设计](design.md) | 业务流程、字段、权限与验收要求 |
| [设计原型](prototype.html) | 管理页面与企业详情原型 |
| [企业账务闭环](workflows.md) | 应收核销、换汇、成本分摊、服务商确认及月结 |
| [实现与配置](implementation.md) | 接口、数据规则、账户菜单及资源 bucket 配置 |
| [验证记录](validation.md) | 本地测试、浏览器验收与验证边界 |

设计与原型基于 `origin/docs/accounting-management` 的 `4cab773bda883ad6306e26d30c497d032108497e`。运行与配置以实现说明为准；账目附件复用 `DESKTOP_UPDATE_STORAGE_RESOURCE_BUCKET`，通过 `accounting/` 文件夹与其他资源隔离。

[返回文档索引](../README.md)
