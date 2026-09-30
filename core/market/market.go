// Package market 实现订阅源市场: 从远端 manifest 发现 JS 采集器并安装到本地。
// 默认市场仓库: github.com/juncaifeng/digestly-collectors
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
	"time"
)

// DefaultManifestURL 官方市场清单地址,可用环境变量 DIGESTLY_MARKET_URL 覆盖。
const DefaultManifestURL = "https://raw.githubusercontent.com/juncaifeng/digestly-collectors/main/manifest.json"

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
}

func NewClient(cacheDir string) *Client {
	url := os.Getenv("DIGESTLY_MARKET_URL")
	if url == "" {
		url = DefaultManifestURL
	}
	return &Client{
		ManifestURL: url,
		CacheDir:    cacheDir,
		HTTP:        &http.Client{Timeout: 15 * time.Second},
	}
}

// List 拉取清单;网络失败时回退到缓存。
func (c *Client) List(ctx context.Context) (*Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ManifestURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return c.loadCache()
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return c.loadCache()
	}
	if resp.StatusCode != http.StatusOK {
		return c.loadCache()
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("bad manifest: %w", err)
	}
	if c.CacheDir != "" {
		_ = os.MkdirAll(c.CacheDir, 0o755)
		_ = os.WriteFile(filepath.Join(c.CacheDir, "manifest.json"), body, 0o644)
	}
	return &m, nil
}

func (c *Client) loadCache() (*Manifest, error) {
	body, err := os.ReadFile(filepath.Join(c.CacheDir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("manifest 拉取失败且无本地缓存: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Install 下载指定采集器脚本到 destDir/<name>.js,返回其条目信息。
func (c *Client) Install(ctx context.Context, name, destDir string) (*Entry, error) {
	m, err := c.List(ctx)
	if err != nil {
		return nil, err
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
