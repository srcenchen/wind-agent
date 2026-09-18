package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WebSearch 通过外部 HTTP 搜索服务检索互联网。
// Endpoint 为空时走占位桩，便于后续接入真实后端。
type WebSearch struct {
	Endpoint string
	Client   *http.Client
}

type webSearchArgs struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

func (w WebSearch) Name() string {
	return "web_search"
}

func (w WebSearch) Description() string {
	return "搜索互联网，返回与关键词相关的网页摘要。适合查询实时信息、新闻或未知知识。"
}

func (w WebSearch) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "搜索关键词"
			},
			"max_results": {
				"type": "integer",
				"description": "返回结果条数，默认 5",
				"minimum": 1,
				"maximum": 20
			}
		},
		"required": ["query"]
	}`)
}

func (w WebSearch) Execute(ctx context.Context, args string) (string, error) {
	var in webSearchArgs
	if err := json.Unmarshal([]byte(args), &in); err != nil {
		return "", fmt.Errorf("web_search: 参数解析失败: %w", err)
	}
	if in.Query == "" {
		return "", fmt.Errorf("web_search: query 不能为空")
	}
	if in.MaxResults <= 0 {
		in.MaxResults = 5
	}

	if w.Endpoint == "" {
		return fmt.Sprintf("[web_search 占位桩] 已收到查询 %q（max_results=%d），尚未接入真实搜索后端。", in.Query, in.MaxResults), nil
	}

	u, err := url.Parse(w.Endpoint)
	if err != nil {
		return "", fmt.Errorf("web_search: endpoint 非法: %w", err)
	}
	q := u.Query()
	q.Set("q", in.Query)
	q.Set("max_results", fmt.Sprintf("%d", in.MaxResults))
	u.RawQuery = q.Encode()

	client := w.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_search: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("web_search: 读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("web_search: 搜索服务返回 %d: %s", resp.StatusCode, string(body))
	}
	return string(body), nil
}
