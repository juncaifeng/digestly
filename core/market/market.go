// Package market 实现订阅源市场: 从远端 manifest 发现 JS 采集器并安装到本地。
// 默认市场仓库: github.com/juncaifeng/digestly-collectors
//
// 清单读取策略: 本地缓存优先(瞬时返回),后台刷新或显式 refresh 才回源远端;
// 网络失败时永远有缓存兜底,列表接口不做任何网络 IO。
package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultManifestURL 官方市场清单地址,可用环境变量 DIGESTLY_MARKET_URL 覆盖。
const DefaultManifestURL = "https://raw.githubusercontent.com/juncaifeng/digestly-collectors/main/manifest.json"

// StaleAfter 超过该时长未刷新则视为过期,触发后台更新。
const StaleAfter = time.Hour

// ConfigField manifest 中声明的采集器配置项。
type ConfigField struct {
	Type    string `json:"type"`
	Default any    `json:"default"`
}

// Entry 市场里一个可安装的采集器。
type Entry struct {
	Name          string                 `json:"name"`
	Title         string                 `json:"title"`
	Description   string                 `json:"description"`
	URL           string                 `json:"url"` // 相对 manifest 的路径或绝对 URL
	SourceURL     string                 `json:"source_url"`
	Tags          []string               `json:"tags"`
	ScriptVersion int                    `json:"script_version"`
	ConfigSchema  map[string]ConfigField `json:"config_schema"`
}

// Manifest 市场清单。
type Manifest struct {
	Version    int     `json:"version"`
	Collectors []Entry `json:"collectors"`
}

// Client 市场客户端,带本地缓存(离线时降级用缓存)。
type Client struct {
	ManifestURL string
	CacheDir    string
	HTTP        *http.Client

	mu          sync.Mutex
	manifest    *Manifest // 内存中的最新清单
	loadedAt    time.Time // 该清单最后一次成功从远端拉取的时间
	refreshing  bool
}

func NewClient(cacheDir string) *Client {
	url := os.Getenv("DIGESTLY_MARKET_URL")
	if url == "" {
		url = DefaultManifestURL
	}
	c := &Client{
		ManifestURL: url,
		CacheDir:    cacheDir,
		HTTP:        &http.Client{Timeout: 15 * time.Second},
	}
	// 启动预热: 磁盘缓存 → 内存,失败不致命(首次无缓存时 manifest 为空)
	c.loadCache()
	return c
}

// List 瞬时返回缓存清单(永不阻塞在网络),缓存过期时触发后台刷新。
// 返回的 Manifest 仅供读取,调用方不得修改。
func (c *Client) List(ctx context.Context) (*Manifest, error) {
	c.mu.Lock()
	m := c.manifest
	stale := time.Since(c.loadedAt) > StaleAfter
	needRefresh := m == nil || stale
	shouldKick := needRefresh && !c.refreshing
	if shouldKick {
		c.refreshing = true
	}
	c.mu.Unlock()

	if m == nil {
		return nil, fmt.Errorf("市场清单尚未就绪,请稍后重试或点刷新")
	}
	if shouldKick {
		go func() {
			defer func() {
				c.mu.Lock()
				c.refreshing = false
				c.mu.Unlock()
			}()
			_, _ = c.Refresh(context.Background())
		}()
	}
	return m, nil
}

// Refresh 显式回源远端拉取清单并更新缓存;失败时保留现有缓存。
func (c *Client) Refresh(ctx context.Context) (*Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ManifestURL, nil)
	if err != nil {
		return c.fallback(err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return c.fallback(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return c.fallback(err)
	}
	if resp.StatusCode != http.StatusOK {
		return c.fallback(fmt.Errorf("manifest HTTP %d", resp.StatusCode))
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return c.fallback(fmt.Errorf("bad manifest: %w", err))
	}
	if c.CacheDir != "" {
		_ = os.MkdirAll(c.CacheDir, 0o755)
		_ = os.WriteFile(filepath.Join(c.CacheDir, "manifest.json"), body, 0o644)
	}
	c.mu.Lock()
	c.manifest = &m
	c.loadedAt = time.Now()
	c.mu.Unlock()
	return &m, nil
}

// fallback 拉取失败时: 优先用当前内存清单,其次磁盘缓存。
func (c *Client) fallback(cause error) (*Manifest, error) {
	c.mu.Lock()
	m := c.manifest
	c.mu.Unlock()
	if m != nil {
		return m, nil
	}
	m2, err := c.loadCache()
	if err != nil {
		return nil, fmt.Errorf("manifest 拉取失败且无本地缓存: %w", cause)
	}
	return m2, nil
}

// loadCache 从磁盘读缓存到内存(启动预热与兜底用)。
func (c *Client) loadCache() (*Manifest, error) {
	body, err := os.ReadFile(filepath.Join(c.CacheDir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.manifest = &m
	// 缓存文件的修改时间即上次成功刷新时间
	if fi, err := os.Stat(filepath.Join(c.CacheDir, "manifest.json")); err == nil {
		c.loadedAt = fi.ModTime()
	}
	c.mu.Unlock()
	return &m, nil
}

// Staleness 返回清单是否过期、上次成功刷新时间。
func (c *Client) Staleness() (stale bool, lastFetch time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	last := c.loadedAt
	return last.IsZero() || time.Since(last) > StaleAfter, last
}

// Install 下载指定采集器脚本到 destDir/<name>.js,返回其条目信息。
// 总是先刷新一次清单,保证拿到的是最新版本号与脚本地址。
func (c *Client) Install(ctx context.Context, name, destDir string) (*Entry, error) {
	m, err := c.Refresh(ctx)
	if err != nil {
		c.mu.Lock()
		m = c.manifest
		c.mu.Unlock()
	}
	if m == nil {
		return nil, fmt.Errorf("market manifest unavailable: %v", err)
	}
	var entry *Entry
	for i := range m.Collectors {
		if m.Collectors[i].Name == name {
			entry = &m.Collectors[i]
			break
		}
	}
	if entry == nil {
		return nil, fmt.Errorf("collector %q not found in market", name)
	}
	scriptURL := entry.URL
	if !strings.HasPrefix(scriptURL, "http") {
		// 相对路径: 基于 manifest URL 的目录解析
		base := c.ManifestURL[:strings.LastIndex(c.ManifestURL, "/")+1]
		scriptURL = base + entry.URL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scriptURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", scriptURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(destDir, name+".js"), body, 0o644); err != nil {
		return nil, err
	}
	return entry, nil
}

// DefaultConfig 按 config_schema 实例化默认配置(JSON 字符串)。
func DefaultConfig(e *Entry) string {
	cfg := map[string]string{}
	for k, f := range e.ConfigSchema {
		cfg[k] = fmt.Sprint(f.Default)
	}
	b, _ := json.Marshal(cfg)
	return string(b)
}
