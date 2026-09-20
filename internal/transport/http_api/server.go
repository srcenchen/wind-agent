package http_api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"
	"wind-agent/internal/domain"
	"wind-agent/internal/service"

	"github.com/gin-gonic/gin"
)

// http_api 传输协议

// webSystemPrompt Web 聊天框不解析 markdown，要求模型输出纯文本。
const webSystemPrompt = "输出纯文本，不要使用 markdown（聊天框不解析 markdown）。"

type Server struct {
	svc  *service.SessionService
	addr string
}

func NewServer(svc *service.SessionService, addr string) *Server {
	return &Server{svc: svc, addr: addr}
}

func (s *Server) Run(ctx context.Context) error {
	// 启动 http 服务器
	r := gin.New()
	r.Use(gin.Recovery())
	srv := &http.Server{
		Addr:    s.addr,
		Handler: r,
	}
	chatG := r.Group("/chat")
	chatG.POST("", s.chatHandler)

	sessG := r.Group("/sessions")
	sessG.GET("", s.listSessionsHandler)
	sessG.POST("", s.createSessionHandler)
	sessG.GET("/:sessionID", s.getSessionHandler)
	sessG.DELETE("/:sessionID", s.deleteSessionHandler)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	log.Printf("http server listening on %s", s.addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

type createSessionRequest struct {
	Provider string `json:"provider"`
	Title    string `json:"title"`
}

type chatRequest struct {
	SessionID string `json:"session_id"`
	Provider  string `json:"provider"`
	Content   string `json:"content"`
}

func (s *Server) chatHandler(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.SessionID == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id and content are required"})
		return
	}

	in := domain.Inbound{
		SessionId:    req.SessionID,
		Provider:     req.Provider,
		Content:      req.Content,
		SystemPrompt: webSystemPrompt,
	}
	t := newSSETransport(c, in)
	defer t.Close()

	if err := s.svc.Chat(c.Request.Context(), in, t); err != nil {
		log.Printf("chat ended: %v", err)
	}
}

type sessionDTO struct {
	SessionID string           `json:"session_id"`
	Provider  string           `json:"provider"`
	Title     string           `json:"title"`
	CreatedAt string           `json:"created_at"`
	UpdatedAt string           `json:"updated_at"`
	Messages  []domain.Message `json:"messages,omitempty"`
}

type sessionSummaryDTO struct {
	SessionID string `json:"session_id"`
	Provider  string `json:"provider"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Server) createSessionHandler(c *gin.Context) {
	var req createSessionRequest
	_ = c.ShouldBindJSON(&req)
	sess, err := s.svc.CreateSession(c.Request.Context(), req.Provider, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toSessionDTO(sess, nil))
}

func (s *Server) listSessionsHandler(c *gin.Context) {
	list, err := s.svc.ListSessions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]sessionSummaryDTO, 0, len(list))
	for _, item := range list {
		out = append(out, sessionSummaryDTO{
			SessionID: item.SessionID,
			Provider:  item.Provider,
			Title:     item.Title,
			CreatedAt: item.CreatedAt.Format(time.RFC3339),
			UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"sessions": out})
}

func (s *Server) getSessionHandler(c *gin.Context) {
	sessionID := c.Param("sessionID")
	sess, msgs, ok, err := s.svc.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	c.JSON(http.StatusOK, toSessionDTO(sess, msgs))
}

func (s *Server) deleteSessionHandler(c *gin.Context) {
	sessionID := c.Param("sessionID")
	if err := s.svc.DeleteSession(c.Request.Context(), sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": sessionID})
}

func toSessionDTO(sess domain.Session, msgs []domain.Message) sessionDTO {
	return sessionDTO{
		SessionID: sess.SessionID,
		Provider:  sess.Provider,
		Title:     sess.Title,
		CreatedAt: sess.CreatedAt.Format(time.RFC3339),
		UpdatedAt: sess.UpdatedAt.Format(time.RFC3339),
		Messages:  msgs,
	}
}
