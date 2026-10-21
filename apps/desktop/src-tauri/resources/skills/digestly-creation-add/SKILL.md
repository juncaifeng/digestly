---
name: digestly-creation-add
description: 在 digestly 中创建/配置创作单元(名称、文件夹、大纲模板、素材源)。当用户说"添加一个创作""新建创作""配置创作素材源"时使用。需要 digestly-cli(桌面端安装时勾选 PATH 后可用)。
---

# digestly 添加创作

创作单元(Creation)是 digestly 的一等公民:一个名称 + 可选文件夹 + 大纲模板 + 素材源(可混合整个专栏和单个订阅源)。用 `digestly-cli` 操作,不依赖桌面端窗口。`-data` 默认自动探测桌面端数据目录。

## 步骤

1. **选素材源**(拿到 column/feed id):
   - `digestly-cli -cmd columns` → 专栏 id / 名称 / 源数
   - `digestly-cli -cmd feeds` → 订阅源 id / 名称 / 所属专栏
2. **选模板**:`digestly-cli -cmd templates` → 模板 id / 名称(`[内置]` 的可编辑不可删)
3. **创建**:`digestly-cli -cmd creation-add -name 日报1 -group "AI 日报" -template 1 -sources "column:1,feed:9"`
   - `-group` 可省(省略 = 未分组);`-template` / `-sources` 可之后再配
   - `-sources` 逗号分隔:`column:<id>` = 整个专栏(取其全部订阅源),`feed:<id>` = 单个订阅源

## 修改 / 删除

- `digestly-cli -cmd creation-update -id 1 -sources "feed:9,feed:10"` — 只覆盖显式传入的字段;`-sources` 是**整体替换**语义
- `digestly-cli -cmd creation-del -id 1`(历史文章一并删除)
- `digestly-cli -cmd creation-list` — 列出现有创作(id / 名称 / 文件夹 / 模板 / 源)

## 注意

- Windows 命令路径:PATH 中为 `digestly-cli`,完整路径通常 `C:\Program Files\digestly\digestly-cli.exe`。
- 生成文章见 digestly-creation-run;生成前预览见 digestly-creation-plan。
