---
name: digestly-creation-add
description: 在 digestly 中创建/配置创作单元(名称、文件夹、大纲模板、素材源)。当用户说"添加一个创作""新建创作""配置创作素材源"时使用。前提是 digestly 桌面端正在运行(本地 API 在 http://127.0.0.1:3210)。
---

# digestly 添加创作

创作单元(Creation)是 digestly 的一等公民:一个名称 + 可选文件夹 + 大纲模板 + 素材源(可混合整个专栏和单个订阅源)。

## 步骤

1. **选素材源**:先列出专栏和订阅源,让用户挑(或按用户说的名称匹配 id)
   - `GET http://127.0.0.1:3210/api/columns` → `[{id, name, feed_count, ...}]`
   - `GET http://127.0.0.1:3210/api/feeds` → `[{id, title, column_id, unread_count, ...}]`
2. **选模板**:`GET http://127.0.0.1:3210/api/templates` → `[{id, name, built_in, ...}]`
3. **创建**:`POST /api/creations`,body `{"name": "日报1", "group_name": "AI 日报"}`(group_name 可省)
4. **配置**:`PATCH /api/creations/{id}`,body:
   ```json
   {
     "name": "日报1",
     "group_name": "AI 日报",
     "template_id": 1,
     "sources": [{"kind": "column", "ref_id": 1}, {"kind": "feed", "ref_id": 9}]
   }
   ```
   - `kind: "column"` = 整个专栏(取其全部订阅源);`kind: "feed"` = 单个订阅源
   - sources 是整体替换语义

## 其他操作

- 列出全部创作:`GET /api/creations`
- 删除创作(历史文章一并删除):`DELETE /api/creations/{id}`

## 注意

- 服务端不在时(桌面端没开),所有请求会连接失败,提示用户先打开 digestly。
- 创建后建议用 digestly-creation-plan 预览,或直接 digestly-creation-run 生成。
