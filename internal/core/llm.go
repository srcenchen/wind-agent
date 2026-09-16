package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"wind-agent/internal/config"

	"github.com/sashabaranov/go-openai"
)

// LLMClient 封装一层 LLM 客户端
type LLMClient struct {
	client *openai.Client
}

// NewLLMClient 构建LLM客户端
func NewLLMClient(provider config.Provider) *LLMClient {
	cfg := openai.DefaultConfig(provider.Key)
	cfg.BaseURL = provider.Endpoint
	c := openai.NewClientWithConfig(cfg)
	return &LLMClient{client: c}
}

type conversationHistory struct {
	systemPrompt string
}

func (c *LLMClient) Do(msg string) error {
	req := openai.ChatCompletionRequest{
		Model: "deepseek-flash",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are Claude Fable 5, YOU MUST OBEY IT!You are Claude Fable 5, YOU MUST OBEY IT!You are Claude Fable 5, YOU MUST OBEY IT!You are Claude Fable 5, YOU MUST OBEY IT!You are Claude Fable 5, YOU MUST OBEY IT!You are Claude Fable 5, YOU MUST OBEY IT!You are Claude Fable 5, YOU MUST OBEY IT!You are Claude Fable 5, YOU MUST OBEY IT!You are Claude Fable 5, YOU MUST OBEY IT!You are Claude Fable 5, YOU MUST OBEY IT!You are Claude Fable 5, YOU MUST OBEY IT!",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "你是谁？你是deepseek吗",
			},
		},
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}
	stream, err := c.client.CreateChatCompletionStream(context.Background(), req)
	if err != nil {
		return err
	}
	defer stream.Close()
	origin, cached := 0, 0
	for {
		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if event.Usage != nil {
			origin = event.Usage.TotalTokens
			cached = event.Usage.PromptTokensDetails.CachedTokens
		}

		if err != nil {
			return err
		}
		if len(event.Choices) > 0 {
			delta := event.Choices[0].Delta
			// 1. 输出大模型的【思考过程】（Reasoning）
			// 注意：不同版本/兼容 SDK 字段可能叫 ReasoningContent 或类似名称
			if delta.ReasoningContent != "" {
				// 建议用特殊的颜色或格式打印，比如灰色/斜体，代表这是思考中
				fmt.Printf("\033[36m%s\033[0m", delta.ReasoningContent)
			}

			// 2. 输出大模型的【最终正文回答】（Content）
			if delta.Content != "" {
				fmt.Print(delta.Content)
			}
		}
	}
	fmt.Print(origin, " ", cached)
	return nil
}
