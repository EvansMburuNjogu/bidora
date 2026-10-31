// Package localprofile validates resource settings for a future local-only
// PostgreSQL profile. It does not allocate resources or enforce runtime quotas.
package localprofile

import "errors"

// Config is the explicit input boundary for future local-profile integration.
// Values must be supplied by the caller; the zero value is invalid.
// Numeric fields are integers in the units indicated by their names.
type Config struct {
	Environment string
	Profile     string
	CPUs        int
	MemoryMiB   int
	StorageGiB  int
}

// Validate checks local/test scope and the approved resource ceilings without
// mutating the input, applying defaults, or performing I/O. It returns the first
// invalid field in declaration order and never includes caller-supplied values.
// Success does not establish viable PostgreSQL sizing, readiness or disk limits.
func (c Config) Validate() error {
	if c.Environment != "local" && c.Environment != "test" {
		return errors.New("environment must be local or test")
	}
	if c.Profile != "postgres-local" {
		return errors.New("profile must be postgres-local")
	}
	if c.CPUs < 1 || c.CPUs > 2 {
		return errors.New("CPUs must be between 1 and 2")
	}
	if c.MemoryMiB < 1 || c.MemoryMiB > 2048 {
		return errors.New("memory MiB must be between 1 and 2048")
	}
	if c.StorageGiB < 1 || c.StorageGiB > 10 {
		return errors.New("storage GiB must be between 1 and 10")
	}
	return nil
}
