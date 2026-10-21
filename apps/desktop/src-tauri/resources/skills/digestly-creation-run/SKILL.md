---
name: digestly-creation-run
description: 触发 digestly 创作单元生成文章(基于最新未读素材 + 大纲模板 + LLM)。当用户说"开始创作""生成一篇日报/周报""跑一下某创作"时使用。需要 digestly-cli(桌面端安装时勾选 PATH 后可用)。
---

# digestly 开始创作

用 `digestly-cli` 触发生成:拼装素材 → 调 LLM → 文章落库 → 素材标记已读。`-data` 默认自动探测桌面端数据目录。

## 步骤

1. **找创作**:`digestly-cli -cmd creation-list`,按名称匹配 id(名称来自用户口述,模糊匹配后确认)
2. **生成**:`digestly-cli -cmd generate -creation <id 或名称>`

   可选参数:
   - `-template <id>` 临时覆盖默认模板
   - `-force` 不限未读,取最新素材(未读耗尽后强制生成)
   - `-format json` 输出文章 JSON(默认输出 markdown 正文)

   成功输出文章标题与正文,汇报要点给用户。

## 常见错误处理

- **「素材源里没有未读文章」**:未读素材已耗尽。告知用户并询问是否强制生成;确认后加 `-force` 重跑(取最新素材,不限未读)。
- **「该创作未关联大纲模板」**:先用 digestly-creation-add 配模板,或临时传 `-template <id>`。
- **「还没有默认 LLM 配置」**:让用户去 digestly 设置页配大模型。
- 生成会消耗素材(标记已读),是**真实副作用**;生成耗时十几秒属正常。

## 相关

- 生成前先看素材 → digestly-creation-plan
- 历史文章:`digestly-cli -cmd list ...` 或按创作查看用 `-cmd get -id <文章 id>`(文章 id 来自生成结果)
