# 当前接口设计

小程序只读业务数据；企业后台管理邀请和访问；平台 BI 运维只读业务 metadata，并维护身份状态和 BI 清理策略。

| 契约 | 路由 | 鉴权 |
| --- | --- | --- |
| [小程序](openapi.yaml) | `/api/bi/v1`，Desktop 读取位于企业 `/desktop` 下 | 独立 BI JWT |
| [平台运维](admin.openapi.json) | `/api/v1/admin/bi` | 原站 admin + mutation step-up |
| [企业后台扩展](desktop.openapi.yaml) | `/api/v1/desktop/organization/bi` 及企业周月总结 | 原站负责人归属校验；平台企业接口用 admin |
| [隐私说明](privacy-notice.openapi.yaml) | `/api/v1/settings/bi-privacy` | public read / 原站 admin write |

列表默认 `page_size=20`、最大 100；返回 `items/page/page_size/total/has_more`。路径参数为公开对象 ID，授权对象固定 `bi_user_id`，没有旧 `user_id/manager_id` 兼容。

移动响应直接返回 DTO；原站接口使用 `{code:0,message,data}` envelope。移动和 BI 管理错误提供 `code/message/request_id`；原 Desktop 错误保持原站规范。详情、来源和复制逐请求校验企业状态及功能权限，生产请求失败不使用演示兜底。

企业周月生成控制沿用原有平台管理员边界；企业负责人读取日／周／月内容、管理自己的使用者和邀请，不因此获得平台控制权限。小程序没有生成、重试或重生成接口。

旧 59 个 BI operation 已退役。部门、知识、评估、收藏、笔记和受控分享只保留页面设计，不保留空实现 API。当前路由与 OpenAPI 的一致性由后端路由测试核对。
