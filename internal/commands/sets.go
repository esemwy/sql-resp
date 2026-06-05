package commands

import (
	"context"

	"gitlab.smy.com/work/sql-resp/internal/resp"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

func init() {
	Register(Cmd{Name: "SADD", MinArgs: 3, MaxArgs: -1, Exec: withStore(cmdSAdd)})
	Register(Cmd{Name: "SREM", MinArgs: 3, MaxArgs: -1, Exec: withStore(cmdSRem)})
	Register(Cmd{Name: "SMEMBERS", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdSMembers)})
	Register(Cmd{Name: "SISMEMBER", MinArgs: 3, MaxArgs: 3, Exec: withStore(cmdSIsMember)})
	Register(Cmd{Name: "SCARD", MinArgs: 2, MaxArgs: 2, Exec: withStore(cmdSCard)})
	Register(Cmd{Name: "SUNION", MinArgs: 2, MaxArgs: -1, Exec: withStore(cmdSUnion)})
	Register(Cmd{Name: "SINTER", MinArgs: 2, MaxArgs: -1, Exec: withStore(cmdSInter)})
	Register(Cmd{Name: "SDIFF", MinArgs: 2, MaxArgs: -1, Exec: withStore(cmdSDiff)})
}

func cmdSAdd(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.SAdd(ctx, args[1].Str(), dbIdx, argsToStrings(args[2:]))
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdSRem(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.SRem(ctx, args[1].Str(), dbIdx, argsToStrings(args[2:]))
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdSMembers(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	members, err := s.SMembers(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return strSlice(members)
}

func cmdSIsMember(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	ok, err := s.SIsMember(ctx, args[1].Str(), dbIdx, args[2].Str())
	if err != nil {
		return storeErr(err)
	}
	return boolInt(ok)
}

func cmdSCard(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	n, err := s.SCard(ctx, args[1].Str(), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return resp.Integer(n)
}

func cmdSUnion(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	members, err := s.SUnion(ctx, argsToStrings(args[1:]), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return strSlice(members)
}

func cmdSInter(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	members, err := s.SInter(ctx, argsToStrings(args[1:]), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return strSlice(members)
}

func cmdSDiff(ctx context.Context, s *store.Store, dbIdx int, _ *ClientConn, args []resp.Value) resp.Value {
	members, err := s.SDiff(ctx, argsToStrings(args[1:]), dbIdx)
	if err != nil {
		return storeErr(err)
	}
	return strSlice(members)
}
