package postgres

import (
	"context"
	"os"
	"path/filepath"

	"github.com/alice-lg/alice-lg/pkg/config"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Connect creates and configures a pgx pool
func Connect(ctx context.Context, opts *config.PostgresConfig) (*pgxpool.Pool, error) {
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
