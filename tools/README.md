# 仓库维护工具

本目录保存从仓库根目录运行的检查工具。部署回归测试位于 [deploy/tests/](../deploy/tests/)，发布 workflow 的专用工具位于 [.github/release-tools/](../.github/release-tools/README.md)。

| 工具 | 用途 | Python 依赖 |
| --- | --- | --- |
| `check_bi_contracts.py` | 检查 BI 契约、本地 `$ref` 和可选的合成 HTTP 响应 | PyYAML、jsonschema |
| `check_pnpm_audit_exceptions.py` | 按 advisory、包名、严重级别和到期时间检查 pnpm audit 例外 | 标准库 |

```bash
python3 tools/check_bi_contracts.py
python3 tools/check_pnpm_audit_exceptions.py \
  --audit /tmp/sub2api-pnpm-audit.json \
  --exceptions .github/audit-exceptions.yml
```

第二条命令要求先生成本次运行的审计报告。生成方式见 [开发指南](../docs/development/DEV_GUIDE.md#ci-与本地验证)，不能使用历史快照代替当前扫描结果。

[仓库目录说明](../docs/development/REPOSITORY_LAYOUT.md)
