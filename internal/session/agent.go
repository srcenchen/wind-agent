package session

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"sync"

	"wind-agent/internal/core"
	"wind-agent/internal/domain"
)

const systemPrompt = "你是一个Helpful AI助手。"

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
// 历史持久化在 repo；同一会话串行、不同会话互不阻塞。
type Agent struct {
	loop  *core.AgentLoop
	repo  domain.SessionRepo
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewAgent(llms *core.Registry, tools *core.ToolRegistry, repo domain.SessionRepo) *Agent {
	return &Agent{
		loop:  core.NewAgentLoop(llms, tools),
		repo:  repo,
		locks: make(map[string]*sync.Mutex),
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

// turn 处理一轮对话：同一会话串行，历史按 sessionID 从 repo 载入并落库。
func (a *Agent) turn(ctx context.Context, t Transport, in domain.Inbound) error {
	lock := a.sessionLock(in.SessionId)
	lock.Lock()
	defer lock.Unlock()

	content := userContent(in)
	log.Printf("session %s inbound: %s", in.SessionId, content)

	provider, err := a.ensureSession(ctx, in)
	if err != nil {
		return err
	}

	prior, err := a.repo.Messages(ctx, in.SessionId)
	if err != nil {
		return err
	}

	sys := systemPrompt
	if in.SystemPrompt != "" {
		sys += "\n\n" + in.SystemPrompt
	}
	hist := make([]domain.Message, 0, len(prior)+2)
	hist = append(hist, domain.Message{Role: domain.RoleSystem, Content: sys})
	hist = append(hist, prior...)
	hist = append(hist, domain.Message{Role: domain.RoleUser, Content: content})

	emit := func(ev domain.Event) error {
		return t.Emit(ctx, ev)
	}
	// Run 返回的切片已包含传入的 hist，直接覆盖，不能再 append。
	out, err := a.loop.Run(ctx, provider, hist, emit)
	// hist 含 system 与已有历史，只有其后的才是本轮新增消息；system 不落库。
	if len(out) > len(hist) {
		if perr := a.repo.Append(ctx, in.SessionId, out[len(hist):]); perr != nil {
			log.Printf("session %s persist failed: %v", in.SessionId, perr)
		}
	}
	if err != nil {
		log.Printf("session %s turn failed: %v", in.SessionId, err)
		_ = t.Emit(ctx, domain.Event{Type: domain.EventError, Err: err})
	}
	return nil
}

// ensureSession 取会话默认 provider，不存在则按入站信息创建。
func (a *Agent) ensureSession(ctx context.Context, in domain.Inbound) (string, error) {
	s, ok, err := a.repo.Get(ctx, in.SessionId)
	if err != nil {
		return "", err
	}
	if !ok {
		s = domain.Session{SessionID: in.SessionId, Provider: in.Provider, Title: defaultTitle(in.Content)}
		if err := a.repo.Create(ctx, s); err != nil {
			return "", err
		}
	}
	if in.Provider != "" {
		_ = a.repo.Touch(ctx, in.SessionId, in.Provider)
		return in.Provider, nil
	}
	return s.Provider, nil
}

func defaultTitle(content string) string {
	r := []rune(content)
	if len(r) > 20 {
		return string(r[:20])
	}
	return string(r)
}

// userContent 把共享会话（如群聊）里的发送者与被 @ 的人拼进消息，供模型感知。
func userContent(in domain.Inbound) string {
	content := in.Content
	if in.Speaker != "" {
		content = speakerLabel(in.Speaker, in.SpeakerName) + " 说：" + content
	}
	if len(in.Mentions) > 0 {
		parts := make([]string, 0, len(in.Mentions))
		for _, m := range in.Mentions {
			parts = append(parts, mentionTag(m.ID, m.Name))
		}
		content += "\n[本轮@: " + strings.Join(parts, ", ") + "]"
	}
	return content
}

// speakerLabel 渲染发送者标记：[id] 或 [id]（昵称）。
func speakerLabel(id, name string) string {
	if name == "" {
		return "[" + id + "]"
	}
	return "[" + id + "]（" + name + "）"
}

// mentionTag 渲染被 @ 用户：id 或 id（昵称）。
func mentionTag(id, name string) string {
	if name == "" {
		return id
	}
	return id + "（" + name + "）"
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
