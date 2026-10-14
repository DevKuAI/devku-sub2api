# BI 优化实现与验证入口

本轮覆盖小程序、Sub2API 原生读取及 Web BI 管理。采用现有 Taro／React external store、Gin、SQL Repository 和 Desktop 报告路径，未恢复旧 BI Context 或事实复制链路。

| 优化 | 实现与验证入口 |
| --- | --- |
| 分页和权限状态 | 小程序 `nativeState.ts` / `nativeStore.ts` 与 native-state tests，H5 返回流程回读 |
| 分层限流与版本 | `internal/bi/rate_limit.go` / `client_version.go`，单元与真实路由联调 |
| 统一统计 cutoff | `as_of`、Repeatable Read、并发入库对账与 query profile |
| 企业内称呼 | migration 259、邀请 recipient_name、访问 organization_alias、revision 保护 |
| 邀请与扫码 | Web 手动分发二维码；小程序预览、主动确认、只读核实与独立恢复 |
| 总结阅读 | 最新可读周期、结构化 outline、安全 Markdown、校验的冻结日总结跳转 |
| 数据覆盖与首页 | overview 复用统计；按成员 ID 关联用量／上报，日期语义分开 |
| 契约与本地联调 | 四份当前 OpenAPI、synthetic response validator、实际小程序 TypeScript client |

## 联调命令

从 backend 目录执行，两个 checkout 及小程序依赖需已存在：

```sh
uv run --with jsonschema==4.25.1 --with PyYAML==6.0.3 -- env -u OPENAI_API_KEY BI_MOBILE_PROJECT=/path/to/devku_mobile/miniprogram BI_REQUIRE_MOBILE_HTTP=1 BI_REQUIRE_CONTRACT_VALIDATION=1 go test -tags=integration ./internal/bi -count=1
```

上面的 `uv run` 命令安装固定版本的契约验证依赖。合成响应排除包含会话凭证的认证响应；缺少小程序 checkout 时，严格联调模式明确失败。两端联调仅在本地手动执行，Sub2API 的 GitHub Actions 不拉取或测试小程序仓库。

查询 profile 使用 40 位成员、4 万条合成日志和 7/30/90 天范围，不设置依赖机器速度的通过阈值。当前本地成员查询 P95 从约 38/98/314 ms 改善到约 21/54/140 ms；这是单环境结果，不是生产 SLA。命令：

```sh
env -u OPENAI_API_KEY go test -tags=integration ./internal/repository -run '^TestDesktopNativeMemberQueryProfile$' -v -count=1
```

本地测试、H5、weapp/H5 构建、微信兑换与扫码真机、容量、生产接入和远端 CI 分开验收。独立隐私站点、只读权限、Token 字符串、不可观测值、归档版本与源数据保留仍为约束。

[返回 BI 索引](README.md)
