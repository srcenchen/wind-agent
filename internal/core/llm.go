package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"wind-agent/internal/config"
	"wind-agent/internal/domain"

	"github.com/sashabaranov/go-openai"
)

// LLMClient 封装一层 LLM 客户端
type LLMClient struct {
	client *openai.Client
	model  string
}

// NewLLMClient 构建LLM客户端
func NewLLMClient(provider config.Provider) *LLMClient {
	cfg := openai.DefaultConfig(provider.Key)
	cfg.BaseURL = provider.Endpoint
	return &LLMClient{
		client: openai.NewClientWithConfig(cfg),
		model:  provider.Model,
	}
}

// ChatStream 发起一次流式对话：reason/content 增量通过 emit 推出，
// 返回本轮完整的 assistant 消息（含 tool_calls）。ctx 取消时也会退出。
func (c *LLMClient) ChatStream(
	ctx context.Context,
	msgs []domain.Message,
	tools []ToolSpec,
	emit func(domain.Event) error,
) (*domain.Message, error) {
	req := openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: toOpenAiMessage(msgs),
		Tools:    toOpenAiTools(tools),
	}
	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	var reason, content string
	toolsMap := make(map[string]domain.ToolCall)
	// 流式循环
	for {
		recv, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		c := recv.Choices
		if len(c) == 0 {
			continue
		}
		c0D := c[0].Delta
		if c0D.ReasoningContent != "" {
			emit(buildEvent(domain.EventReason, c0D.ReasoningContent))
			reason += c0D.ReasoningContent
		}
		if c0D.Content != "" {
			emit(buildEvent(domain.EventContent, c0D.Content))
			content += c0D.Content
		}
		// Tools Call 解析
		if len(c0D.ToolCalls) != 0 {
			receiveToolsCall(&toolsMap, c0D.ToolCalls)
		}
		if c[0].FinishReason == "stop" {
			emit(buildEvent(domain.EventDone, ""))
		}
	}
	msg := domain.Message{
		Role:      domain.RoleAssistant,
		Content:   content,
		Reason:    reason,
		ToolCalls: collectToolCalls(toolsMap),
	}
	return &msg, nil
}

// collectToolCalls 按流式 index 顺序取出拼接完成的工具调用。
func collectToolCalls(toolsMap map[string]domain.ToolCall) []domain.ToolCall {
	if len(toolsMap) == 0 {
		return nil
	}
	keys := make([]string, 0, len(toolsMap))
	for k := range toolsMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return toolCallIndex(keys[i]) < toolCallIndex(keys[j])
	})
	out := make([]domain.ToolCall, 0, len(keys))
	for _, k := range keys {
		out = append(out, toolsMap[k])
	}
	return out
}

func toolCallIndex(key string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(key, "tool_call_"))
	if err != nil {
		return 1 << 30
	}
	return n
}

func receiveToolsCall(toolsMap *map[string]domain.ToolCall, toolCalls []openai.ToolCall) {
	for _, tc := range toolCalls {
		idx := 0
		if tc.Index != nil {
			idx = *tc.Index
		}
		mapKey := fmt.Sprintf("tool_call_%d", idx)
		existing, exists := (*toolsMap)[mapKey]
		// 初次接收
		if !exists {
			id := tc.ID
			if id == "" {
				id = mapKey
			}
			(*toolsMap)[mapKey] = domain.ToolCall{
				ID:   id,
				Name: tc.Function.Name,
				Args: tc.Function.Arguments,
			}
		} else {
			existing.Args += tc.Function.Arguments
			if tc.ID != "" && existing.ID == "" {
				existing.ID = tc.ID
			}
			if tc.Function.Name != "" && existing.Name == "" {
				existing.Name = tc.Function.Name
			}

			(*toolsMap)[mapKey] = existing // 写回 map
		}
	}
}

func buildEvent(eType domain.EventType, text string) domain.Event {
	return domain.Event{
		Type:     eType,
		Text:     text,
		Err:      nil,
		ToolCall: nil,
	}
}

func toOpenAiTools(tools []ToolSpec) []openai.Tool {
	out := make([]openai.Tool, 0, len(tools))
	for _, t := range tools {
		out = append(out, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return out
}

func toOpenAiMessage(msgs []domain.Message) []openai.ChatCompletionMessage {
	resp := make([]openai.ChatCompletionMessage, len(msgs))
	for i, msg := range msgs {
		out := openai.ChatCompletionMessage{
			Role:             string(msg.Role),
			Content:          msg.Content,
			ReasoningContent: msg.Reason,
			ToolCallID:       msg.ToolCallID,
		}
		if len(msg.ToolCalls) > 0 {
			out.ToolCalls = make([]openai.ToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				out.ToolCalls = append(out.ToolCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Args,
					},
				})
			}
		}
		resp[i] = out
	}
	return resp
}
