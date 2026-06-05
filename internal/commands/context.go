package commands

import (
	"context"

	"gitlab.smy.com/work/sql-resp/internal/store"
)

type ctxKey int

const storeKey ctxKey = 0

// WithStore returns a context carrying the store.
func WithStore(ctx context.Context, s *store.Store) context.Context {
	return context.WithValue(ctx, storeKey, s)
}

// storeFromCtx retrieves the store from the context.
func storeFromCtx(ctx context.Context) *store.Store {
	s, _ := ctx.Value(storeKey).(*store.Store)
	return s
}
