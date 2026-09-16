package http_api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"
	"wind-agent/internal/session"

	"github.com/gin-gonic/gin"
)

// http_api 传输协议

type Server struct {
	ag   *session.Agent
	addr *string
}

func NewServer(ag *session.Agent, addr *string) *Server {
	return &Server{ag: ag, addr: addr}
}

func (s *Server) Run(ctx context.Context) error {
	// 启动 http 服务器
	r := gin.New()
	r.Use(gin.Recovery())
	srv := &http.Server{
		Addr:    *s.addr,
		Handler: r,
	}
	r.GET("/chat", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	log.Printf("http server listening on %s", *s.addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// chat
