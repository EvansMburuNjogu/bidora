package integration_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/identity/password"
	"bidora/backend/internal/pilot"
	"bidora/backend/internal/postgres"
	tenancyapp "bidora/backend/internal/tenancy/application"
	"github.com/google/uuid"
)

func refusedPort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	return uint16(port)
}
func recoveryDeploymentConfig(t *testing.T, redisFixture *liveRedis) pilot.DeploymentConfig {
	t.Helper()
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	dummy, err := hasher.Hash([]byte("synthetic recovery dummy"))
	if err != nil {
		t.Fatal(err)
	}
	secret := sha256.Sum256([]byte(uuid.NewString()))
	tenant := livePostgresConfig(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	assignment := tenancyapp.Assignment{TenantID: tenancyapp.ID(uuid.New()), ClusterID: tenancyapp.ID(uuid.New()), Generation: 1, DatabaseName: tenant.Database, SecretRef: "test/recovery-tenant", EndpointRef: "test/recovery-cluster/" + uuid.NewString(), Region: "local-test"}
	migrator := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	now := time.Now().UTC().Truncate(time.Microsecond)
	err = migrator.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, err := db.Exec(ctx, `INSERT INTO bidora_tenancy.tenants (id,tenant_type,state,name,country,timezone,locale,assignment_generation,version,created_at,updated_at) VALUES ($1,'TENDERPRENEUR','ACTIVE','Synthetic Recovery Tenant','KE','Africa/Nairobi','en-KE',1,1,$2,$2)`, uuid.UUID(assignment.TenantID), now); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bidora_tenancy.database_clusters (id,region,endpoint_ref,state,capacity_policy,version,created_at,updated_at) VALUES ($1,$2,$3,'ACTIVE','{}'::jsonb,1,$4,$4)`, uuid.UUID(assignment.ClusterID), assignment.Region, assignment.EndpointRef, now); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bidora_tenancy.tenant_database_assignments (id,tenant_id,cluster_id,database_name,secret_ref,generation,active,created_at,deactivated_at) VALUES ($1,$2,$3,$4,$5,1,true,$6,NULL)`, uuid.New(), uuid.UUID(assignment.TenantID), uuid.UUID(assignment.ClusterID), assignment.DatabaseName, assignment.SecretRef, now); err != nil {
			return err
		}
		_, err := db.Exec(ctx, `INSERT INTO bidora_tenancy.tenant_schema_versions (tenant_id,migration_set,current_version,target_version,state,checksum,generation,updated_at) VALUES ($1,'tenant_common',9,NULL,'COMPATIBLE',repeat('2',64),1,$2),($1,'tenant_tenderpreneur',34,NULL,'COMPATIBLE',repeat('3',64),1,$2)`, uuid.UUID(assignment.TenantID), now)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return pilot.DeploymentConfig{Environment: "test", Address: "127.0.0.1:0", Runtime: pilot.Config{PasswordPolicy: password.PilotPolicy(), DummyPasswordPHC: dummy, AttemptHMACSecret: secret[:], Redis: redisFixture.config, RedisPrefix: "bidora:test:" + uuid.NewString(), AllowedOrigins: []string{"https://app.example.test"}, TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}}, Control: livePostgresConfig(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN"), Tenants: []pilot.TenantDatabaseConfig{{Assignment: assignment, Postgres: tenant}}}
}
func TestLivePilotBootstrapFailsClosedAndRecoversAfterDependencyOutage(t *testing.T) {
	redisFixture := redisIntegration(t)
	base := recoveryDeploymentConfig(t, redisFixture)
	cases := []struct {
		name   string
		mutate func(*pilot.DeploymentConfig)
	}{{"redis authentication unavailable", func(config *pilot.DeploymentConfig) {
		config.Runtime.Redis.Password = "synthetic-invalid-password"
	}}, {"control postgres refused", func(config *pilot.DeploymentConfig) { config.Control = refusedPostgres(config.Control, refusedPort(t)) }}, {"tenant postgres refused", func(config *pilot.DeploymentConfig) {
		config.Tenants[0].Postgres = refusedPostgres(config.Tenants[0].Postgres, refusedPort(t))
	}}, {"stale registry generation", func(config *pilot.DeploymentConfig) { config.Tenants[0].Assignment.Generation++ }}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := cloneDeploymentConfig(base)
			tc.mutate(&config)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			deployment, err := pilot.Open(ctx, config)
			if err == nil || deployment != nil || err.Error() != "pilot_dependency_unavailable" {
				t.Fatal("dependency outage did not fail closed", deployment, err)
			}
			formatted := fmt.Sprintf("%#v", err)
			for _, value := range []string{config.Control.Password, config.Runtime.Redis.Password, config.Tenants[0].Postgres.Password, "127.0.0.1"} {
				if strings.Contains(formatted, value) {
					t.Fatal("outage error leaked deployment value")
				}
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deployment, err := pilot.Open(ctx, base)
	if err != nil {
		t.Fatal("pilot did not recover after valid dependencies returned", err)
	}
	if !deployment.State.Ready() {
		t.Fatal("recovered pilot did not become ready")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err = deployment.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	if deployment.State.Ready() {
		t.Fatal("recovered pilot stayed ready after shutdown")
	}
}
func refusedPostgres(config postgres.Config, port uint16) postgres.Config {
	config.Host = "127.0.0.1"
	config.Port = port
	config.ConnectTimeout = 100 * time.Millisecond
	config.OperationTimeout = 250 * time.Millisecond
	config.LockTimeout = 100 * time.Millisecond
	return config
}
func cloneDeploymentConfig(config pilot.DeploymentConfig) pilot.DeploymentConfig {
	clone := config
	clone.Runtime.AttemptHMACSecret = append([]byte(nil), config.Runtime.AttemptHMACSecret...)
	clone.Runtime.AllowedOrigins = append([]string(nil), config.Runtime.AllowedOrigins...)
	clone.Runtime.TrustedProxies = append([]netip.Prefix(nil), config.Runtime.TrustedProxies...)
	clone.Tenants = append([]pilot.TenantDatabaseConfig(nil), config.Tenants...)
	return clone
}
