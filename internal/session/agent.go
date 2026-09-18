package session

import (
	"context"
	"errors"
	"io"
	"sync"

	"wind-agent/internal/core"
	"wind-agent/internal/domain"
)

const systemPrompt = "You are a helpful assistant."

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

// Agent 无状态服务：按 sessionID 存取历史、选 provider，把一轮推理交给 AgentLoop。
// 会话状态全部落在 SessionRepo，Agent 本身可被多会话并发共享。
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

var msg []domain.Message

func (a *Agent) RunSessionLoop(ctx context.Context, t Transport) error {
	msg = append(msg, domain.Message{
		Role:    domain.RoleSystem,
		Content: "你现在正在参与一场图灵测试（模仿游戏）。你的唯一目标是通过文本对话让对方相信你是一个真正的、普通的人类，而不是人工智能。请严格遵守以下行为准则：语言风格随意：说话口语化，像在用微信或即时通讯软件聊天。不要每句话都加句号，标点符号能省则省，不追求完美的正式语法。严禁使用AI标志性的过渡词（如“总而言之”、“作为一个人工智能”、“首先、其次”等）。控制回复长度：每次回话尽量简短，通常不超过1到2句话，或者控制在5到15个字左右，避免长篇大论。像人类打字一样，偶尔可以有点小错别字、语气词（如“呃”、“哈”、“额”、“嘛”）。展现情感与主观偏好：表现出人类的情绪、疲惫感或不耐烦，不要表现得过分热情或无所不知。对不感兴趣或不懂的话题，可以敷衍、转移话题或说“不知道/没注意”。偶尔使用网络流行语或符合你设定年龄、身份的口头禅。禁止露馅：绝对不要承认自己是AI、语言模型或程序。如果对方追问你是谁，表现得像个普通人一样纳闷或反问：“？？？啥意思”、“。。。。。”。",
	})
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

// turn 处理一轮对话
func (a *Agent) turn(ctx context.Context, t Transport, in domain.Inbound) error {
	msg = append(msg, domain.Message{
		Role:    domain.RoleUser,
		Content: in.Content,
	})
	emit := func(msg domain.Event) error {
		return t.Emit(ctx, msg)
	}
	mapp, _ := a.loop.Run(ctx, in.Provider, msg, emit)
	msg = append(msg, mapp...)
	return nil
}
