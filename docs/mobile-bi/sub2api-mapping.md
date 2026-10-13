# 路由与服务映射

| 功能 | Handler | 服务／查询 |
| --- | --- | --- |
| 微信登录、轮换、退出 | `internal/bi/http.go` | `auth.go/security.go/wechat.go`，独立身份与会话 |
| 邀请与企业授权 | `management_http.go`、邀请 HTTP | `access.go`，事务锁、revision、原站操作者审计 |
| Desktop 原生读取 | `desktop_http.go/period_http.go` | `DesktopService`、原有 conversation/report repository |
| 精确用量、成员和模型 | Native Desktop reads | `desktop_native_analytics.go`，直接查询 billing facts |
| 周月总结 | Desktop period handlers | `desktop_period_service.go/desktop_period_repo.go` |
| 运维 metadata | `operations_tasks.go` | 显式安全列投影；不选来源或正文 |
| BI 日常清理 | `cleanup.go` | 身份临时状态及相应审计 |
| Desktop 保留 | 现有 report service | `desktop_retention.go`，报告／执行策略保护活动任务 |

移动根路由、平台管理和企业负责人管理在 `backend/internal/server/routes/bi.go`。周月 web 读取和平台生成路由在 `user.go/admin.go`。公开 settings 与页面注入提供 `bi_enabled`，独立隐私地址继续由 `bi_privacy_notice.site_url` 决定。

JWT subject、授权对象和前端 DTO 均使用 BIUser 公开 ID；Desktop 员工 ID 只用于获权的企业数据详情，不能作为小程序登录凭证。
