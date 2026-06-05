package commands

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gitlab.smy.com/work/sql-resp/internal/resp"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

func init() {
	Register(Cmd{Name: "GET", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdGet)})
	Register(Cmd{Name: "SET", MinArgs: 3, MaxArgs: -1, Exec: withStore(cmdSet)})
	Register(Cmd{Name: "DEL", MinArgs: 2, MaxArgs: -1, Exec: withStore(cmdDel)})
	Register(Cmd{Name: "EXISTS", MinArgs: 2, MaxArgs: -1, Exec: withStore(cmdExists)})
	Register(Cmd{Name: "EXPIRE", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdExpire)})
	Register(Cmd{Name: "PEXPIRE", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdPExpire)})
	Register(Cmd{Name: "TTL", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdTTL)})
	Register(Cmd{Name: "PTTL", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdPTTL)})
	Register(Cmd{Name: "PERSIST", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdPersist)})
	Register(Cmd{Name: "TYPE", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdType)})
	Register(Cmd{Name: "RENAME", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdRename)})
	Register(Cmd{Name: "KEYS", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdKeys)})
	Register(Cmd{Name: "DBSIZE", MinArgs: 1, MaxArgs: 1, Exec: withStore(cmdDBSize)})
	Register(Cmd{Name: "FLUSHDB", MinArgs: 1, MaxArgs: 2, Exec: withStore(cmdFlushDB)})
	Register(Cmd{Name: "FLUSHALL", MinArgs: 1, MaxArgs: 2, Exec: withStore(cmdFlushAll)})
	Register(Cmd{Name: "RANDOMKEY", MinArgs: 1, MaxArgs: 1, Exec: withStore(cmdRandomKey)})
	Register(Cmd{Name: "SELECT", MinArgs: 2, MaxArgs: 2, Exec: cmdSelect})
	Register(Cmd{Name: "INCR", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdIncr)})
	Register(Cmd{Name: "INCRBY", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdIncrBy)})
	Register(Cmd{Name: "DECR", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdDecr)})
	Register(Cmd{Name: "DECRBY", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdDecrBy)})
	Register(Cmd{Name: "MGET", MinArgs: 2, MaxArgs: -1, Exec: withStore(cmdMGet)})
	Register(Cmd{Name: "MSET", MinArgs: 3, MaxArgs: -1, Exec: withStore(cmdMSet)})
	Register(Cmd{Name: "SETNX", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdSetNX)})
	Register(Cmd{Name: "GETSET", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdGetSet)})
	Register(Cmd{Name: "APPEND", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdAppend)})
	Register(Cmd{Name: "STRLEN", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdStrLen)})
}

// storeHandler is a Handler variant that receives the Store and dbIdx.
type storeHandler func(ctx context.Context, s *store.Store, dbIdx int, cc *ClientConn, args []resp.Value) resp.Value

// withStore wraps a storeHandler, extracting the store from the context.
func withStore(h storeHandler) Handler {
	return func(ctx context.Context, cc *ClientConn, args []resp.Value) resp.Value {
		s := storeFromCtx(ctx)
		if s == nil {
			return resp.Error("ERR store not available")
		}
		return h(ctx, s, cc.DB, cc, args)
	}
}

func cmdGet(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	val, found, err := s.Get(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	if !found {
		return resp.NullBulkString()
	}
	return resp.BulkString(val)
}

func cmdSet(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	key := args[1].Str()
	val := args[2].Str()
	opts := store.SetOptions{}

	for i := 3; i < len(args); i++ {
		switch strings.ToUpper(args[i].Str()) {
		case "EX":
			if i+1 >= len(args) {
				return resp.Error(ErrSyntax)
			}
			i++
			secs, err := strconv.ParseInt(args[i].Str(), 10, 64)
			if err != nil || secs <= 0 {
				return resp.Error("ERR invalid expire time in 'set' command")
			}
			ms := time.Now().Add(time.Duration(secs) * time.Second).UnixMilli()
			opts.ExpiresAt = &ms
		case "PX":
			if i+1 >= len(args) {
				return resp.Error(ErrSyntax)
			}
			i++
			ms, err := strconv.ParseInt(args[i].Str(), 10, 64)
			if err != nil || ms <= 0 {
				return resp.Error("ERR invalid expire time in 'set' command")
			}
			ms2 := time.Now().UnixMilli() + ms
			opts.ExpiresAt = &ms2
		case "NX":
			opts.NX = true
		case "XX":
			opts.XX = true
		default:
			return resp.Error(ErrSyntax)
		}
	}

	ok, err := s.Set(ctx, key, dbIdx, val, opts)
	if err != nil {
		return storeErr(err)
	}
	if !ok {
		return resp.NullBulkString()
	}
	return resp.SimpleString("OK")
}

func cmdDel(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	keys := make([]string, len(args)-1)
	for i, a := range args[1:] {
		keys[i] = a.Str()
	}
	n, err := s.Del(ctx, keys, dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdExists(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	keys := make([]string, len(args)-1)
	for i, a := range args[1:] {
		keys[i] = a.Str()
	}
	n, err := s.Exists(ctx, keys, dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdExpire(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	secs, err := strconv.ParseInt(args[2].Str(), 10, 64)
	if err != nil {
		return resp.Error("ERR value is not an integer or out of range")
	}
	ok, err := s.Expire(ctx, args[1].Str(), dbIdx, secs)
	if err != nil {
		return storeErr(err)
	}
	return boolInt(ok)
}

func cmdPExpire(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	ms, err := strconv.ParseInt(args[2].Str(), 10, 64)
	if err != nil {
		return resp.Error("ERR value is not an integer or out of range")
	}
	ok, err := s.PExpire(ctx, args[1].Str(), dbIdx, ms)
	if err != nil {
		return storeErr(err)
	}
	return boolInt(ok)
}

func cmdTTL(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.TTL(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdPTTL(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.PTTL(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdPersist(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	ok, err := s.Persist(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return boolInt(ok)
}

func cmdType(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	typ, err := s.Type(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return resp.SimpleString(typ)
}

func cmdRename(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	if err := s.Rename(ctx, args[1].Str(), args[2].Str(), dbIdx); err != nil {
		if err == store.ErrNoKey {
			return resp.Error("ERR no such key")
		}
		return storeErr(err)
	}
	return resp.SimpleString("OK")
}

func cmdKeys(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	keys, err := s.Keys(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	elems := make([]resp.Value, len(keys))
	for i, k := range keys {
		elems[i] = resp.BulkString(k)
	}
	return resp.Array(elems)
}

func cmdDBSize(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, _ []resp.Value) resp.Value {
	n, err := s.DBSize(ctx, dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdFlushDB(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, _ []resp.Value) resp.Value {
	if err := s.FlushDB(ctx, dbIdx); err != nil {
		return storeErr(err)
	}
	return resp.SimpleString("OK")
}

func cmdFlushAll(ctx context.Context, s *store.Store, _ int, _ *ClientConn, _ []resp.Value) resp.Value {
	if err := s.FlushAll(ctx); err != nil {
		return storeErr(err)
	}
	return resp.SimpleString("OK")
}

func cmdRandomKey(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, _ []resp.Value) resp.Value {
	key, err := s.RandomKey(ctx, dbIdx)
	if err != nil {
		return storeErr(err)
	}
	if key == "" {
		return resp.NullBulkString()
	}
	return resp.BulkString(key)
}

func cmdSelect(_ context.Context, cc *ClientConn, args []resp.Value) resp.Value {
	n, err := strconv.Atoi(args[1].Str())
	if err != nil || n < 0 || n > 15 {
		return resp.Error("ERR DB index is out of range")
	}
	cc.DB = n
	return resp.SimpleString("OK")
}

func cmdIncr(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.IncrBy(ctx, args[1].Str(), dbIdx, 1)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdIncrBy(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	delta, err := strconv.ParseInt(args[2].Str(), 10, 64)
	if err != nil {
		return resp.Error("ERR value is not an integer or out of range")
	}
	n, err := s.IncrBy(ctx, args[1].Str(), dbIdx, delta)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdDecr(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.IncrBy(ctx, args[1].Str(), dbIdx, -1)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdDecrBy(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	delta, err := strconv.ParseInt(args[2].Str(), 10, 64)
	if err != nil {
		return resp.Error("ERR value is not an integer or out of range")
	}
	n, err := s.IncrBy(ctx, args[1].Str(), dbIdx, -delta)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdMGet(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	elems := make([]resp.Value, len(args)-1)
	for i, a := range args[1:] {
		val, found, err := s.Get(ctx, a.Str(), dbIdx)
		if err != nil || !found {
			elems[i] = resp.NullBulkString()
		} else {
			elems[i] = resp.BulkString(val)
		}
	}
	return resp.Array(elems)
}

func cmdMSet(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	if len(args)%2 == 0 {
		return resp.Error("ERR wrong number of arguments for 'mset' command")
	}
	for i := 1; i < len(args); i += 2 {
		if _, err := s.Set(ctx, args[i].Str(), dbIdx, args[i+1].Str(), store.SetOptions{}); err != nil {
			return storeErr(err)
		}
	}
	return resp.SimpleString("OK")
}

func cmdSetNX(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	ok, err := s.Set(ctx, args[1].Str(), dbIdx, args[2].Str(), store.SetOptions{NX: true})
	if err != nil {
		return storeErr(err)
	}
	return boolInt(ok)
}

func cmdGetSet(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	old, found, err := s.Get(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	if _, err := s.Set(ctx, args[1].Str(), dbIdx, args[2].Str(), store.SetOptions{}); err != nil {
		return storeErr(err)
	}
	if !found {
		return resp.NullBulkString()
	}
	return resp.BulkString(old)
}

func cmdAppend(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.Append(ctx, args[1].Str(), dbIdx, args[2].Str())
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdStrLen(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.StrLen(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func boolInt(b bool) resp.Value {
	if b {
		return resp.Integer(1)
	}
	return resp.Integer(0)
}

func storeErr(err error) resp.Value {
	if err == store.ErrWrongType {
		return resp.Error(ErrWrongType)
	}
	return resp.Error("ERR " + err.Error())
}
