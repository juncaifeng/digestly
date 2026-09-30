package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/juncaifeng/digestly/core/model"
)

// JSDir 存放用户 JS 采集脚本的目录，由外层在启动时指定。
var JSDir = "data/collectors"

// LoadJSScripts 扫描 JSDir，把每个 .js 注册为 js:<文件名> 采集器。
func LoadJSScripts() error {
	entries, err := os.ReadDir(JSDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".js")
		code, err := os.ReadFile(filepath.Join(JSDir, e.Name()))
		if err != nil {
			return err
		}
		Register(&jsCollector{name: "js:" + name, code: string(code)})
	}
	return nil
}

// jsCollector 执行用户 JS 脚本。脚本约定:
//   function collect(source) { return [{guid,title,link,author,content,published}, ...] }
// 宿主注入 http.get(url) -> string，便于脚本抓取页面。
type jsCollector struct {
	name string
	code string
}

func (c *jsCollector) Name() string { return c.name }

func (c *jsCollector) Collect(ctx context.Context, src Source) ([]model.Item, error) {
	vm := goja.New()
	c.injectHTTP(vm)
	if _, err := vm.RunString(c.code); err != nil {
		return nil, fmt.Errorf("script %s: %w", c.name, err)
	}
	fn, ok := goja.AssertFunction(vm.Get("collect"))
	if !ok {
		return nil, fmt.Errorf("script %s must define collect(source)", c.name)
	}
	srcObj := vm.ToValue(map[string]any{"url": src.URL, "config": src.Config})
	v, err := fn(goja.Undefined(), srcObj)
	if err != nil {
		return nil, fmt.Errorf("script %s collect(): %w", c.name, err)
	}
	var rows []struct {
		GUID      string `json:"guid"`
		Title     string `json:"title"`
		Link      string `json:"link"`
		Author    string `json:"author"`
		Content   string `json:"content"`
		Published string `json:"published"`
	}
	if err := vm.ExportTo(v, &rows); err != nil {
		return nil, fmt.Errorf("script %s bad result: %w", c.name, err)
	}
	items := make([]model.Item, 0, len(rows))
	for _, r := range rows {
		it := model.Item{
			FeedID:  src.FeedID,
			GUID:    r.GUID,
			Title:   r.Title,
			Link:    r.Link,
			Author:  r.Author,
			Content: r.Content,
			Status:  model.StatusPending,
		}
		if it.GUID == "" {
			it.GUID = r.Link
		}
		if t, err := time.Parse(time.RFC3339, r.Published); err == nil {
			it.PublishedAt = &t
		}
		items = append(items, it)
	}
	return items, nil
}

func (c *jsCollector) injectHTTP(vm *goja.Runtime) {
	httpObj := vm.NewObject()
	_ = httpObj.Set("get", func(url string) (string, error) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "digestly/0.1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if err != nil {
			return "", err
		}
		return string(b), nil
	})
	_ = vm.Set("http", httpObj)
}
