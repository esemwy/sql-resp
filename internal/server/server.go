// Package server implements the TCP listener and per-connection handler.
package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"gitlab.smy.com/work/sql-resp/internal/commands"
	"gitlab.smy.com/work/sql-resp/internal/pubsub"
	"gitlab.smy.com/work/sql-resp/internal/resp"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

// Config holds the options the server needs at startup.
type Config struct {
	Addr      string // e.g. ":6379"
	Password  string // empty = no auth required
	TLSConfig *tls.Config
	Store     *store.Store // nil = no data commands available
}

// Server is a RESP2 TCP server.
type Server struct {
	cfg      Config
	listener net.Listener
	broker   *pubsub.Broker
}

// New creates a Server but does not start listening yet.
func New(cfg Config) *Server {
	return &Server{
		cfg:    cfg,
		broker: pubsub.New(),
	}
}

// ListenAndServe binds the listener and blocks serving connections.
// Returns when the listener is closed.
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	if s.cfg.TLSConfig != nil {
		ln = tls.NewListener(ln, s.cfg.TLSConfig)
	}
	s.listener = ln
	log.Printf("sql-resp listening on %s", ln.Addr())
	return s.serve(ln)
}

// Close stops accepting new connections.
func (s *Server) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *Server) serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

// connWriter wraps a net.Conn and bufio.Writer for thread-safe writing.
// Pub/sub push messages and command responses share the same writer.
type connWriter struct {
	mu sync.Mutex
	bw *bufio.Writer
}

func newConnWriter(conn net.Conn) *connWriter {
	return &connWriter{bw: bufio.NewWriter(conn)}
}

func (cw *connWriter) Write(v resp.Value) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	if err := v.WriteTo(cw.bw); err != nil {
		return err
	}
	return cw.bw.Flush()
}

// connSub implements pubsub.Subscriber for a connection.
type connSub struct {
	cw *connWriter
}

func (cs *connSub) Deliver(channel, message string) {
	msg := resp.Array([]resp.Value{
		resp.BulkString("message"),
		resp.BulkString(channel),
		resp.BulkString(message),
	})
	cs.cw.Write(msg) //nolint:errcheck
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	authenticated := s.cfg.Password == ""
	cc := commands.NewClientConn(authenticated)
	rd := resp.NewReader(conn)
	cw := newConnWriter(conn)
	sub := &connSub{cw: cw}

	// Track subscribed channels for this connection.
	subscribedChannels := map[string]bool{}

	for {
		val, err := rd.ReadValue()
		if err != nil {
			if err != io.EOF {
				log.Printf("read error: %v", err)
			}
			// Clean up subscriptions on disconnect.
			s.broker.Unsubscribe("", sub)
			return
		}

		args := toArgs(val)
		if len(args) == 0 {
			continue
		}

		name := strings.ToUpper(args[0].Str())

		// Auth check.
		if !cc.Authenticated && name != "AUTH" && name != "QUIT" {
			cw.Write(resp.Error("NOAUTH Authentication required.")) //nolint:errcheck
			continue
		}

		if name == "AUTH" {
			r := s.handleAuth(cc, args)
			cw.Write(r) //nolint:errcheck
			continue
		}

		// Pub/sub commands.
		switch name {
		case "SUBSCRIBE":
			for _, a := range args[1:] {
				ch := a.Str()
				n := s.broker.Subscribe(ch, sub)
				subscribedChannels[ch] = true
				cw.Write(resp.Array([]resp.Value{ //nolint:errcheck
					resp.BulkString("subscribe"),
					resp.BulkString(ch),
					resp.Integer(int64(n)),
				}))
			}
			continue
		case "UNSUBSCRIBE":
			if len(args) == 1 {
				// Unsubscribe all.
				s.broker.Unsubscribe("", sub)
				subscribedChannels = map[string]bool{}
				cw.Write(resp.Array([]resp.Value{ //nolint:errcheck
					resp.BulkString("unsubscribe"),
					resp.NullBulkString(),
					resp.Integer(0),
				}))
			} else {
				for _, a := range args[1:] {
					ch := a.Str()
					n := s.broker.Unsubscribe(ch, sub)
					delete(subscribedChannels, ch)
					cw.Write(resp.Array([]resp.Value{ //nolint:errcheck
						resp.BulkString("unsubscribe"),
						resp.BulkString(ch),
						resp.Integer(int64(n)),
					}))
				}
			}
			continue
		case "PUBLISH":
			if len(args) < 3 {
				cw.Write(resp.Error("ERR wrong number of arguments for 'publish' command")) //nolint:errcheck
				continue
			}
			n := s.broker.Publish(args[1].Str(), args[2].Str())
			cw.Write(resp.Integer(int64(n))) //nolint:errcheck
			continue
		}

		ctx := context.Background()
		if s.cfg.Store != nil {
			ctx = commands.WithStore(ctx, s.cfg.Store)
		}
		r, _ := commands.QueueOrDispatch(ctx, cc, args)
		cw.Write(r) //nolint:errcheck

		if name == "QUIT" {
			s.broker.Unsubscribe("", sub)
			return
		}
	}
}

func (s *Server) handleAuth(cc *commands.ClientConn, args []resp.Value) resp.Value {
	if len(args) < 2 {
		return resp.Error("ERR wrong number of arguments for 'auth' command")
	}
	if s.cfg.Password == "" {
		cc.Authenticated = true
		return resp.SimpleString("OK")
	}
	if args[1].Str() == s.cfg.Password {
		cc.Authenticated = true
		return resp.SimpleString("OK")
	}
	return resp.Error("WRONGPASS invalid username-password pair or user is disabled.")
}

// toArgs extracts the array elements from a RESP value as a command argument list.
func toArgs(v resp.Value) []resp.Value {
	if v.Type() == resp.TypeArray {
		return v.Elems()
	}
	return []resp.Value{v}
}

// Addr returns the server's listening address (for tests).
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return fmt.Sprintf("%s", s.listener.Addr())
}
