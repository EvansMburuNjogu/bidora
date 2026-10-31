package localprofile_test

import (
	"net/netip"
	"strings"
	"testing"

	"bidora/backend/internal/localprofile"
)

func TestLoadPostgresLoopback(t *testing.T) {
	for _, address := range []string{"127.0.0.1:5432", "127.0.0.2:1", "127.0.0.1:65535", "[::1]:15432"} {
		t.Run(address, func(t *testing.T) {
			values := localEnvironment()
			values["BIDORA_PG_ADDR"] = address
			got, err := localprofile.LoadPostgres(func(key string) string { return values[key] })
			if err != nil || got.Resources != validConfig() || got.Address != netip.MustParseAddrPort(address) {
				t.Fatalf("unexpected configuration: %+v, %v", got, err)
			}
		})
	}
}

func TestLoadPostgresRejectsUnsafeAddress(t *testing.T) {
	for _, address := range []string{
		"", "127.0.0.1", "127.0.0.1:", "127.0.0.1:0", "127.0.0.1:65536",
		"localhost:5432", "db.example:5432", "0.0.0.0:5432", "192.168.1.2:5432", "[::]:5432", "[2001:db8::1]:5432",
		"[::ffff:127.0.0.1]:5432", "[::1%lo0]:5432", "127.1:5432", "127.000.0.1:5432", "::1:5432",
		"127.0.0.1:+5432", "127.0.0.1:-1", "127.0.0.1:05432", "127.0.0.1:5e3", "127.0.0.1:54.32", "127.0.0.1:１２",
		" 127.0.0.1:5432", "127.0.0.1:5432\n", "127.0.0.1:5432\x00", "127.0.0.1:5432,remote:5432",
		"postgres://user:secret-canary@127.0.0.1:5432/db", "127.0.0.1:5432?password=secret-canary", strings.Repeat("x", 1000),
	} {
		values := localEnvironment()
		values["BIDORA_PG_ADDR"] = address
		got, err := localprofile.LoadPostgres(func(key string) string { return values[key] })
		if err == nil || got != (localprofile.PostgresConfig{}) {
			t.Fatal("unsafe address accepted or partial configuration returned")
		}
		if err.Error() != "BIDORA_PG_ADDR must be a literal loopback IP and explicit decimal port from 1 to 65535" {
			t.Fatalf("unexpected or unsafe error: %v", err)
		}
	}
}

func TestLoadPostgresDoesNotFallBackToLibpq(t *testing.T) {
	t.Setenv("PGHOST", "127.0.0.1")
	t.Setenv("PGPORT", "5432")
	t.Setenv("PGDATABASE", "personal-database")
	t.Setenv("PGSERVICE", "personal-service")
	values := localEnvironment()
	if _, err := localprofile.LoadPostgres(func(key string) string { return values[key] }); err == nil {
		t.Fatal("missing explicit endpoint fell back to ambient libpq settings")
	}
}

func TestLoadPostgresRejectsInvalidResourceConfig(t *testing.T) {
	if got, err := localprofile.LoadPostgres(nil); err == nil || got != (localprofile.PostgresConfig{}) {
		t.Fatal("nil environment reader accepted")
	}
	values := localEnvironment()
	values["BIDORA_ENV"] = "production"
	values["BIDORA_PG_ADDR"] = "127.0.0.1:5432"
	if got, err := localprofile.LoadPostgres(func(key string) string { return values[key] }); err == nil || got != (localprofile.PostgresConfig{}) {
		t.Fatal("non-local configuration accepted")
	}
}
