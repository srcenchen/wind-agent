# AGENTS.md

本文件为在此仓库中工作的 AI 代理提供指引。

## 项目简介

- **wind-agent**：一个 Go 项目，模块名 `wind-agent`。
- 当前处于起步阶段，`cmd/wind_agent/main.go` 为空入口，功能待实现。

## 技术栈

- 语言：Go 1.26
- 包管理：Go Modules（`go.mod`）

## 常用命令

| 用途 | 命令 |
| --- | --- |
| 编译 | `go build ./...` |
| 运行 | `go run .` |
| 测试 | `go test ./...` |
| 静态检查 | `go vet ./...` |
| 格式化 | `gofmt -w .` |
| 依赖整理 | `go mod tidy` |

## 目录结构

```
wind-agent/
├── main.go          # 程序入口
├── go.mod           # 模块定义
├── opencode.json    # opencode 项目配置（含 MCP）
└── AGENTS.md        # 本文件
```

## 代码约定

- 遵循标准 Go 风格，提交前运行 `gofmt`。
- 不添加无意义的注释；仅在逻辑不直观时补充说明。
- 新增依赖前先确认是否必要，并使用 `go mod tidy` 同步。

## MCP 服务器

项目通过 `opencode.json` 配置了一个名为 `goland` 的远程 MCP 服务器（GoLand 内置）：

- 类型：`remote`（SSE）
- 地址：`http://127.0.0.1:64422/sse`
- 请求头：`IJ_MCP_SERVER_PROJECT_PATH` 指向本项目根目录
- 能力：约 44 个工具，涵盖构建/运行配置、文件读写与搜索、符号分析、重构、终端执行、代码检查，以及数据库/SQL 操作

该服务由 GoLand 提供，仅在本项目内生效。使用前请确认 IDE 已启动且 MCP 服务已开启，否则连接会失败。
