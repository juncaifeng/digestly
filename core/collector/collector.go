// Package collector 定义采集器抽象与注册表。
// 内置 rss/atom/jsonfeed；扩展支持 JS 脚本(js:<name>)与 Go 插件(go:<name>)。
package collector

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/juncaifeng/digestly/core/model"
)

// Source 一次采集的输入。
type Source struct {
	FeedID int64
	URL    string
	Config map[string]string
}

// Collector 所有采集模块的统一接口。输出为尚未标注的原始 Item。
type Collector interface {
	// Name 注册名，如 "rss"、"js:hackernews"。
	Name() string
	// Collect 抓取并解析出原始文章列表。
	Collect(ctx context.Context, src Source) ([]model.Item, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Collector{}
)

// Register 注册采集器。同名后者覆盖前者。
func Register(c Collector) {
	mu.Lock()
	defer mu.Unlock()
	registry[c.Name()] = c
}

// Get 按名取采集器，支持 js: 前缀惰性解析。
func Get(name string) (Collector, error) {
	mu.RLock()
	c, ok := registry[name]
	mu.RUnlock()
	if ok {
		return c, nil
	}
	return nil, fmt.Errorf("collector %q not registered", name)
}

// Names 返回所有已注册采集器名。
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
