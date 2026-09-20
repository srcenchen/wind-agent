package qqbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultAPI = "https://api.bot.qq.com"

type ClientConfig struct {
	AppID  string
	Secret string
	API    string
}

type Client struct {
	appID  string
	secret string
	api    string
	http   *http.Client

	mu      sync.Mutex
	token   string
	tokenAt time.Time
	ttl     time.Duration
}

func NewClient(cfg ClientConfig) *Client {
	api := strings.TrimRight(cfg.API, "/")
	if api == "" {
		api = defaultAPI
	}
	return &Client{
		appID:  cfg.AppID,
		secret: cfg.Secret,
		api:    api,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) GatewayURL(ctx context.Context) (string, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.api+"/gateway", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("X-Union-Appid", c.appID)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("qq gateway: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.URL == "" {
		return "", fmt.Errorf("qq gateway: empty url")
	}
	return out.URL, nil
}

func (c *Client) AccessToken(ctx context.Context) (string, error) {
	return c.accessToken(ctx)
}

func (c *Client) ReplyC2C(ctx context.Context, userOpenID, msgID, content string) error {
	return c.postMessage(ctx, "/v2/users/"+userOpenID+"/messages", msgID, content)
}

func (c *Client) ReplyGroup(ctx context.Context, groupOpenID, msgID, content string) error {
	return c.postMessage(ctx, "/v2/groups/"+groupOpenID+"/messages", msgID, content)
}

func (c *Client) postMessage(ctx context.Context, path, msgID, content string) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{
		"msg_type": 2,
		"markdown": map[string]any{"content": content},
		"msg_id":   msgID,
		"msg_seq":  1,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.api+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-Union-Appid", c.appID)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("qq send %s: HTTP %d %s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Since(c.tokenAt) < c.ttl {
		return c.token, nil
	}
	body, err := json.Marshal(map[string]string{
		"appId":        c.appID,
		"clientSecret": c.secret,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.api+"/app/getAppAccessToken", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("qq token: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out struct {
		AccessToken string          `json:"access_token"`
		ExpiresIn   json.RawMessage `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("qq token: empty access_token")
	}
	sec := int64(7200)
	if len(out.ExpiresIn) > 0 {
		if n, err := strconv.ParseInt(strings.Trim(string(out.ExpiresIn), `"`), 10, 64); err == nil && n > 0 {
			sec = n
		}
	}
	ttl := time.Duration(sec) * time.Second
	if ttl > time.Minute {
		ttl -= time.Minute
	}
	c.token = out.AccessToken
	c.tokenAt = time.Now()
	c.ttl = ttl
	return c.token, nil
}
