package commands

import "gitlab.smy.com/work/sql-resp/internal/resp"

// ClientConn holds per-connection state visible to command handlers.
// The server creates one per accepted connection.
type ClientConn struct {
	DB int // currently selected database index (0–15)

	// Multi/Exec state
	InMulti    bool
	QueuedCmds [][]resp.Value // commands queued during a MULTI block

	// Auth state
	Authenticated bool
}

// NewClientConn returns a ClientConn with default state.
// authenticated is true when the server has no requirepass configured.
func NewClientConn(authenticated bool) *ClientConn {
	return &ClientConn{
		DB:            0,
		Authenticated: authenticated,
	}
}
