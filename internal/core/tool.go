package core

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool 一个可被模型调用的工具。
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage // 参数的 JSON Schema
	Execute(ctx context.Context, args string) (string, error)
}

// ToolSpec 给模型看的工具描述。
type ToolSpec struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// ToolRegistry 工具表：name -> Tool。
type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry(tools ...Tool) *ToolRegistry {
	m := make(map[string]Tool, len(tools))
	for _, t := range tools {
		m[t.Name()] = t
	}
	return &ToolRegistry{tools: m}
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) Specs() []ToolSpec {
	out := make([]ToolSpec, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, ToolSpec{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return out
}

func (r *ToolRegistry) Execute(ctx context.Context, name, args string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return t.Execute(ctx, args)
}
