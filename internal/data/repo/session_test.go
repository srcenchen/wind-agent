package repo_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"wind-agent/internal/data/dao"
	"wind-agent/internal/data/model"
	"wind-agent/internal/data/repo"
	"wind-agent/internal/domain"
)

func newRepo(t *testing.T) *repo.SessionRepo {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Session{}, &model.SessionMessage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repo.NewSessionRepo(dao.NewSessionDao(db))
}

func TestSessionPersistence(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)

	if err := r.Create(ctx, domain.Session{SessionID: "web:1", Provider: "deepseek", Title: "t1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, ok, err := r.Get(ctx, "web:1")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.Title != "t1" || got.Provider != "deepseek" {
		t.Fatalf("unexpected session: %+v", got)
	}

	turn1 := []domain.Message{
		{Role: domain.RoleUser, Content: "hi"},
		{Role: domain.RoleAssistant, Content: "hello", Reason: "r",
			ToolCalls: []domain.ToolCall{{ID: "call_1", Name: "weather", Args: "{}"}}},
		{Role: domain.RoleTool, ToolCallID: "call_1", Content: "sunny"},
	}
	if err := r.Append(ctx, "web:1", turn1); err != nil {
		t.Fatalf("append turn1: %v", err)
	}
	turn2 := []domain.Message{
		{Role: domain.RoleUser, Content: "bye"},
		{Role: domain.RoleAssistant, Content: "cya"},
	}
	if err := r.Append(ctx, "web:1", turn2); err != nil {
		t.Fatalf("append turn2: %v", err)
	}

	msgs, err := r.Messages(ctx, "web:1")
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	if len(msgs) != 5 {
		t.Fatalf("want 5 messages, got %d", len(msgs))
	}
	if msgs[0].Role != domain.RoleUser || msgs[0].Content != "hi" {
		t.Fatalf("unexpected first message: %+v", msgs[0])
	}
	if len(msgs[1].ToolCalls) != 1 || msgs[1].ToolCalls[0].Name != "weather" {
		t.Fatalf("tool_calls not round-tripped: %+v", msgs[1].ToolCalls)
	}
	if msgs[2].ToolCallID != "call_1" || msgs[2].Content != "sunny" {
		t.Fatalf("unexpected tool message: %+v", msgs[2])
	}

	list, err := r.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].SessionID != "web:1" {
		t.Fatalf("unexpected list: %+v", list)
	}

	if err := r.Delete(ctx, "web:1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok, _ := r.Get(ctx, "web:1"); ok {
		t.Fatalf("session still exists after delete")
	}
	left, _ := r.Messages(ctx, "web:1")
	if len(left) != 0 {
		t.Fatalf("messages not cascade-deleted: %d", len(left))
	}
}
