# 两端联调说明

以 [当前接口设计](desktop-api-design.md)、三份 OpenAPI 和 [本地联调](integration-runbook.md) 为准。旧账号绑定、Connector、Context 及 59-operation v1 契约不再用于新客户端。

接口授权对象是 `bi_user_id`；邀请码在 JSON body 传输；分页使用 page/page_size；Token 是 nullable decimal string。小程序只读，生产失败不回退演示。无权限、未启用和未接入必须分别说明。

未实施的知识、评估、采用和受控分享保留设计，不补空 API。完整验证与限制见 [验收](acceptance.md)。

微信扫一扫加入企业的后端已增加 scene 凭证与官方出码接口，Web 已接入官方图片、失败重试和手动邀请码备用方式；小程序继续接入扫码落地页。请求响应、失败恢复、旧邀请码兼容及验收清单见 [完整修改交接](wechat-scan-join-handoff.md)。
