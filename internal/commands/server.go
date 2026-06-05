package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"gitlab.smy.com/work/sql-resp/internal/resp"
)

func init() {
	Register(Cmd{
		Name:    "PING",
		MinArgs: 1,
		MaxArgs: 2,
		Exec:    cmdPing,
	})
	Register(Cmd{
		Name:    "ECHO",
		MinArgs: 2,
		MaxArgs: 2,
		Exec:    cmdEcho,
	})
	Register(Cmd{
		Name:    "QUIT",
		MinArgs: 1,
		MaxArgs: 1,
		Exec:    cmdQuit,
	})
	Register(Cmd{
		Name:    "COMMAND",
		MinArgs: 1,
		MaxArgs: -1,
		Exec:    cmdCommand,
	})
}

func cmdPing(_ context.Context, _ *ClientConn, args []resp.Value) resp.Value {
	if len(args) == 2 {
		return resp.BulkString(args[1].Str())
	}
	return resp.SimpleString("PONG")
}

func cmdEcho(_ context.Context, _ *ClientConn, args []resp.Value) resp.Value {
	return resp.BulkString(args[1].Str())
}

func cmdQuit(_ context.Context, _ *ClientConn, _ []resp.Value) resp.Value {
	return resp.SimpleString("OK")
}

func cmdCommand(_ context.Context, _ *ClientConn, args []resp.Value) resp.Value {
	if len(args) >= 2 && strings.ToUpper(args[1].Str()) == "COUNT" {
		return resp.Integer(int64(len(registry)))
	}

	cmds := All()
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })

	elems := make([]resp.Value, len(cmds))
	for i, c := range cmds {
		arity := int64(c.MinArgs)
		if c.MaxArgs == -1 {
			arity = -arity
		}
		elems[i] = resp.Array([]resp.Value{
			resp.BulkString(strings.ToLower(c.Name)),
			resp.Integer(arity),
			resp.Array([]resp.Value{}), // flags (empty for now)
			resp.Integer(0),            // first key
			resp.Integer(0),            // last key
			resp.Integer(0),            // step
		})
	}
	return resp.Array(elems)
}

// ErrWrongType is the standard Redis wrong-type error message.
const ErrWrongType = "WRONGTYPE Operation against a key holding the wrong kind of value"

// ErrSyntax is the standard syntax error.
const ErrSyntax = "ERR syntax error"

// Errorf returns a resp.Error with a formatted message.
func Errorf(format string, args ...any) resp.Value {
	return resp.Error(fmt.Sprintf(format, args...))
}
