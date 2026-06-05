package commands

import (
	"context"
	"strings"

	"gitlab.smy.com/work/sql-resp/internal/resp"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

func init() {
	Register(Cmd{Name: "HSET", MinArgs: 4, MaxArgs: -1, Exec: withStore(cmdHSet)})
	Register(Cmd{Name: "HGET", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdHGet)})
	Register(Cmd{Name: "HDEL", MinArgs: 3, MaxArgs: -1, Exec: withStore(cmdHDel)})
	Register(Cmd{Name: "HEXISTS", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdHExists)})
	Register(Cmd{Name: "HGETALL", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdHGetAll)})
	Register(Cmd{Name: "HKEYS", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdHKeys)})
	Register(Cmd{Name: "HVALS", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdHVals)})
	Register(Cmd{Name: "HLEN", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdHLen)})
	Register(Cmd{Name: "HMGET", MinArgs: 3, MaxArgs: -1, Exec: withStore(cmdHMGet)})
	Register(Cmd{Name: "HMSET", MinArgs: 4, MaxArgs: -1, Exec: withStore(cmdHMSet)})
}

func cmdHSet(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	if (len(args)-2)%2 != 0 {
		return resp.Error("ERR wrong number of arguments for 'hset' command")
	}
	key := args[1].Str()
	pairs := map[string]string{}
	for i := 2; i < len(args); i += 2 {
		pairs[args[i].Str()] = args[i+1].Str()
	}
	n, err := s.HSet(ctx, key, dbIdx, pairs)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdHGet(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	val, found, err := s.HGet(ctx, args[1].Str(), dbIdx, args[2].Str())
	if err != nil {
		return storeErr(err)
	}
	if !found {
		return resp.NullBulkString()
	}
	return resp.BulkString(val)
}

func cmdHDel(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	fields := make([]string, len(args)-2)
	for i, a := range args[2:] {
		fields[i] = a.Str()
	}
	n, err := s.HDel(ctx, args[1].Str(), dbIdx, fields)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdHExists(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	ok, err := s.HExists(ctx, args[1].Str(), dbIdx, args[2].Str())
	if err != nil {
		return storeErr(err)
	}
	return boolInt(ok)
}

func cmdHGetAll(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	m, err := s.HGetAll(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	elems := make([]resp.Value, 0, len(m)*2)
	for f, v := range m {
		elems = append(elems, resp.BulkString(f), resp.BulkString(v))
	}
	return resp.Array(elems)
}

func cmdHKeys(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	keys, err := s.HKeys(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return strSlice(keys)
}

func cmdHVals(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	vals, err := s.HVals(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return strSlice(vals)
}

func cmdHLen(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.HLen(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdHMGet(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	fields := make([]string, len(args)-2)
	for i, a := range args[2:] {
		fields[i] = a.Str()
	}
	vals, err := s.HMGet(ctx, args[1].Str(), dbIdx, fields)
	if err != nil {
		return storeErr(err)
	}
	elems := make([]resp.Value, len(vals))
	for i, v := range vals {
		if v == nil {
			elems[i] = resp.NullBulkString()
		} else {
			elems[i] = resp.BulkString(v.(string))
		}
	}
	return resp.Array(elems)
}

func cmdHMSet(ctx context.Context, s *store.Store, dbIdx int, cc *ClientConn, args []resp.Value) resp.Value {
	// HMSET is identical to HSET but always returns OK.
	if (len(args)-2)%2 != 0 {
		return resp.Error("ERR wrong number of arguments for 'hmset' command")
	}
	key := args[1].Str()
	pairs := map[string]string{}
	for i := 2; i < len(args); i += 2 {
		pairs[args[i].Str()] = args[i+1].Str()
	}
	_, err := s.HSet(ctx, key, dbIdx, pairs)
	if err != nil {
		return storeErr(err)
	}
	return resp.SimpleString("OK")
}

func strSlice(vals []string) resp.Value {
	elems := make([]resp.Value, len(vals))
	for i, v := range vals {
		elems[i] = resp.BulkString(v)
	}
	return resp.Array(elems)
}

var _ = strings.ToUpper // suppress unused import if needed
