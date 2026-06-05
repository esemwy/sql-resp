package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConf(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "test.conf")
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDefaults(t *testing.T) {
	cfg, err := Load("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 6379 {
		t.Errorf("port: got %d", cfg.Port)
	}
	if cfg.Databases != 16 {
		t.Errorf("databases: got %d", cfg.Databases)
	}
	if cfg.Hz != 100 {
		t.Errorf("hz: got %d", cfg.Hz)
	}
	if cfg.Backend != "sqlite" {
		t.Errorf("backend: got %s", cfg.Backend)
	}
	if cfg.InMemory {
		t.Error("in-memory should be false by default")
	}
}

func TestFileLoading(t *testing.T) {
	conf := writeConf(t, `
# comment
port 7379
requirepass secret
databases 8
hz 50
backend sqlite
dsn /tmp/test.db
`)
	cfg, err := Load(conf, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 7379 {
		t.Errorf("port: got %d", cfg.Port)
	}
	if cfg.Password != "secret" {
		t.Errorf("password: got %q", cfg.Password)
	}
	if cfg.Databases != 8 {
		t.Errorf("databases: got %d", cfg.Databases)
	}
	if cfg.Hz != 50 {
		t.Errorf("hz: got %d", cfg.Hz)
	}
	if cfg.DSN != "/tmp/test.db" {
		t.Errorf("dsn: got %q", cfg.DSN)
	}
}

func TestInMemoryViaSave(t *testing.T) {
	conf := writeConf(t, `save ""`)
	cfg, err := Load(conf, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.InMemory {
		t.Error("expected in-memory mode")
	}
}

func TestFlagOverride(t *testing.T) {
	conf := writeConf(t, "port 7379\nrequirepass file-pass")
	cfg, err := Load(conf, map[string]string{
		"port":        "8888",
		"requirepass": "flag-pass",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8888 {
		t.Errorf("port: got %d", cfg.Port)
	}
	if cfg.Password != "flag-pass" {
		t.Errorf("password: got %q", cfg.Password)
	}
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("SQL_RESP_PASSWORD", "env-pass")
	t.Setenv("SQL_RESP_DSN", "env-dsn")
	cfg, err := Load("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Password != "env-pass" {
		t.Errorf("password: got %q", cfg.Password)
	}
	if cfg.DSN != "env-dsn" {
		t.Errorf("dsn: got %q", cfg.DSN)
	}
}

func TestNoPersistFlag(t *testing.T) {
	cfg, err := Load("", map[string]string{"no-persist": ""})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.InMemory {
		t.Error("expected in-memory mode from --no-persist flag")
	}
}

func TestUnknownBackend(t *testing.T) {
	_, err := Load("", map[string]string{"backend": "mysql"})
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/to.conf", nil)
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}
