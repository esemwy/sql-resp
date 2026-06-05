package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitlab.smy.com/work/sql-resp/internal/resp"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

var startTime = time.Now()

func init() {
	Register(Cmd{Name: "INFO", MinArgs: 1, MaxArgs: 2, Exec: withOptionalStore(cmdInfo)})
}

func withOptionalStore(h storeHandler) Handler {
	return func(ctx context.Context, cc *ClientConn, args []resp.Value) resp.Value {
		s := storeFromCtx(ctx) // may be nil
		return h(ctx, s, cc.DB, cc, args)
	}
}

func cmdInfo(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	section := ""
	if len(args) >= 2 {
		section = strings.ToLower(args[1].Str())
	}

	var sb strings.Builder

	if section == "" || section == "server" {
		sb.WriteString("# Server\r\n")
		sb.WriteString("redis_version:7.0.0-sql-resp\r\n")
		sb.WriteString(fmt.Sprintf("uptime_in_seconds:%d\r\n", int(time.Since(startTime).Seconds())))
		sb.WriteString("tcp_port:6379\r\n")
		sb.WriteString("\r\n")
	}

	if section == "" || section == "clients" {
		sb.WriteString("# Clients\r\n")
		sb.WriteString("connected_clients:1\r\n")
		sb.WriteString("\r\n")
	}

	if section == "" || section == "memory" {
		sb.WriteString("# Memory\r\n")
		sb.WriteString("used_memory:0\r\n")
		sb.WriteString("used_memory_human:0B\r\n")
		sb.WriteString("\r\n")
	}

	if section == "" || section == "replication" {
		sb.WriteString("# Replication\r\n")
		sb.WriteString("role:master\r\n")
		sb.WriteString("connected_slaves:0\r\n")
		sb.WriteString("\r\n")
	}

	if (section == "" || section == "keyspace") && s != nil {
		sb.WriteString("# Keyspace\r\n")
		for db := 0; db < 16; db++ {
			n, err := s.DBSize(ctx, db)
			if err == nil && n > 0 {
				sb.WriteString(fmt.Sprintf("db%d:keys=%d,expires=0\r\n", db, n))
			}
		}
		sb.WriteString("\r\n")
	}

	return resp.BulkString(sb.String())
}
