# 独立身份与企业邀请协议

## 身份

服务端用微信兑换得到的 `(appid, openid)` 识别 `BIUser`。数据库仅保存带独立密钥的 openid 摘要，不信任客户端传入 openid。JWT `sub` 为 `bi_user_id`，audience 为 `bi-miniprogram`；与原站账号和 Desktop 员工身份没有关联。

微信登录不会创建原站账号，也不会授予后台权限。原站企业负责人使用既有 web JWT、企业归属校验和 step-up 管理自己的企业；平台管理员可管理所有企业。

## 邀请与授权

管理员预设整企业功能权限，创建随机邀请码。默认有效 24 小时，单人一次性；服务端仅存凭证摘要，创建时返回邀请码和小程序路径。管理员自行发送，系统不发送消息。

使用者微信登录 → `POST /invitations/preview` 核对企业与权限 → 主动确认 → `POST /invitations/accept`，事务成功后立即开通，无二次审批。凭证放 JSON body，不进入 API URL。接口均需小程序 Bearer。

领取事务检查身份、企业、邀请有效期及撤销状态，并确认创建者仍是有效平台管理员或本企业负责人。并发领取仅一人成功；同领取者重放返回当前授权。重复加入不扩权；被企业撤销者不能通过新邀请自行恢复，须管理员修改访问状态。

授权按 `BIUser × 企业` 保存，权限和 revision 相互独立。暂不支持团队级范围。修改权限要求 `expected_revision`；冲突返回 409，不覆盖最新授权。

| 功能权限 | 实际开放内容 | 依赖 |
| --- | --- | --- |
| `analytics:read` | 企业用量、模型、上报汇总 | 无 |
| `members:read` | 成员目录与状态 | 无 |
| `conversations:read` | 会话问题、响应正文 | `members:read` |
| `reports:read` | 企业和成员日报、周月总结、版本及来源 | `members:read` |
| `reports:copy` | 经当前权限校验的总结复制 | `members:read`、`reports:read` |

企业撤权只影响本企业。平台禁用身份会递增 auth_version 并撤销全部会话；重新启用身份也不恢复旧会话。审计区分 `web_user`、`bi_user`、`system`。

## 会话与失败

Access Token 15 分钟，Refresh Token 12 小时；refresh 轮换且客户端单飞。两者仅存内存，不写 Storage、URL、日志。主动退出立即清空企业缓存与在途请求代次，只保存非敏感退出标记；再次点击登录才恢复。启动会删除旧持久化 refresh key。

切企业、改周期、撤权或退出增加请求代次；迟到响应不写回。401 可轮换一次，403 不通过 refresh 扩权；404 不泄露对象存在与否。读取失败显示独立重试，不回退演示。Web mutation 继续执行 step-up；423 提示先完成验证。未知创建结果应先刷新列表确认，不自动重发。

## 兼容、邀请称呼与限流

最低兼容版本默认 `0.2.0`，由 `bi.min_client_version` 配置；低于下限返回 `426 CLIENT_UPGRADE_REQUIRED` 和 `min_client_version`，不兑换或消费微信 code。客户端也在请求 code 前核对 bootstrap。兼容版本与应用发行版本分开。

`recipient_name` 是管理员预设的企业内称呼；首次领取保存为本企业的 `organization_alias`，不改变全局 BIUser。重复领取与已有访问不覆盖称呼。后续称呼更新与权限一起使用 `expected_revision`。

限流按 IP、操作和已登录身份分层：IP 外层 600 次／分钟，未登录操作每 IP 120 次／分钟，已登录邀请操作每 IP 300 次／分钟、每 BIUser 30 次／分钟；不同操作分别计数。429 带 Retry-After，客户端等待期间不重复发送同一路径。

领取超时先只读核对邀请是否已由当前身份使用，不自动重发 mutation。已领取但企业内容读取失败单独恢复；退出或切会话后不展示旧操作反馈。
