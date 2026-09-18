package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadQQTransport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := []byte(`
app:
  transports:
    qq:
      enable: true
      app_id: "1903697237"
      secret: s3cret
`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Transports.QQ.Enable {
		t.Fatal("qq not enabled")
	}
	if cfg.Transports.QQ.AppID != "1903697237" {
		t.Fatalf("app_id=%s", cfg.Transports.QQ.AppID)
	}
	if cfg.Transports.QQ.Secret != "s3cret" {
		t.Fatalf("secret=%s", cfg.Transports.QQ.Secret)
	}
}
