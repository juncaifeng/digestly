# 采集器扩展

digestly 的采集模块统一实现 `core/collector.Collector` 接口:

```go
type Collector interface {
    Name() string
    Collect(ctx context.Context, src Source) ([]model.Item, error)
}
```

三种实现方式:

| 类型 | 命名 | 说明 |
|---|---|---|
| 内置 | `rss` / `atom` / `jsonfeed` | gofeed 解析，开箱即用 |
| JS 脚本 | `js:<脚本名>` | 放在 `data/collectors/*.js`，goja 执行，改完重启即生效 |
| Go 插件 | `go:<插件名>` | hashicorp go-plugin 子进程(规划中，见下) |

## JS 脚本约定

必须导出 `collect(source)`，宿主注入 `http.get(url)`:

```js
function collect(source) {
  const html = http.get(source.url);
  return [{ guid, title, link, author, content, published }];
}
```

示例见 `examples/hackernews.js`。

## Go 插件(roadmap)

计划采用 hashicorp/go-plugin 的子进程 RPC 方案:

- 插件是独立 Go 二进制，实现同一 `Collector` 接口的 RPC 封装
- server 启动时扫描插件目录并 spawn，崩溃隔离、可独立分发
- 适合性能敏感或需要复杂依赖(如 headless 渲染)的采集场景

移动端注意: gomobile 场景下子进程不可用，Go 插件仅限桌面/server 形态。
