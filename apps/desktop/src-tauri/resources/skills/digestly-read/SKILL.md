---
name: digestly-read
description: 浏览/搜索 digestly 已采集的资讯文章(按订阅源、专栏、未读、关键词、时间筛选),以及查看文章详情。当用户说"看看最近有什么文章""搜一下某主题""某订阅源最近更新了什么"时使用。前提是 digestly 桌面端正在运行(本地 API 在 http://127.0.0.1:3210)。
---

# digestly 浏览资讯

## 常用查询

`GET http://127.0.0.1:3210/api/items`,query 参数可组合:

| 参数 | 说明 |
| --- | --- |
| `feed_id` | 按订阅源(id 从 `GET /api/feeds` 拿) |
| `column_id` | 按专栏(含订阅继承与手动收藏) |
| `q` | 全文搜索(标题/正文/摘要/关键词) |
| `unread=1` | 只看未读 |
| `archived=1` | 看归档箱(默认只出未归档) |
| `limit` / `offset` | 分页,默认建议 limit=20 |

## 读单篇全文

`GET /api/items/{id}` → 含完整 `content`(列表接口可能裁掉正文,详情一定用这个)。

## 其他

- 标记已读/未读:`POST /api/items/{id}/read`,body `{"read": true}`
- 归档/恢复:`POST /api/items/{id}/archive`,body `{"archived": true}`
- 手动刷新某源:`POST /api/feeds/{id}/refresh`

## 注意

- 返回量大时先只列标题摘要让用户挑,再用 id 取详情,避免一次塞入过多内容。
