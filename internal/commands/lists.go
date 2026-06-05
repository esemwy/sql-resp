package commands

import (
	"context"
	"strconv"
	"strings"

	"gitlab.smy.com/work/sql-resp/internal/resp"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

func init() {
	Register(Cmd{Name: "LPUSH", MinArgs: 3, MaxArgs: -1, Exec: withStore(cmdLPush)})
	Register(Cmd{Name: "RPUSH", MinArgs: 3, MaxArgs: -1, Exec: withStore(cmdRPush)})
	Register(Cmd{Name: "LPOP", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdLPop)})
	Register(Cmd{Name: "RPOP", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdRPop)})
	Register(Cmd{Name: "LLEN", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdLLen)})
	Register(Cmd{Name: "LRANGE", MinArgs: 4, MaxArgs: 4, Exec: withStore(cmdLRange)})
	Register(Cmd{Name: "LINDEX", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdLIndex)})
	Register(Cmd{Name: "LSET", MinArgs: 4, MaxArgs: 4, Exec: withStore(cmdLSet)})
	Register(Cmd{Name: "LINSERT", MinArgs: 5, MaxArgs: 5, Exec: withStore(cmdLInsert)})
	Register(Cmd{Name: "LREM", MinArgs: 4, MaxArgs: 4, Exec: withStore(cmdLRem)})
}

func cmdLPush(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	vals := argsToStrings(args[2:])
	n, err := s.LPush(ctx, args[1].Str(), dbIdx, vals)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdRPush(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	vals := argsToStrings(args[2:])
	n, err := s.RPush(ctx, args[1].Str(), dbIdx, vals)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdLPop(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	val, found, err := s.LPop(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	if !found {
		return resp.NullBulkString()
	}
	return resp.BulkString(val)
}

func cmdRPop(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	val, found, err := s.RPop(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	if !found {
		return resp.NullBulkString()
	}
	return resp.BulkString(val)
}

func cmdLLen(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.LLen(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdLRange(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	start, err1 := strconv.ParseInt(args[2].Str(), 10, 64)
	stop, err2 := strconv.ParseInt(args[3].Str(), 10, 64)
	if err1 != nil || err2 != nil {
		return resp.Error("ERR value is not an integer or out of range")
	}
	vals, err := s.LRange(ctx, args[1].Str(), dbIdx, start, stop)
	if err != nil {
		return storeErr(err)
	}
	return strSlice(vals)
}

func cmdLIndex(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	idx, err := strconv.ParseInt(args[2].Str(), 10, 64)
	if err != nil {
		return resp.Error("ERR value is not an integer or out of range")
	}
	val, found, err := s.LIndex(ctx, args[1].Str(), dbIdx, idx)
	if err != nil {
		return storeErr(err)
	}
	if !found {
		return resp.NullBulkString()
	}
	return resp.BulkString(val)
}

func cmdLSet(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	idx, err := strconv.ParseInt(args[2].Str(), 10, 64)
	if err != nil {
		return resp.Error("ERR value is not an integer or out of range")
	}
	if err := s.LSet(ctx, args[1].Str(), dbIdx, idx, args[3].Str()); err != nil {
		return storeErr(err)
	}
	return resp.SimpleString("OK")
}

func cmdLInsert(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	before := strings.ToUpper(args[2].Str()) == "BEFORE"
	if !before && strings.ToUpper(args[2].Str()) != "AFTER" {
		return resp.Error("ERR syntax error")
	}
	n, err := s.LInsert(ctx, args[1].Str(), dbIdx, before, args[3].Str(), args[4].Str())
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdLRem(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	count, err := strconv.ParseInt(args[2].Str(), 10, 64)
	if err != nil {
		return resp.Error("ERR value is not an integer or out of range")
	}
	n, err := s.LRem(ctx, args[1].Str(), dbIdx, count, args[3].Str())
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func argsToStrings(args []resp.Value) []string {
	s := make([]string, len(args))
	for i, a := range args {
		s[i] = a.Str()
	}
	return s
}
