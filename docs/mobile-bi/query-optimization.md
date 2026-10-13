# 当前查询边界

用量、模型、成员直接查询 Desktop 历史密钥关联与 billing facts；汇总独立于完整分页，不从 Top 10 或已加载页反推。

报告列表与运维采用显式 metadata 投影，正文、冻结来源和 checkpoint 按需读取。运维不能选择正文、输入快照或中间输出；企业正文读取继续执行原业务权限。

旧 AnalysisContext 查询计划与历史 BI 事实优化已退役。当前优化、容量和生产性能以 [验收进度](acceptance-progress.md) 为准，未做容量验证时不沿用旧性能基线。
