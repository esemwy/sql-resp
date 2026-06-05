package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gitlab.smy.com/work/sql-resp/internal/resp"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

func init() {
	Register(Cmd{Name: "ZADD", MinArgs: 4, MaxArgs: -1, Exec: withStore(cmdZAdd)})
	Register(Cmd{Name: "ZREM", MinArgs: 3, MaxArgs: -1, Exec: withStore(cmdZRem)})
	Register(Cmd{Name: "ZSCORE", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdZScore)})
	Register(Cmd{Name: "ZRANK", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdZRank)})
	Register(Cmd{Name: "ZRANGE", MinArgs: 4, MaxArgs: 4, Exec: withStore(cmdZRange)})
	Register(Cmd{Name: "ZRANGEBYSCORE", MinArgs: 4, MaxArgs: -1, Exec: withStore(cmdZRangeByScore)})
	Register(Cmd{Name: "ZCARD", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdZCard)})
	Register(Cmd{Name: "ZCOUNT", MinArgs: 4, MaxArgs: 4, Exec: withStore(cmdZCount)})
	Register(Cmd{Name: "ZINCRBY", MinArgs: 4, MaxArgs: 4, Exec: withStore(cmdZIncrBy)})
}

func cmdZAdd(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	opts := store.ZAddOptions{}
	i := 2
	for ; i < len(args); i++ {
		switch strings.ToUpper(args[i].Str()) {
		case "NX":
			opts.NX = true
		case "XX":
			opts.XX = true
		default:
			goto parsePairs
		}
	}
parsePairs:
	if (len(args)-i)%2 != 0 {
		return resp.Error("ERR syntax error")
	}
	members := map[string]float64{}
	for ; i < len(args); i += 2 {
		score, err := strconv.ParseFloat(args[i].Str(), 64)
		if err != nil {
			return resp.Error("ERR value is not a valid float")
		}
		members[args[i+1].Str()] = score
	}
	n, err := s.ZAdd(ctx, args[1].Str(), dbIdx, members, opts)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdZRem(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.ZRem(ctx, args[1].Str(), dbIdx, argsToStrings(args[2:]))
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdZScore(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	score, found, err := s.ZScore(ctx, args[1].Str(), dbIdx, args[2].Str())
	if err != nil {
		return storeErr(err)
	}
	if !found {
		return resp.NullBulkString()
	}
	return resp.BulkString(formatFloat(score))
}

func cmdZRank(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	rank, found, err := s.ZRank(ctx, args[1].Str(), dbIdx, args[2].Str())
	if err != nil {
		return storeErr(err)
	}
	if !found {
		return resp.NullBulkString()
	}
	return resp.Integer(rank)
}

func cmdZRange(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	start, err1 := strconv.ParseInt(args[2].Str(), 10, 64)
	stop, err2 := strconv.ParseInt(args[3].Str(), 10, 64)
	if err1 != nil || err2 != nil {
		return resp.Error("ERR value is not an integer or out of range")
	}
	members, err := s.ZRange(ctx, args[1].Str(), dbIdx, start, stop)
	if err != nil {
		return storeErr(err)
	}
	return strSlice(members)
}

func cmdZRangeByScore(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	members, err := s.ZRangeByScore(ctx, args[1].Str(), dbIdx, args[2].Str(), args[3].Str())
	if err != nil {
		return storeErr(err)
	}
	return strSlice(members)
}

func cmdZCard(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.ZCard(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdZCount(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.ZCount(ctx, args[1].Str(), dbIdx, args[2].Str(), args[3].Str())
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdZIncrBy(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	delta, err := strconv.ParseFloat(args[2].Str(), 64)
	if err != nil {
		return resp.Error("ERR value is not a valid float")
	}
	newScore, err := s.ZIncrBy(ctx, args[1].Str(), dbIdx, delta, args[3].Str())
	if err != nil {
		return storeErr(err)
	}
	return resp.BulkString(formatFloat(newScore))
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}
