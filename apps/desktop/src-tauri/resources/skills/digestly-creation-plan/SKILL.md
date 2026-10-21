---
name: digestly-creation-plan
description: 预览 digestly 创作的生成输入(大纲模板 + 素材清单 + 完整 prompt),不调用 LLM、无副作用。当用户说"预览一下创作""看看这次会用什么素材""把素材/prompt 给我"时使用;也适合自己作为外部 agent 取用素材再自行生成。需要 digestly-cli(桌面端安装时勾选 PATH 后可用)。
---

# digestly 创作预览(plan)

用 `digestly-cli` 直接读本地数据库,不依赖桌面端窗口。`-data` 默认自动探测桌面端数据目录。

## 步骤

1. **找创作**:`digestly-cli -cmd creation-list`,按名称匹配 id
2. **预览**:`digestly-cli -cmd plan -creation <id 或名称>`(可加 `-format json`)

   可选参数:
   - `-template <id>` 临时覆盖默认模板
   - `-force` 不限未读,取最新素材(未读耗尽时用)
   - `-limit 20 -offset 0` 素材清单分页展示

返回(结构化可用 `-format json`):

- 创作与模板信息、展开后的订阅源 id、素材清单(id/标题/摘要,不含正文)
- `history_titles`:历史创作标题(生成时作为上下文)
- `prompt`:完整拼装后的 prompt,可直接喂给自己的模型

## 素材过多时分页细读

`plan` 只给素材**元数据**(不含正文)。取正文:

- `digestly-cli -cmd plan -creation 2 -ids-only` — 只输出素材 id 列表
- `digestly-cli -cmd get -id <素材 id>` — 按 id 逐条读全文
- 或 `digestly-cli -cmd list -limit 20 -offset 40 ...` 常规分页

## 注意

- plan 无副作用:不调 LLM、不落库、不标记已读。
- 真实生成见 digestly-creation-run;配置创作见 digestly-creation-add。
