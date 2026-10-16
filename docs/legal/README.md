# 部署与运营合规文档

本目录保存管理员确认流程使用的中英文承诺正文，面向 Sub2API 的部署者和运营者。

| 文档 | 内容 |
| --- | --- |
| [中文正文](admin-compliance.zh.md) | 部署与运营责任、非关联关系、合规承诺及电子确认 |
| [English](admin-compliance.en.md) | 对应的英文承诺正文 |

正文由 `frontend/src/views/public/LegalDocumentView.vue` 直接导入；路径、公开 URL、确认短语及版本由 `backend/internal/service/admin_compliance.go` 定义。修改正文时需核对该处版本和管理员重新确认流程，不能只调整展示文本。

正文存放在 `docs/legal/`，该路径同时用于前端导入和 Docker 构建。

[返回文档索引](../README.md)
