package core

import "wind-agent/internal/config"

// Registry 多 provider 注册表：id -> client，启动时建好、之后只读。
type Registry struct {
	clients map[string]*LLMClient
	def     string
}

func NewRegistry(providers []config.Provider) *Registry {
	m := make(map[string]*LLMClient, len(providers))
	for _, p := range providers {
		m[p.Id] = NewLLMClient(p)
	}
	def := ""
	if len(providers) > 0 {
		def = providers[0].Id
	}
	return &Registry{clients: m, def: def}
}

// Get 按 id 取 LLM；id 为空时取默认 provider。
func (r *Registry) Get(id string) (LLM, bool) {
	if id == "" {
		id = r.def
	}
	c, ok := r.clients[id]
	if !ok {
		return nil, false
	}
	return c, true
}

func (r *Registry) Default() string {
	return r.def
}
