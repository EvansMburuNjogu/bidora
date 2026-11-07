package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type operationError struct {
	code  string
	cause error
}

func (e *operationError) Error() string                 { return e.code }
func (e *operationError) Unwrap() error                 { return e.cause }
func (e *operationError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.code) }
func safeError(code string, cause error) error          { return &operationError{code, cause} }

// Pool owns its connections. No raw pool/connection is exported, preventing
// unbounded borrows from defeating shutdown. New is lazy, not readiness proof.
// Do not copy. There is no implicit retry, migration or tenant fallback.
type Pool struct {
	pool    *pgxpool.Pool
	life    context.Context
	cancel  context.CancelFunc
	once    sync.Once
	done    chan struct{}
	timeout time.Duration
}

// New creates a lazy zero-minimum pool without contacting PostgreSQL. Readiness
// must be established explicitly through Ping on the intended target/identity.
func New(config Config) (*Pool, error) {
	settings, err := poolConfig(config)
	if err != nil {
		return nil, err
	}
	life, cancel := context.WithCancel(context.Background())
	p, err := pgxpool.NewWithConfig(life, settings)
	if err != nil {
		cancel()
		return nil, safeError("postgres_pool_create_failed", err)
	}
	return &Pool{pool: p, life: life, cancel: cancel, done: make(chan struct{}), timeout: config.OperationTimeout}, nil
}

// Ping bounds acquisition, connection and SQL responsiveness checks by the
// caller's deadline or configured operation timeout, whichever occurs first.
func (p *Pool) Ping(ctx context.Context) error {
	if ctx == nil {
		return errors.New("postgres operation requires a context")
	}
	if p == nil || p.pool == nil || p.life.Err() != nil {
		return errors.New("postgres_pool_closed")
	}
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	stop := context.AfterFunc(p.life, cancel)
	defer func() { stop(); cancel() }()
	if err := p.pool.Ping(ctx); err != nil {
		return safeError("postgres_unavailable", err)
	}
	return nil
}

// DBTX is the narrow pgx surface accepted by generated infrastructure queries.
// Application/domain packages must not depend on it.
type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// WithinTransaction owns read-write begin/rollback/commit with configured
// operation bounds. The callback must perform database work only.
func (p *Pool) WithinTransaction(ctx context.Context, fn func(context.Context, DBTX) error) error {
	return p.withinTransaction(ctx, pgx.ReadWrite, pgx.ReadCommitted, fn)
}

// WithinReadOnlyTransaction gives bounded query adapters an explicitly read-only
// transaction without exporting the raw pool.
func (p *Pool) WithinReadOnlyTransaction(ctx context.Context, fn func(context.Context, DBTX) error) error {
	return p.withinTransaction(ctx, pgx.ReadOnly, pgx.ReadCommitted, fn)
}

// WithinConsistentReadTransaction gives a multi-owner authorization read one
// repeatable snapshot. It remains read-only and does not replace command locks.
func (p *Pool) WithinConsistentReadTransaction(ctx context.Context, fn func(context.Context, DBTX) error) error {
	return p.withinTransaction(ctx, pgx.ReadOnly, pgx.RepeatableRead, fn)
}

func (p *Pool) withinTransaction(ctx context.Context, mode pgx.TxAccessMode, isolation pgx.TxIsoLevel, fn func(context.Context, DBTX) error) error {
	if ctx == nil {
		return errors.New("postgres transaction requires a context")
	}
	if fn == nil {
		return errors.New("postgres transaction requires a callback")
	}
	if p == nil || p.pool == nil || p.life.Err() != nil {
		return errors.New("postgres_pool_closed")
	}
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	stop := context.AfterFunc(p.life, cancel)
	defer func() { stop(); cancel() }()
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: isolation, AccessMode: mode})
	if err != nil {
		return safeError("postgres_transaction_begin_failed", err)
	}
	committed := false
	defer func() {
		if !committed {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), p.timeout)
			defer rollbackCancel()
			_ = tx.Rollback(rollbackCtx)
		}
	}()
	if mode == pgx.ReadWrite {
		if err = applyRequestAudit(ctx, tx); err != nil {
			return err
		}
	}
	if err = fn(ctx, tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return safeError("postgres_transaction_commit_failed", err)
	}
	committed = true
	return nil
}

// Close stops new work, cancels active operations and waits for pool cleanup.
// If the caller deadline expires, cleanup continues and a later Close can wait
// again. A timeout is not reported as successful closure. Driver connection
// construction remains bounded by ConnectTimeout even after acquisition cancels.
func (p *Pool) Close(ctx context.Context) error {
	if ctx == nil {
		return errors.New("postgres shutdown requires a context")
	}
	if p == nil || p.pool == nil {
		return nil
	}
	p.once.Do(func() {
		p.cancel()
		go func() { p.pool.Close(); close(p.done) }()
	})
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return safeError("postgres_shutdown_incomplete", ctx.Err())
	}
}
