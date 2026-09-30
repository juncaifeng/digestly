// 示例 JS 采集器: 抓取 Hacker News 官方 API 的 topstories。
// 放入 data/collectors/ 目录后，feed 的 collector 填 "js:hackernews" 即可使用。
//
// 脚本约定: 必须定义 collect(source)，返回文章数组。
// 宿主注入: http.get(url) -> string (同步阻塞)
// source: { url: string, config: Record<string,string> }

function collect(source) {
  const ids = JSON.parse(http.get("https://hacker-news.firebaseio.com/v0/topstories.json"));
  const items = [];
  for (const id of ids.slice(0, 20)) {
    const s = JSON.parse(
      http.get("https://hacker-news.firebaseio.com/v0/item/" + id + ".json")
    );
    if (!s || !s.title) continue;
    items.push({
      guid: "hn-" + s.id,
      title: s.title,
      link: s.url || "https://news.ycombinator.com/item?id=" + s.id,
      author: s.by || "",
      content: (s.text || "") + "<p>Score: " + (s.score || 0) + "</p>",
      published: new Date((s.time || 0) * 1000).toISOString(),
    });
  }
  return items;
}
