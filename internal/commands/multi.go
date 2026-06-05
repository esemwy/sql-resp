package commands

import (
	"context"

	"gitlab.smy.com/work/sql-resp/internal/resp"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

func init() {
	Register(Cmd{Name: "MULTI", MinArgs: 1, MaxArgs: 1, Exec: cmdMulti})
	Register(Cmd{Name: "EXEC", MinArgs: 1, MaxArgs: 1, Exec: cmdExec})
	Register(Cmd{Name: "DISCARD", MinArgs: 1, MaxArgs: 1, Exec: cmdDiscard})
}

func cmdMulti(_ context.Context, cc *ClientConn, _ []resp.Value) resp.Value {
	if cc.InMulti {
		return resp.Error("ERR MULTI calls can not be nested")
	}
	cc.InMulti = true
	cc.QueuedCmds = nil
	return resp.SimpleString("OK")
}

func cmdDiscard(_ context.Context, cc *ClientConn, _ []resp.Value) resp.Value {
	if !cc.InMulti {
		return resp.Error("ERR DISCARD without MULTI")
	}
	cc.InMulti = false
	cc.QueuedCmds = nil
	return resp.SimpleString("OK")
}

func cmdExec(ctx context.Context, cc *ClientConn, _ []resp.Value) resp.Value {
	if !cc.InMulti {
		return resp.Error("ERR EXEC without MULTI")
	}
	cc.InMulti = false
	queued := cc.QueuedCmds
	cc.QueuedCmds = nil

	results := make([]resp.Value, len(queued))
	for i, args := range queued {
		results[i] = Dispatch(ctx, cc, args)
	}
	return resp.Array(results)
}

// QueueOrDispatch is called by the server connection loop instead of Dispatch
// when a connection may be in MULTI mode.
func QueueOrDispatch(ctx context.Context, cc *ClientConn, args []resp.Value) (resp.Value, bool) {
	if !cc.InMulti {
		return Dispatch(ctx, cc, args), false
	}
	name := ""
	if len(args) > 0 {
		name = upperStr(args[0].Str())
	}
	// EXEC, DISCARD, QUIT exit or commit the transaction.
	switch name {
	case "EXEC", "DISCARD", "QUIT":
		return Dispatch(ctx, cc, args), false
	case "MULTI":
		return resp.Error("ERR MULTI calls can not be nested"), false
	}
	// Validate the command exists and has correct arity before queuing.
	cmd, ok := registry[name]
	if !ok {
		return resp.Error("ERR unknown command `" + lowerStr(name) + "`"), false
	}
	if len(args) < cmd.MinArgs {
		return resp.Error("ERR wrong number of arguments for '" + lowerStr(name) + "' command"), false
	}
	cc.QueuedCmds = append(cc.QueuedCmds, args)
	return resp.SimpleString("QUEUED"), false
}

func upperStr(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

func lowerStr(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// Ensure store import is used somewhere to avoid circular import issues.
var _ *store.Store
