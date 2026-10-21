// 外部 Agent 进程生成后端: 通过子进程调用本地 agent CLI(dsh / openclaw / claude 等),
// prompt 经 {prompt_file} 占位文件或 stdin 传入, 从 stdout 取生成文本。
// 与 Chat 互斥二选一, 由 LLMConfig.Kind 决定。
package llm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// agentTimeout 外部 agent 跑一次创作的上限(允许工具调用/多轮, 比直连 LLM 长很多)。
const agentTimeout = 10 * time.Minute

// RunAgent 运行外部 agent 命令生成文本。
//
// command 是命令模板:
//   - 含 {prompt_file} 时, prompt 写入临时文件并替换该占位(推荐长 prompt, 如 openclaw --message-file);
//   - 不含时, prompt 从 stdin 传入(如 claude -p、dsh --profile headless)。
//
// stdout 整体作为生成结果返回(截断到 4MiB); 非零退出码返回含 stderr 的错误。
func RunAgent(ctx context.Context, command, prompt string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("agent 命令为空, 请在配置里填写命令模板")
	}
	ctx, cancel := context.WithTimeout(ctx, agentTimeout)
	defer cancel()

	// shell 执行以支持管道/引号; Windows 用 cmd /C
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	if strings.Contains(command, "{prompt_file}") {
		f, err := os.CreateTemp("", "digestly-prompt-*.md")
		if err != nil {
			return "", err
		}
		defer os.Remove(f.Name())
		if _, err := f.WriteString(prompt); err != nil {
			f.Close()
			return "", err
		}
		if err := f.Close(); err != nil {
			return "", err
		}
		// Windows 路径含反斜杠, sh 模式下需保留原样; cmd /C 中引号包裹
		p := f.Name()
		if runtime.GOOS == "windows" {
			p = strings.ReplaceAll(p, "/", "\\")
			cmd = exec.CommandContext(ctx, "cmd", "/C", strings.ReplaceAll(command, "{prompt_file}", `"`+p+`"`))
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", strings.ReplaceAll(command, "{prompt_file}", p))
		}
		cmd.Env = os.Environ()
	} else {
		cmd.Stdin = strings.NewReader(prompt)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("agent 执行超时(%s): %w", agentTimeout, err)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("agent 执行失败: %s", truncRunes(msg, 2000))
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return "", fmt.Errorf("agent 无输出(stderr: %s)", truncRunes(strings.TrimSpace(stderr.String()), 500))
	}
	if len(out) > 4<<20 {
		out = out[:4<<20]
	}
	return out, nil
}

// truncRunes 按 rune 截断, 防止把多字节字符切坏。
func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
