# 配置与部署

BI 默认关闭。配置模板见 `deploy/config.example.yaml`、`.env.example` 和现有 Compose；沿用独立 AppID/AppSecret、JWT Secret、Identity Secret，不复用 web JWT 或 Desktop JWT。

| 检查项 | 入口／处理 |
| --- | --- |
| BI 服务开关 | 部署配置 `BI_ENABLED` |
| 微信 AppID 与兑换凭证 | 服务端部署配置，不放入小程序或 public settings |
| JWT／Identity Secret | 独立随机密钥；修改会使会话或身份摘要失效，应计划迁移 |
| 独立隐私说明 | 后台设置发布 `bi_privacy_notice`；`site_url` 不继承 `frontend_url` |
| Desktop 服务 | 部署 Desktop 开关与企业状态 |
| 上报与总结 | 企业配置：上报、日总结、分析模型、预算和保留策略 |
| 周月自动生成 | 企业开关默认关闭；依赖日总结开启 |
| 企业访问 | 创建邀请，由使用者主动领取 |

公开 settings／页面注入新增 `bi_enabled`，仅用于入口判断，不暴露秘密。服务开关、隐私发布、企业状态和数据准备分别呈现。

本轮新增 migration 257／258，不修改旧 migration checksum。257 清理未发布旧 BI；258 增加周月任务、执行和默认关闭的开关。迁移会永久删除明确列出的旧 BI 开发数据，不影响 Desktop、原站用户和计费。应用到非隔离库前仍需独立部署授权。

新增 migration 259 为邀请预设称呼和企业内称呼增加独立列，不修改历史 migration。称呼不是身份鉴权字段。

Sub2API 的 GitHub Actions 仅验证本仓库，不配置跨仓 BI CI。两端联调使用本地已有的小程序 checkout，命令见 [本地联调与验证](integration-runbook.md)。

## 官方小程序码接入

出码复用已有 BI AppID/AppSecret，不新增密钥配置。服务端须可访问 `api.weixin.qq.com`，并按微信要求配置出口 IP 白名单。migration 265 为邀请增加可空短凭证摘要和唯一索引，保留旧邀请；只对新建邀请返回 scene。正式版使用 `pages/invitation/index` 与 `check_path=true`，需先发布对应小程序页面。开发／体验联调可在出码请求指定 `develop`／`trial`，不会改变默认正式版。出码缓存为进程内微信 access token 缓存，普通模式刷新，不持久化图片或明文邀请凭证。接口及前端接入见 [扫码加入交接](wechat-scan-join-handoff.md)。
