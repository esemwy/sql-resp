// Package commands implements the command registry and all command handlers.
package commands

import (
	"context"
	"fmt"
	"strings"

	"gitlab.smy.com/work/sql-resp/internal/resp"
)

// Handler is the function signature for a command handler.
// args[0] is always the command name (uppercase).
type Handler func(ctx context.Context, cc *ClientConn, args []resp.Value) resp.Value

// Cmd describes a registered command.
type Cmd struct {
	Name    string
	MinArgs int // minimum number of args including the command name
	MaxArgs int // -1 means variadic
	Exec    Handler
}

var registry = map[string]Cmd{}

// Register adds a command to the registry. Called from init() in each command file.
func Register(c Cmd) {
	registry[strings.ToUpper(c.Name)] = c
}

// Dispatch looks up and executes a command.
func Dispatch(ctx context.Context, cc *ClientConn, args []resp.Value) resp.Value {
	if len(args) == 0 {
		return resp.Error("ERR empty command")
	}
	name := strings.ToUpper(args[0].Str())
	cmd, ok := registry[name]
	if !ok {
		return resp.Error(fmt.Sprintf("ERR unknown command `%s`", strings.ToLower(name)))
	}
	if len(args) < cmd.MinArgs {
		return resp.Error(fmt.Sprintf("ERR wrong number of arguments for '%s' command", strings.ToLower(name)))
	}
	if cmd.MaxArgs >= 0 && len(args) > cmd.MaxArgs {
		return resp.Error(fmt.Sprintf("ERR wrong number of arguments for '%s' command", strings.ToLower(name)))
	}
	return cmd.Exec(ctx, cc, args)
}

// All returns every registered command, sorted by name. Used by COMMAND.
func All() []Cmd {
	cmds := make([]Cmd, 0, len(registry))
	for _, c := range registry {
		cmds = append(cmds, c)
	}
	return cmds
}
