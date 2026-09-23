# 与 devku-sub2api 的能力映射

核验日期：2026-09-21；本地仓库提交 `45294e278`。只读取源码，未连接生产、未读取生产配置、未实际调用接口。以下“已有”仅表示本地代码存在，不保证部署实例启用了路由或具有同样版本。以本轮用户要求为产品范围，不把仓库中的历史商业报告当作新增指令。

## 已核实能力

| 能力 / 已有路径 | 本地证据 | 本版处理与限制 |
| --- | --- | --- |
| `/api/v1/auth/login`、`/login/2fa`、`/refresh`、`/logout`、`/me` | [认证路由](../../../devku-sub2api/backend/internal/server/routes/auth.go)；[原客户端](../../src/api/client.ts) | 原站身份验证与绑定确认可复用；不把原登录凭证直接交小程序持久化 |
| `/api/v1/settings/public` | 同上 | 原站验证码/协议配置；新bootstrap仅返回小程序允许的公开字段 |
| `/api/v1/auth/oauth/wechat/start`、`/callback` 等 | [WeChat OAuth](../../../devku-sub2api/backend/internal/handler/auth_wechat_oauth.go) | 当前是网页/扫码OAuth，代码涉及oauth2/authorize、qrconnect；不能冒充wx.login/code2session。新增小程序身份桥接 |
| `GET /api/v1/desktop/organization` | [企业路由](../../../devku-sub2api/backend/internal/server/routes/user.go)；[管理handler](../../../devku-sub2api/backend/internal/handler/desktop_management_handler.go) | 使用管理账号的企业绑定；找不到企业返回204。接口受Desktop.Enabled特性门控。不能以204构造示例企业 |
| `GET /api/v1/desktop/organization/members` | 同上；[成员用量模型](../../../devku-sub2api/backend/internal/service/desktop_models.go) | page/page_size/search/status，返回姓名手机号状态及today/30d/total用量；手机DTO去手机号。只有滚动合计不足以支持周留存/周期人员活跃 |
| `GET /api/v1/usage/dashboard/trend`、`/models`、`/snapshot-v2` | [usage handler](../../../devku-sub2api/backend/internal/handler/usage_handler.go) | 可作用户范围用量来源与对账；不是团队/应用/场景BI。models用户接口仅requested model口径 |
| 使用明细、输入输出缓存桶、请求时间及API Key关联 | [UsageLog Schema](../../../devku-sub2api/backend/ent/schema/usage_log.go)；[SQL统计](../../../devku-sub2api/backend/internal/repository/usage_log_repo_dashboard.go) | 可建立服务端增量归一化；不要把API Key值或原请求正文同步给小程序 |
| OpenAI总输入拆成非缓存输入/缓存读/缓存写 | [归一化实现](../../../devku-sub2api/backend/internal/service/openai_gateway_usage.go) | 标准input需重新合并互斥桶；不能直接沿用演示字段的相同名字 |
| `/api/desktop/v1/auth/*`、`/me`、`/usage/summary` | [Desktop路由](../../../devku-sub2api/backend/internal/server/routes/desktop.go) | 员工客户端协议，不作为管理者认证或BI权限入口 |
| `/api/desktop/v1/resources` 与版本制品 | 同上；[资源契约](../../../devku-sub2api/docs/desktop-resources-v1.md) | 可评估作为部分应用/Skill目录来源；资源版本不等于知识版本，不自动视为业务案例 |

sub2api的group是模型/计费配置语义，不能未经组织映射就当作部门。管理企业接口由当前账号查绑定，不接受客户端随意查询企业；新的多企业BI角色需独立建设，不假定每个原管理账号天然管理多个企业。

## 差距与新增设计

| 新能力 | 现状判断 | 对应设计 |
| --- | --- | --- |
| 小程序身份、管理账号绑定、BI授权角色 | 未在已检查路由确认小程序code2session及部门授权 | auth与organizations、按能力授权；旧身份只作为可信桥接 |
| 部门/岗位历史、适用人群 | 当前Desktop成员DTO不足 | directory/membership/eligibility同步；统一主归属与历史有效期 |
| 人员主动/自动/unknown、应用及场景归因 | API Key、模型、user-agent不足以证明 | 可信工作入口事件＋网关用量关联；unknown保留，不猜测 |
| 任意周/月采用、频率、稳定与留存 | 30天总量不足以还原活跃日 | 事件历史、首用历史完整性、水位及聚合 |
| 知识、案例、来源ACL、引用版本、失败样本 | 未在管理接口确认完整链路 | 标准内容与引用对象；只读知识入口和个人反馈 |
| 质量版本比较 | 日志成功不代表业务质量通过 | 独立固定测试集、标准、版本与样本结果 |
| 报告快照、历史回看、授权分享 | 当前小程序仅本地拼文本 | 服务端不可变快照、任务状态、幂等、接收者权限 |
| 收藏和note持久化 | 演示只有内存 | 用户/企业隔离的自然键、ETag并发保护 |

“未确认”不是断言整个仓库完全没有相关能力。实施前按本设计对象核对负责模块，已有合适能力可适配，但不能因为有同名字段就绕过口径与权限验证。

## 归一化适配要求

1. 保持原接口对旧客户端的契约，BI新增DTO不改旧JSON字段含义。
2. 企业public_id、成员public_id、内部user/api_key映射存在服务端映射表。模型密钥轮换不生成新员工；映射必须保留有效时间段。共享密钥不能拆出个人行为时保留unknown。
3. 对账使用相同时间区间、时区、用户范围、模型来源、缓存编码。既有成员rolling30d不能作为自然月的验收基线。代码中存在不同token_count公式，必须选定对账路径并保留差异解释。
4. usage日志未必包含所有失败尝试；失败率须连接错误/执行结果事实，缺少结果覆盖时指标未知，不能拿成功日志总数当全部请求。
5. 上游数据晚到通过新data_revision修正在线查询，已生成报告不悄悄变化。组织调岗映射与密钥轮换必须有历史有效区间。
6. 不把客服对话的存在、模型生成的摘要或外部资源文件自动认定为已验证知识；知识状态由有权限的原维护入口提供。
