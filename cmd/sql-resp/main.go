package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitlab.smy.com/work/sql-resp/internal/config"
	"gitlab.smy.com/work/sql-resp/internal/db"
	"gitlab.smy.com/work/sql-resp/internal/server"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

var version = "dev"

func main() {
	var (
		configFile = flag.String("config", "", "path to redis.conf-style config file")
		port       = flag.String("port", "", "port to listen on")
		noPersist  = flag.Bool("no-persist", false, "use in-memory SQLite (data lost on restart)")
		showVer    = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("sql-resp %s\n", version)
		os.Exit(0)
	}

	flags := map[string]string{}
	if *port != "" {
		flags["port"] = *port
	}
	if *noPersist {
		flags["no-persist"] = ""
	}

	cfg, err := config.Load(*configFile, flags)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	dsn := cfg.DSN
	if cfg.InMemory {
		dsn = ":memory:"
	}

	var backend db.DB
	switch cfg.Backend {
	case "sqlite", "":
		backend, err = db.OpenSQLite(dsn)
	case "postgres":
		backend, err = db.OpenPostgres(dsn)
	default:
		log.Fatalf("unsupported backend %q", cfg.Backend)
	}
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer backend.Close()

	if err := backend.Migrate(context.Background()); err != nil {
		log.Fatalf("db migrate: %v", err)
	}

	st := store.New(backend)

	// Background TTL sweep.
	sweepInterval := time.Duration(cfg.Hz) * time.Millisecond
	go func() {
		ticker := time.NewTicker(sweepInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := st.SweepExpired(context.Background()); err != nil {
				log.Printf("sweep error: %v", err)
			}
		}
	}()

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := server.New(server.Config{
		Addr:     addr,
		Password: cfg.Password,
		Store:    st,
	})

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down")
		srv.Close()
	}()

	if err := srv.ListenAndServe(); err != nil {
		log.Printf("server: %v", err)
	}
}
