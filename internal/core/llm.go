package core

import (
	"context"
	"errors"
	"io"
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
	}
	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, err
	}
	var reason, content string
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
			break
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
		if c[0].FinishReason == "stop" {
			emit(buildEvent(domain.EventDone, ""))
		}
	}
	msg := domain.Message{
		Role:    domain.RoleAssistant,
		Content: content,
		Reason:  reason,
	}
	return &msg, nil
}

func buildEvent(eType domain.EventType, text string) domain.Event {
	return domain.Event{
		Type:     eType,
		Text:     text,
		Err:      nil,
		ToolCall: nil,
	}
}

func toOpenAiMessage(msgs []domain.Message) []openai.ChatCompletionMessage {
	resp := make([]openai.ChatCompletionMessage, len(msgs))
	for i, msg := range msgs {
		resp[i] = openai.ChatCompletionMessage{
			Role:             string(msg.Role),
			Content:          msg.Content,
			ReasoningContent: msg.Reason,
		}
	}
	return resp
}
