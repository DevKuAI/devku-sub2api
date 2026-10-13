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
