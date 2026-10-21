---
name: digestly-read
description: 浏览/搜索 digestly 已采集的资讯文章(按订阅源、专栏、未读、关键词、时间筛选),以及查看文章详情、标记已读、归档。当用户说"看看最近有什么文章""搜一下某主题""某订阅源最近更新了什么"时使用。需要 digestly-cli(桌面端安装时勾选 PATH 后可用;否则用安装目录下的完整路径)。
---

# digestly 浏览资讯

用 `digestly-cli` 直接读本地数据库,**不需要打开桌面端窗口**。`-data` 默认自动探测桌面端数据目录;dev 联调时用 `-data data`。

## 常用查询

`digestly-cli -cmd list`,参数可组合:

| 参数 | 说明 |
| --- | --- |
| `-q <关键词>` | 全文搜索(标题/正文/摘要/关键词) |
| `-feed <id 或名称>` | 按订阅源 |
| `-column <id 或名称>` | 按专栏(含订阅继承与手动收藏) |
| `-unread` | 只看未读 |
| `-since today\|7d\|30d\|2026-10-20` | 起始时间,`-until` 同理 |
| `-tag <标签>` / `-status pending\|processed\|failed` | 附加筛选 |
| `-limit 20 -offset 0` | 分页 |
| `-format md\|text\|json` | 输出格式,默认 md(适合直接读);机器处理用 json |
| `-full` | 带正文(默认只给摘要) |

## 发现 id

- 订阅源:`digestly-cli -cmd feeds`(id / 名称 / 所属专栏 / 来源类型 / URL)
- 专栏:`digestly-cli -cmd columns`(id / 名称 / 源数 / 收藏数)

## 读单篇全文

`digestly-cli -cmd get -id <id>` — 详情一定含完整 `content`(列表接口可能裁掉正文)。

## 已读 / 归档

- `digestly-cli -cmd read -id <id>` / `-cmd unread -id <id>`
- `digestly-cli -cmd archive -id <id>` / `-cmd unarchive -id <id>`

## 注意

- 输出为 UTF-8;Windows 下自行处理时记得按 UTF-8 解码。
- 命令路径:PATH 中为 `digestly-cli`;Windows 安装版完整路径通常为 `C:\Program Files\digestly\digestly-cli.exe`。
