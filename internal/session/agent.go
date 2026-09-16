package session

import (
	"context"
	"wind-agent/internal/config"
	"wind-agent/internal/core"
	"wind-agent/internal/message"
)

type Emitter interface {
	Emit(ctx context.Context, event message.Event) error
}

type Receiver interface {
	Receive(ctx context.Context) (message.Inbound, error)
}

type Transport interface {
	Emitter
	Receiver
	Close() error
}

type Agent struct {
	provider  config.Provider
	llmClient *core.LLMClient
}

func NewAgent(provider config.Provider) *Agent {
	llmC := core.NewLLMClient(provider)
	return &Agent{
		provider:  provider,
		llmClient: llmC,
	}
}

func (a *Agent) RunSessionLoop(ctx context.Context, t Transport) error {
	for {
		in, err := t.Receive(ctx)
		if err != nil {
			return err
		}
		go func() {
			a.llmClient.Do(in.Content)
		}()
	}
}
