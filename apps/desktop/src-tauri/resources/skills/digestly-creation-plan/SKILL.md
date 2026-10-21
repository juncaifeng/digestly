---
name: digestly-creation-plan
description: 预览 digestly 创作的生成输入(大纲模板 + 素材清单 + 完整 prompt),不调用 LLM、无副作用。当用户说"预览一下创作""看看这次会用什么素材""把素材/prompt 给我"时使用;也适合自己作为外部 agent 取用素材再自行生成。前提是 digestly 桌面端正在运行(本地 API 在 http://127.0.0.1:3210)。
---

# digestly 创作预览(plan)

## 步骤

1. **找创作**:`GET http://127.0.0.1:3210/api/creations`,按名称匹配 id
2. **预览**:`POST /api/creations/{id}/plan`,body `{}`(可选 `{"template_id": N, "force": true}`)

返回:
```json
{
  "creation": {...},
  "template": {"id": 1, "name": "话题综述", "prompt": "..."},
  "feed_ids": [9, 10, 11],
  "item_ids": [767653, 767652],
  "items": [{"id": 767653, "title": "...", "summary": "...", "content": ""}],
  "history_titles": ["..."],
  "prompt": "完整拼装后的 prompt"
}
```

## 素材过多时分页细读

`items` 里**不含正文**(content 为空)。需要正文时按 id 逐条取:

- `GET /api/items/{id}` → 含完整 `content`

推荐流程:plan 拿 `item_ids` → 逐个 `GET /api/items/{id}` 细读 → 自己拼上下文生成。

## 错误

- 422 "素材源里没有未读文章":未读耗尽,加 `"force": true` 预览最新素材。
- 预览是纯读取,不会标记已读、不落库,可安全反复调用。
