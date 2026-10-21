package llm

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// echoCmd 返回一个把 stdin 原样打到 stdout 的跨平台命令。
func echoCmd() string {
	if runtime.GOOS == "windows" {
		return `more`
	}
	return `cat`
}

func TestRunAgentStdin(t *testing.T) {
	out, err := RunAgent(context.Background(), echoCmd(), "hello 世界", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hello 世界") {
		t.Fatalf("stdout = %q, want contains prompt", out)
	}
}

func TestRunAgentPromptFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows cmd 路径引用差异,人工验证")
	}
	// 用 shell 读文件并回显,验证 {prompt_file} 替换
	out, err := RunAgent(context.Background(), `cat {prompt_file}`, "文件内容 ABC", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "文件内容 ABC") {
		t.Fatalf("stdout = %q, want file content", out)
	}
}

func TestRunAgentErrors(t *testing.T) {
	if _, err := RunAgent(context.Background(), "", "x", nil); err == nil {
		t.Fatal("empty command should error")
	}
	// 非零退出码
	if _, err := RunAgent(context.Background(), "exit 3", "x", nil); err == nil {
		t.Fatal("non-zero exit should error")
	}
}

func TestRunAgentTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows sleep 语义不同")
	}
	if testing.Short() {
		t.Skip("-short 跳过超时用例")
	}
	// 用小超时单独测 ctx: RunAgent 内部超时是 10min,这里测外部 ctx 取消
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := RunAgent(ctx, "sleep 5", "x", nil); err == nil {
		t.Fatal("should error on ctx timeout")
	}
}
