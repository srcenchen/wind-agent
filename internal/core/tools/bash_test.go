package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBashRunsCommand(t *testing.T) {
	out, err := Bash{}.Execute(context.Background(), `{"command":"echo hello"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Fatalf("out = %q", out)
	}
}

func TestBashReportsNonZeroExit(t *testing.T) {
	out, err := Bash{}.Execute(context.Background(), `{"command":"echo boom >&2; exit 3"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "boom") || !strings.Contains(out, "退出码 3") {
		t.Fatalf("out = %q", out)
	}
}

func TestBashTimeout(t *testing.T) {
	start := time.Now()
	out, err := Bash{}.Execute(context.Background(), `{"command":"sleep 5","timeout_seconds":1}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "命令超时") {
		t.Fatalf("out = %q", out)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("timeout not enforced, took %s", elapsed)
	}
}

func TestBashUsesWorkdir(t *testing.T) {
	dir := t.TempDir()
	out, err := Bash{}.Execute(context.Background(), `{"command":"pwd","workdir":"`+dir+`"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, dir) {
		t.Fatalf("out = %q, want workdir %q", out, dir)
	}
}

func TestBashRejectsEmptyCommand(t *testing.T) {
	if _, err := (Bash{}).Execute(context.Background(), `{"command":"  "}`); err == nil {
		t.Fatal("expected error for empty command")
	}
}
