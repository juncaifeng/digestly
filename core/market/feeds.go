package market

// 订阅源市场(第三方市场源): 解析一个地址里**明确声明**的订阅源清单,产出可订阅目录。
//
// 只认仓库/地址里写明的清单文件(OPML / feeds.json / manifest feeds 节),
// 有就解析,没有就报"没有清单"——不猜测、不拿仓库 commits/releases 凑订阅源。

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MarketFeed 市场里一条可订阅的订阅源(只是目录条目,不负责采集)。
type MarketFeed struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	FeedURL     string   `json:"feed_url"` // 真正的 RSS/Atom 地址(含 RSSHub 这类网关地址)
	SiteURL     string   `json:"site_url"`
	Tags        []string `json:"tags"`
	Catalog     string   `json:"catalog"` // 解析出它的清单文件地址
}

// rawBase GitHub 仓库清单文件基址,测试时可替换。
var rawBase = "https://raw.githubusercontent.com"

// catalogFiles 仓库里认作"订阅源清单"的固定文件名(按顺序找,首个命中生效)。
var catalogFiles = []string{"feeds.opml", "feeds.json", "opml.xml", "manifest.json"}

// ParseFeeds 解析市场源地址,返回其中声明的订阅源。
// 不传 client 时用默认 15s 超时客户端。
func ParseFeeds(ctx context.Context, client *http.Client, rawURL string) ([]MarketFeed, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if _, err := url.Parse(rawURL); err != nil {
		return nil, fmt.Errorf("地址不合法: %w", err)
	}
	var semantic error // 格式认得但内容不合格(如指向了采集器清单)——如实上报
	var notFound, netErr int
	for _, u := range catalogURLs(rawURL) {
		body, err := fetch(ctx, client, u)
		if err != nil {
			// 只有明确 404 才算"文件不存在";网络失败不许冒充"没有清单"
			if isNotFound(err) {
				notFound++
			} else {
				netErr++
			}
			continue
		}
		feeds, err := parseCatalog(u, body)
		if err != nil {
			if !strings.Contains(err.Error(), "无法识别") {
				semantic = err
			}
			continue
		}
		if len(feeds) == 0 {
			continue
		}
		return feeds, nil
	}
	if semantic != nil {
		return nil, fmt.Errorf("%s: %w", rawURL, semantic)
	}
	if notFound == 0 && netErr > 0 {
		return nil, fmt.Errorf("拉取清单失败(网络不通或被拦截),未能确认 %s 有无订阅源", rawURL)
	}
	return nil, fmt.Errorf("该地址没有声明订阅源清单(找过 %s),没有可解析的源",
		strings.Join(catalogFiles, " / "))
}

// catalogURLs 把用户填的地址展开为候选清单文件地址(按顺序尝试)。
func catalogURLs(rawURL string) []string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	p := strings.Trim(u.Path, "/")

	// 1) 直接指向清单文件: raw 地址、GitHub blob 地址、或任意 .opml/.json/.xml
	if strings.Contains(u.Host, "raw.githubusercontent.com") {
		return []string{rawURL}
	}
	if i := strings.Index(p, "/blob/"); i >= 0 {
		parts := strings.SplitN(p[i+len("/blob/"):], "/", 2)
		if len(parts) == 2 {
			return []string{fmt.Sprintf("%s/%s/%s/%s", rawBase, p[:i], parts[0], parts[1])}
		}
	}
	if strings.HasSuffix(strings.ToLower(p), ".opml") ||
		strings.HasSuffix(strings.ToLower(p), ".json") ||
		strings.HasSuffix(strings.ToLower(p), ".xml") {
		return []string{rawURL}
	}

	// 2) GitHub 仓库: 尝试固定清单文件名
	if u.Host == "github.com" || u.Host == "www.github.com" {
		segs := strings.Split(p, "/")
		if len(segs) == 2 && segs[0] != "" && segs[1] != "" {
			out := make([]string, 0, len(catalogFiles))
			for _, f := range catalogFiles {
				out = append(out, fmt.Sprintf("%s/%s/%s/HEAD/%s", rawBase, segs[0], segs[1], f))
			}
			return out
		}
	}
	return nil
}

func fetch(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{code: resp.StatusCode}
	}
	return readLimited(resp.Body)
}

// parseCatalog 按内容嗅探清单格式并解析(OPML 优先,其次 JSON)。
func parseCatalog(catalogURL string, body []byte) ([]MarketFeed, error) {
	trimmed := strings.TrimLeft(string(body), " \t\r\n")
	trimmed = strings.TrimPrefix(trimmed, "")
	switch {
	case strings.HasPrefix(trimmed, "<"):
		return parseOPML(catalogURL, body)
	case strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["):
		return parseJSONCatalog(catalogURL, body)
	}
	return nil, fmt.Errorf("无法识别的清单格式")
}

// --- OPML ---

type opmlDoc struct {
	Body opmlBody `xml:"body"`
}
type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}
type opmlOutline struct {
	Text     string        `xml:"text,attr"`
	Title    string        `xml:"title,attr"`
	Category string        `xml:"category,attr"`
	XMLURL   string        `xml:"xmlUrl,attr"`
	HTMLURL  string        `xml:"htmlUrl,attr"`
	Children []opmlOutline `xml:"outline"`
}

// parseOPML 解析 OPML 订阅列表(常见于精选订阅集合)。
// 只收带 xmlUrl 的节点;分组节点(无 xmlUrl)的名称作为标签下传。
func parseOPML(catalogURL string, body []byte) ([]MarketFeed, error) {
	var doc opmlDoc
	if err := xmlUnmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("OPML 解析失败: %w", err)
	}
	var out []MarketFeed
	var walk func(nodes []opmlOutline, tag string)
	walk = func(nodes []opmlOutline, tag string) {
		for _, n := range nodes {
			cur := tag
			if n.XMLURL == "" {
				cur = strings.TrimSpace(firstNonEmpty(n.Title, n.Text))
			}
			if n.XMLURL != "" {
				title := strings.TrimSpace(firstNonEmpty(n.Title, n.Text))
				tags := []string{}
				if cur != "" {
					tags = append(tags, cur)
				}
				if n.Category != "" {
					tags = append(tags, n.Category)
				}
				out = append(out, MarketFeed{
					Name:        n.XMLURL,
					Title:       firstNonEmpty(title, n.XMLURL),
					Description: "来自订阅清单 " + catalogURL,
					FeedURL:     n.XMLURL,
					SiteURL:     n.HTMLURL,
					Tags:        tags,
					Catalog:     catalogURL,
				})
			}
			walk(n.Children, cur)
		}
	}
	walk(doc.Body.Outlines, "")
	if len(out) == 0 {
		return nil, fmt.Errorf("OPML 里没有带 xmlUrl 的订阅源")
	}
	return out, nil
}

// --- JSON 清单 ---
// 兼容两种形态:
//   {"feeds": [{"title","feed_url","site_url","description","tags"}]}
//   [{"title","feed_url",...}]   (裸数组)
// feed_url 也可写作 url;RSSHub 这类网关地址就是普通 feed_url,原样返回。

type jsonFeed struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	FeedURL     string   `json:"feed_url"`
	URL         string   `json:"url"`
	SiteURL     string   `json:"site_url"`
	Tags        []string `json:"tags"`
}

func parseJSONCatalog(catalogURL string, body []byte) ([]MarketFeed, error) {
	var list []jsonFeed
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "[") {
		if err := jsonUnmarshal(body, &list); err != nil {
			return nil, fmt.Errorf("JSON 清单解析失败: %w", err)
		}
	} else {
		var wrapper struct {
			Feeds     []jsonFeed `json:"feeds"`
			SourceURL string     `json:"source_url"`
			Catalog   []jsonFeed `json:"sources"`
		}
		if err := jsonUnmarshal(body, &wrapper); err != nil {
			return nil, fmt.Errorf("JSON 清单解析失败: %w", err)
		}
		list = append(wrapper.Feeds, wrapper.Catalog...)
		// 声明为采集器清单(而非订阅源清单)时不冒充
		if len(list) == 0 {
			var coll struct {
				Collectors []jsonFeed `json:"collectors"`
			}
			if err := jsonUnmarshal(body, &coll); err == nil && len(coll.Collectors) > 0 {
				return nil, fmt.Errorf("这是采集器清单(manifest),不是订阅源清单")
			}
		}
	}
	out := make([]MarketFeed, 0, len(list))
	for _, f := range list {
		feedURL := firstNonEmpty(f.FeedURL, f.URL)
		if feedURL == "" {
			continue
		}
		tags := f.Tags
		if tags == nil {
			tags = []string{}
		}
		if len(tags) == 0 {
			tags = []string{"json"}
		}
		out = append(out, MarketFeed{
			Name:        firstNonEmpty(f.Name, feedURL),
			Title:       firstNonEmpty(f.Title, feedURL),
			Description: f.Description,
			FeedURL:     feedURL,
			SiteURL:     f.SiteURL,
			Tags:        tags,
			Catalog:     catalogURL,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("JSON 清单里没有订阅源条目")
	}
	return out, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// readLimited 限量读取响应体(清单文件不该超过 2MB)。
func readLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, 2<<20))
}

func xmlUnmarshal(data []byte, v any) error  { return xml.Unmarshal(data, v) }
func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// httpError 区分"文件不存在"与网络失败,避免把断网误报成"没有清单"。
type httpError struct{ code int }

func (e *httpError) Error() string { return fmt.Sprintf("HTTP %d", e.code) }

func isNotFound(err error) bool {
	var he *httpError
	return errors.As(err, &he) && (he.code == http.StatusNotFound || he.code == http.StatusForbidden)
}
