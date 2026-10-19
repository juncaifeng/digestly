package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestListReadsCacheWithoutNetwork(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"version":1,"collectors":[]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := NewClient(dir)
	c.ManifestURL = srv.URL
	if _, err := c.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := atomic.LoadInt32(&hits)

	// 冷却期内 List 不回源
	for i := 0; i < 5; i++ {
		c.List(context.Background())
	}
	if got := atomic.LoadInt32(&hits); got != before {
		t.Fatalf("List must not hit network when fresh: %d -> %d", before, got)
	}
}

func TestListKicksBackgroundRefreshWhenStale(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"version":1,"collectors":[]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := NewClient(dir)
	c.ManifestURL = srv.URL
	if _, err := c.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}

	// 人为制造过期
	c.mu.Lock()
	c.loadedAt = time.Now().Add(-2 * time.Hour)
	c.mu.Unlock()

	before := atomic.LoadInt32(&hits)
	c.List(context.Background())
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&hits) <= before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&hits) <= before {
		t.Fatal("stale cache must trigger background refresh")
	}
	stale, _ := c.Staleness()
	if stale {
		t.Fatal("cache should be fresh after background refresh")
	}
}

func TestOfflineFallsBackToDiskCache(t *testing.T) {
	dir := t.TempDir()
	body := `{"version":1,"collectors":[{"name":"x","title":"X","url":"x.js","script_version":1}]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// 指向一个必然失败的地址
	c := NewClient(dir)
	c.ManifestURL = "http://127.0.0.1:1/manifest.json"
	c.HTTP = &http.Client{Timeout: 200 * time.Millisecond}

	m, _ := c.List(context.Background())
	if len(m.Collectors) != 1 || m.Collectors[0].Name != "x" {
		t.Fatalf("disk cache fallback broken: %+v", m)
	}
}

func TestRefreshFailureKeepsExistingManifest(t *testing.T) {
	dir := t.TempDir()
	c := NewClient(dir)
	c.ManifestURL = "http://127.0.0.1:1/manifest.json"
	c.HTTP = &http.Client{Timeout: 200 * time.Millisecond}
	c.mu.Lock()
	c.manifest = &Manifest{Version: 9}
	c.mu.Unlock()

	if _, err := c.Refresh(context.Background()); err != nil {
		t.Fatal("refresh falls back to existing manifest, err =", err)
	}
	m, _ := c.List(context.Background())
	if m.Version != 9 {
		t.Fatalf("existing manifest must survive failed refresh: %+v", m)
	}
}
