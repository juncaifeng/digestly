---
name: digestly-creation-run
description: 触发 digestly 创作单元生成文章(基于最新未读素材 + 大纲模板 + LLM)。当用户说"开始创作""生成一篇日报/周报""跑一下某创作"时使用。前提是 digestly 桌面端正在运行(本地 API 在 http://127.0.0.1:3210)。
---

# digestly 开始创作

## 步骤

1. **找创作**:`GET http://127.0.0.1:3210/api/creations`,按名称匹配出 id(名称来自用户口述,模糊匹配后确认)
2. **生成**:`POST /api/creations/{id}/generate`,body `{}`(可省 template_id 则用创作默认模板)

   成功返回 201 + 文章 JSON:`{id, creation_id, title, content, item_ids, ...}`。把标题和正文要点汇报给用户。

## 常见错误处理

- **422 "素材源里没有未读文章"**:未读素材已耗尽。告知用户并询问是否强制生成;确认后重发 `{"force": true}`(取最新素材,不限未读)。
- **422 "该创作未关联大纲模板"**:先用 digestly-creation-add 配模板,或在 body 里临时指定 `{"template_id": N}`。
- **422 "还没有默认 LLM 配置"**:让用户去 digestly 设置页配大模型。
- 生成会消耗素材(标记已读),是真实副作用;生成耗时十几秒属正常。

## 相关

- 历史文章:`GET /api/articles?creation_id={id}`
- 生成前想先看素材 → digestly-creation-plan
