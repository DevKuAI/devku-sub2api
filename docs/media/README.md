# 图片与视频任务文档

按 Provider 和任务类型选择入口。异步单次图片、Gemini 批量图片与 Seedance 视频使用不同的任务接口和存储路径。

| 文档 | 用途与边界 |
| --- | --- |
| [异步图片任务](ASYNC_IMAGE_TASKS.md) | OpenAI / Grok 长耗时图片任务、对象存储、提交与轮询 |
| [批量图片 MVP](BATCH_IMAGE_MVP.md) | Gemini / Vertex 图片批处理、Redis worker、计费与清理；未来优化单独标注 |
| [Seedance 原生 API](seedance-api.md) | OpenAI API Key 账号上的 Ark 视频创建、查询与删除，以及查询结算边界 |

[返回文档索引](../README.md)
