package core

import (
	"context"
	"fmt"

	"wind-agent/internal/domain"
)

// LLM 是推理循环依赖的模型端口（接口定义在使用方 core）。
type LLM interface {
	ChatStream(
		ctx context.Context,
		msgs []domain.Message,
		tools []ToolSpec,
		emit func(domain.Event) error,
	) (*domain.Message, error)
}

// maxSteps 限制一次对话里「推理→调工具」的最大轮数，防止模型死循环。
const maxSteps = 128

// AgentLoop 推理循环（ReAct）：反复调 LLM，直到没有 tool_calls 为止。
type AgentLoop struct {
	llms  *Registry
	tools *ToolRegistry
}

func NewAgentLoop(llms *Registry, tools *ToolRegistry) *AgentLoop {
	return &AgentLoop{llms: llms, tools: tools}
}

// Run 用给定消息跑一轮推理，事件通过 emit 推出，返回结束后的完整消息。
func (l *AgentLoop) Run(
	ctx context.Context,
	providerID string,
	msgs []domain.Message,
	emit func(domain.Event) error,
) ([]domain.Message, error) {
	llm, ok := l.llms.Get(providerID)
	if !ok {
		return msgs, fmt.Errorf("unknown provider %q", providerID)
	}

	var specs []ToolSpec
	if l.tools != nil {
		specs = l.tools.Specs()
	}

	for range maxSteps {
		assistant, err := llm.ChatStream(ctx, msgs, specs, emit)
		if err != nil {
			return msgs, err
		}
		msgs = append(msgs, *assistant)

		// 没有工具调用 → 这就是最终回复
		if len(assistant.ToolCalls) == 0 {
			return msgs, nil
		}

		// 有工具调用 → 逐个执行，把结果作为 tool 消息回填，再来一轮
		for _, tc := range assistant.ToolCalls {
			if err := emit(domain.Event{Type: domain.EventToolCall, ToolCall: &tc}); err != nil {
				return msgs, err
			}
			out, err := l.tools.Execute(ctx, tc.Name, tc.Args)
			if err != nil {
				out = "tool error: " + err.Error()
			}
			msgs = append(msgs, domain.Message{
				Role:       domain.RoleTool,
				ToolCallID: tc.ID,
				Content:    out,
			})
			if err := emit(domain.Event{Type: domain.EventToolResult, Text: out}); err != nil {
				return msgs, err
			}
		}
	}

	return msgs, fmt.Errorf("agent loop: exceeded %d steps", maxSteps)
}
