package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	// ErrMaxConnsUnconfigured will be returned, if the
	// the maximum connections are zero.
	ErrMaxConnsUnconfigured = errors.New("MaxConns not configured")
)

// ConnectOpts database connection options
type ConnectOpts struct {
	URL      string
	MaxConns int32
	MinConns int32
}

// Connect creates and configures a pgx pool
func Connect(ctx context.Context, opts *ConnectOpts) (*pgxpool.Pool, error) {
	// Initialize postgres connection
	cfg, err := pgxpool.ParseConfig(opts.URL)
	if err != nil {
		return nil, err
	}

	cfg.ConnConfig.RuntimeParams["application_name"] = filepath.Base(os.Args[0])
	if opts.MaxConns == 0 {
		return nil, ErrMaxConnsUnconfigured
	}

	// We need some more connections
	cfg.MaxConns = opts.MaxConns
	cfg.MinConns = opts.MinConns

	return pgxpool.ConnectConfig(ctx, cfg)
}
