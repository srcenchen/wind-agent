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
	//chatG.GET("/sessions", s.listSessionsHandler)
	//chatG.POST("/sessions", s.createSessionHandler)
	//chatG.GET("/sessions/:sessionID", s.getSessionHandler)
	chatG.POST("", s.chatHandler)
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
		SessionId: req.SessionID,
		Provider:  req.Provider,
		Content:   req.Content,
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
	Messages  []domain.Message `json:"messages,omitempty"`
}

type sessionSummaryDTO struct {
	SessionID string `json:"session_id"`
	Provider  string `json:"provider"`
	UpdatedAt string `json:"updated_at"`
}
