# 实现说明

当前实现采用 [独立身份协议](protocol.md) 与 [Desktop 原生数据映射](sub2api-mapping.md)。原独立 BI 导入、事实复制、Context、统计、内容、收藏笔记、报告分享、Connector CLI 和 Worker 已退役。

小程序保留 React external store，演示 Workspace 与生产 DTO 分离。生产使用内存会话、企业授权、模块缓存及请求代次；详情和分页按需读取。不新增通用业务框架。

周月总结复用现有 Desktop 总结执行路径，详见 [周月总结](period-reports.md)。平台管理员控制生成，企业负责人和小程序按当前业务权限阅读。

状态以 [验收进度](acceptance-progress.md) 为准，不把已实现代码标为设备或生产验收通过。
