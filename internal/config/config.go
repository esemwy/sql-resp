// Package config loads server configuration from a redis.conf-style file,
// CLI flags, and environment variables.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all server configuration.
type Config struct {
	Port        int
	Password    string // requirepass
	TLSCert     string // tls-cert-file
	TLSKey      string // tls-key-file
	Databases   int    // number of databases (default 16)
	Hz          int    // background sweep interval in ms (default 100)
	InMemory    bool   // true when save "" or --no-persist
	Backend     string // "sqlite" or "postgres"
	DSN         string // database DSN
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		Port:      6379,
		Databases: 16,
		Hz:        100,
		Backend:   "sqlite",
		DSN:       "sql-resp.db",
	}
}

// Load returns a Config by merging defaults, a config file, and flags.
// filePath may be empty to skip file loading.
// flags is a map of key→value from parsed CLI flags (may be nil).
func Load(filePath string, flags map[string]string) (Config, error) {
	cfg := Default()

	if filePath != "" {
		if err := cfg.loadFile(filePath); err != nil {
			return cfg, err
		}
	}

	// Environment variable overrides for secrets.
	if v := os.Getenv("SQL_RESP_PASSWORD"); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv("SQL_RESP_DSN"); v != "" {
		cfg.DSN = v
	}

	// CLI flag overrides.
	for k, v := range flags {
		if err := cfg.set(k, v); err != nil {
			return cfg, fmt.Errorf("flag --%s: %w", k, err)
		}
	}

	return cfg, nil
}

func (c *Config) loadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		key := strings.ToLower(parts[0])
		val := ""
		if len(parts) == 2 {
			val = strings.TrimSpace(parts[1])
		}
		if err := c.set(key, val); err != nil {
			return fmt.Errorf("config %q: %w", line, err)
		}
	}
	return sc.Err()
}

func (c *Config) set(key, val string) error {
	switch key {
	case "port":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid port %q", val)
		}
		c.Port = n
	case "requirepass":
		c.Password = val
	case "tls-cert-file":
		c.TLSCert = val
	case "tls-key-file":
		c.TLSKey = val
	case "databases":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid databases %q", val)
		}
		c.Databases = n
	case "hz":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid hz %q", val)
		}
		c.Hz = n
	case "save":
		c.InMemory = (val == "" || val == `""`)
	case "no-persist":
		c.InMemory = true
	case "backend":
		if val != "sqlite" && val != "postgres" {
			return fmt.Errorf("unknown backend %q (want sqlite or postgres)", val)
		}
		c.Backend = val
	case "dsn":
		c.DSN = val
	default:
		// Silently ignore unknown directives for forward-compatibility.
	}
	return nil
}
