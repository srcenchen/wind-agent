package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultBashTimeout = 30 * time.Second
	maxBashTimeout     = 5 * time.Minute
	maxBashOutput      = 64 * 1024
)

// Bash 在 shell 里执行命令并返回输出，供 agent 做文件/构建/运行等操作。
type Bash struct {
	WorkDir string        // 默认工作目录，空则用进程当前目录
	Timeout time.Duration // 默认超时，0 则 30s
}

type bashArgs struct {
	Command        string `json:"command"`
	WorkDir        string `json:"workdir"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func (Bash) Name() string { return "bash" }

func (Bash) Description() string {
	return "在本地 shell（bash -c）执行命令，返回标准输出与错误。适合查看文件、搜索、构建、运行测试等；破坏性命令请谨慎。"
}

func (Bash) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "要执行的 shell 命令"
			},
			"workdir": {
				"type": "string",
				"description": "命令的工作目录，默认当前项目目录"
			},
			"timeout_seconds": {
				"type": "integer",
				"description": "超时秒数，默认 30，最大 300",
				"minimum": 1,
				"maximum": 300
			}
		},
		"required": ["command"]
	}`)
}

func (b Bash) Execute(ctx context.Context, args string) (string, error) {
	var in bashArgs
	if err := json.Unmarshal([]byte(args), &in); err != nil {
		return "", fmt.Errorf("bash: 参数解析失败: %w", err)
	}
	if strings.TrimSpace(in.Command) == "" {
		return "", fmt.Errorf("bash: command 不能为空")
	}

	timeout := b.Timeout
	if timeout <= 0 {
		timeout = defaultBashTimeout
	}
	if in.TimeoutSeconds > 0 {
		timeout = time.Duration(in.TimeoutSeconds) * time.Second
	}
	if timeout > maxBashTimeout {
		timeout = maxBashTimeout
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "bash", "-c", in.Command)
	dir := in.WorkDir
	if dir == "" {
		dir = b.WorkDir
	}
	if dir != "" {
		cmd.Dir = dir
	}

	out := &limitedBuffer{limit: maxBashOutput}
	cmd.Stdout = out
	cmd.Stderr = out

	err := cmd.Run()
	text := out.String()
	if out.dropped {
		text += "\n[输出已截断]"
	}

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return text + fmt.Sprintf("\n[命令超时（%s）已终止]", timeout), nil
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Sprintf("%s\n[退出码 %d]", text, exitErr.ExitCode()), nil
		}
		return text, fmt.Errorf("bash: 执行失败: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return "[命令执行成功，无输出]", nil
	}
	return text, nil
}

// limitedBuffer 限制累积输出大小，超出部分丢弃但仍接受写入。
type limitedBuffer struct {
	buf     bytes.Buffer
	limit   int
	dropped bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	remain := l.limit - l.buf.Len()
	if remain <= 0 {
		l.dropped = true
		return len(p), nil
	}
	if len(p) > remain {
		_, _ = l.buf.Write(p[:remain])
		l.dropped = true
		return len(p), nil
	}
	return l.buf.Write(p)
}

func (l *limitedBuffer) String() string { return l.buf.String() }
