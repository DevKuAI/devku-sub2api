# 当前接口设计

小程序只读业务数据；企业后台管理邀请和访问；平台「服务运维」统一查看企业服务、总结任务、模型执行与用量，仅返回业务 metadata；小程序身份、访问状态和清理策略单独管理。

| 契约 | 路由 | 鉴权 |
| --- | --- | --- |
| [小程序](openapi.yaml) | `/api/bi/v1`，Desktop 读取位于企业 `/desktop` 下 | 独立 BI JWT |
| [平台运维](admin.openapi.json) | `/api/v1/admin/desktop/operations` | 原站 admin + mutation step-up |
| [企业后台扩展](desktop.openapi.yaml) | `/api/v1/desktop/organization/bi` 及企业周月总结 | 原站负责人归属校验；平台企业接口用 admin |
| [隐私说明](privacy-notice.openapi.yaml) | `/api/v1/settings/bi-privacy` | public read / 原站 admin write |

列表默认 `page_size=20`、最大 100；返回 `items/page/page_size/total/has_more`。路径参数为公开对象 ID，授权对象固定 `bi_user_id`，没有旧 `user_id/manager_id` 兼容。

移动响应直接返回 DTO；原站接口使用 `{code:0,message,data}` envelope。移动和 BI 管理错误提供 `code/message/request_id`；原 Desktop 错误保持原站规范。详情、来源和复制逐请求校验企业状态及功能权限，生产请求失败不使用演示兜底。

企业周月生成控制沿用原有平台管理员边界；企业负责人读取日／周／月内容、管理自己的使用者和邀请，不因此获得平台控制权限。小程序没有生成、重试或重生成接口。

旧 59 个 BI operation 已退役。部门、知识、评估、收藏、笔记和受控分享只保留页面设计，不保留空实现 API。当前路由与 OpenAPI 的一致性由后端路由测试核对。

## 服务运维入口与开关

管理页面为 `/admin/desktop/operations`，位于「企业 AI 服务 → 服务运维」，支持 `?tab=overview|onboarding|tasks|identities|audit` 定位分区。旧页面 `/admin/bi` 和旧管理 API `/api/v1/admin/bi/*` 已移除，不提供跳转或别名；小程序 `/api/bi/v1` 和企业负责人接口保持原有协议。

- 运行概览：展示部署启用状态、未删除企业与成员数量、活动总结任务，以及企业、工具资源和客户端更新入口。启用状态不等于实时健康检查。
- 企业服务：查看企业功能开关、分析模型、企业成员数与最近上报时间。企业成员和小程序使用者分别统计。
- 总结与用量：查看日／周／月任务和执行元数据；按当前周期类型汇总全部保留记录，包含历史版本和重试。未知 Token 或费用保留未知，正文与来源仍在企业详情查看。
- 小程序访问：管理独立微信身份、企业邀请和访问授权；展示小程序接入配置状态和隐私说明状态。
- 小程序审计：管理小程序访问审计及临时访问数据保留，不代表平台全部操作审计或企业业务数据保留。

公共 `overview`、`organizations`、`report-tasks`（含单任务与执行列表）、`report-executions` 和 `report-statistics` 接口不受 BI 开关限制，均要求平台 admin。小程序邀请、身份、会话、审计、保留及清理接口仅在 BI 启用时注册；敏感写操作仍要求 step-up。停用小程序不影响企业报告运维。修改部署配置后需按既有部署流程重启服务。
