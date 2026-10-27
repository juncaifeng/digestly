package market

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// serveMarket 起一个测试市场: /manifest.json + /demo.js。
func serveMarket(t *testing.T, manifest, script string) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(manifest))
	})
	mux.HandleFunc("/demo.js", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(script))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return &Client{ManifestURL: ts.URL + "/manifest.json", CacheDir: t.TempDir(), HTTP: ts.Client()}
}

func manifestWithChecksum(t *testing.T, script, sha string) string {
	t.Helper()
	return fmt.Sprintf(`{"collectors":[{"name":"demo","title":"Demo","url":"demo.js","script_version":1,"sha256":"%s"}]}`, sha)
}

func TestInstallChecksumOK(t *testing.T) {
	const script = `// demo collector body`
	sum := sha256.Sum256([]byte(script))
	c := serveMarket(t, manifestWithChecksum(t, script, hex.EncodeToString(sum[:])), script)
	dir := t.TempDir()

	entry, err := c.Install(context.Background(), "demo", dir)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Name != "demo" {
		t.Fatalf("entry.Name = %q", entry.Name)
	}
	got, err := os.ReadFile(filepath.Join(dir, "demo.js"))
	if err != nil || string(got) != script {
		t.Fatalf("落盘脚本不符: %v", err)
	}
}

func TestInstallChecksumMismatch(t *testing.T) {
	const script = `// demo collector body`
	c := serveMarket(t, manifestWithChecksum(t, script, strings.Repeat("0", 64)), script)
	dir := t.TempDir()

	_, err := c.Install(context.Background(), "demo", dir)
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("校验不匹配应报错,得到 %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "demo.js")); !os.IsNotExist(statErr) {
		t.Fatal("校验失败时不应落盘脚本")
	}
}

func TestInstallWithoutChecksum(t *testing.T) {
	// 老清单没有 sha256 字段: 向后兼容,不校验直接安装
	const script = `// demo collector body`
	manifest := `{"collectors":[{"name":"demo","title":"Demo","url":"demo.js","script_version":1}]}`
	c := serveMarket(t, manifest, script)
	dir := t.TempDir()

	if _, err := c.Install(context.Background(), "demo", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "demo.js")); err != nil {
		t.Fatalf("应正常落盘: %v", err)
	}
}
