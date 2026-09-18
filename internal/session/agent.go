package session

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"

	"wind-agent/internal/core"
	"wind-agent/internal/domain"
)

const systemPrompt = "你是一个Helpful AI助手，你所输出的内容，你应该只能输出聊天框的文本，由于聊天框没有markdown解析，所以不可以使用markdown"

type Emitter interface {
	Emit(ctx context.Context, event domain.Event) error
}

type Receiver interface {
	Receive(ctx context.Context) (domain.Inbound, error)
}

type Transport interface {
	Emitter
	Receiver
	Close() error
}

// Agent 按 sessionID 维护对话历史、选 provider，把一轮推理交给 AgentLoop。
// 历史暂存在内存（repo 就绪后替换为持久化）；同一会话串行、不同会话互不阻塞。
type Agent struct {
	loop  *core.AgentLoop
	repo  domain.SessionRepo
	mu    sync.Mutex
	locks map[string]*sync.Mutex
	hist  map[string][]domain.Message
}

func NewAgent(llms *core.Registry, tools *core.ToolRegistry, repo domain.SessionRepo) *Agent {
	return &Agent{
		loop:  core.NewAgentLoop(llms, tools),
		repo:  repo,
		locks: make(map[string]*sync.Mutex),
		hist:  make(map[string][]domain.Message),
	}
}

func (a *Agent) RunSessionLoop(ctx context.Context, t Transport) error {
	for {
		in, err := t.Receive(ctx)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := a.turn(ctx, t, in); err != nil {
			return err
		}
	}
}

// turn 处理一轮对话：同一会话串行，历史按 sessionID 隔离。
func (a *Agent) turn(ctx context.Context, t Transport, in domain.Inbound) error {
	lock := a.sessionLock(in.SessionId)
	lock.Lock()
	defer lock.Unlock()

	hist := a.loadHistory(in.SessionId)
	if len(hist) == 0 {
		hist = append(hist, domain.Message{Role: domain.RoleSystem, Content: systemPrompt})
	}
	hist = append(hist, domain.Message{Role: domain.RoleUser, Content: in.Content})

	emit := func(ev domain.Event) error {
		return t.Emit(ctx, ev)
	}
	// Run 返回的切片已包含传入的 hist，直接覆盖，不能再 append。
	out, err := a.loop.Run(ctx, in.Provider, hist, emit)
	a.saveHistory(in.SessionId, out)
	if err != nil {
		log.Printf("session %s turn failed: %v", in.SessionId, err)
		_ = t.Emit(ctx, domain.Event{Type: domain.EventError, Err: err})
	}
	return nil
}

func (a *Agent) sessionLock(id string) *sync.Mutex {
	a.mu.Lock()
	defer a.mu.Unlock()
	l, ok := a.locks[id]
	if !ok {
		l = &sync.Mutex{}
		a.locks[id] = l
	}
	return l
}

func (a *Agent) loadHistory(id string) []domain.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.hist[id]
}

func (a *Agent) saveHistory(id string, hist []domain.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.hist[id] = hist
}
