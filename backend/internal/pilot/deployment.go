package pilot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/health"
	"bidora/backend/internal/postgres"
)

type Deployment struct {
	Runtime *Runtime
	State   *health.State
	Address string
	pools   []*postgres.Pool
}

func Open(ctx context.Context, config DeploymentConfig) (*Deployment, error) {
	if ctx == nil || config.Address == "" || len(config.Tenants) == 0 {
		return nil, errors.New("pilot_deployment_invalid")
	}
	state := &health.State{}
	pools := make([]*postgres.Pool, 0, len(config.Tenants)+1)
	cleanup := func(runtime *Runtime) {
		state.Drain()
		if runtime != nil {
			_ = runtime.Close()
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for index := len(pools) - 1; index >= 0; index-- {
			_ = pools[index].Close(cleanupCtx)
		}
	}
	control, err := postgres.New(config.Control)
	if err != nil {
		return nil, &lifecycleError{code: "pilot_deployment_open_failed", cause: err}
	}
	pools = append(pools, control)
	tenants := make([]TenantPool, 0, len(config.Tenants))
	for _, configured := range config.Tenants {
		pool, openErr := postgres.New(configured.Postgres)
		if openErr != nil {
			cleanup(nil)
			return nil, &lifecycleError{code: "pilot_deployment_open_failed", cause: openErr}
		}
		pools = append(pools, pool)
		tenants = append(tenants, TenantPool{Assignment: configured.Assignment, Pool: pool})
	}
	runtime, err := Build(config.Runtime, Dependencies{State: state, Control: control, Tenants: tenants})
	if err != nil {
		cleanup(nil)
		return nil, &lifecycleError{code: "pilot_deployment_open_failed", cause: err}
	}
	if err = runtime.Probe(ctx); err != nil {
		cleanup(runtime)
		return nil, err
	}
	state.Start()
	return &Deployment{Runtime: runtime, State: state, Address: config.Address, pools: pools}, nil
}
func (d *Deployment) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("pilot_shutdown_invalid")
	}
	if d == nil {
		return nil
	}
	if d.State != nil {
		d.State.Drain()
	}
	var failures []error
	if d.Runtime != nil && d.Runtime.App != nil {
		if err := d.Runtime.App.ShutdownWithContext(ctx); err != nil {
			failures = append(failures, &lifecycleError{code: "pilot_http_shutdown_incomplete", cause: err})
		}
	}
	if d.Runtime != nil {
		if err := d.Runtime.Close(); err != nil {
			failures = append(failures, &lifecycleError{code: "pilot_redis_shutdown_incomplete", cause: err})
		}
	}
	for index := len(d.pools) - 1; index >= 0; index-- {
		if err := d.pools[index].Close(ctx); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

type lifecycleError struct {
	code  string
	cause error
}

func (e *lifecycleError) Error() string                 { return e.code }
func (e *lifecycleError) Unwrap() error                 { return e.cause }
func (e *lifecycleError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.code) }
